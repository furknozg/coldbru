package intake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/notification/links"
	"github.com/furknozg/coldbru/internal/notification/store"
	"github.com/furknozg/coldbru/internal/platform/consumer"
)

// Skip reasons logged for a recipient that is left out of a batch.
const (
	skipNoSchedule = "no schedule for the recipient's (language, timezone)"
	skipNoMessage  = "no message for the recipient's (language, channel)"
	skipInvalid    = "recipient has no customer id, no contact or an unknown channel"
)

// Recipients handles the RECIPIENTS stream: one batch per message (D-077, D-078). Use Handle as the
// consumer.RawHandler of a consumer.RawProcessor.
type Recipients struct {
	Logger *slog.Logger
	// Links creates the short link of every delivery of a batch (D-106).
	Links links.Creator
}

// Handle records the batch and creates a delivery per recipient in the RawProcessor's transaction.
//
//   - Campaign not received yet (the event is on another stream): RetryLater.
//   - Campaign cancelled or completed: the batch is dropped, so no personal data lands on a
//     campaign that has been cleared.
//   - Batch already recorded: nothing is written (recipient_batch's primary key is the dedup key).
//   - A recipient with no schedule for its (language, timezone), no message for its (language,
//     channel), or no usable customer id, contact and channel is skipped and logged at error level;
//     the rest of the batch is written and the batch counts as received.
//   - Undecodable batch, or a batch number outside the campaign's batch count: Malformed.
//   - The short links of the batch's deliveries are requested from the redirect service in one call
//     before the rows are written (D-106). The delivery ids are chosen here (UUIDv7) so the request
//     can name them; the code the service returns is stored on the row. If the service cannot be
//     reached or fails, the handler fails: the transaction rolls back, nothing of the batch is
//     recorded and the message is redelivered with backoff, so the batch is delayed, never half
//     written. An answer that rejects the request (4xx) can never succeed and is Malformed.
func (h *Recipients) Handle(ctx context.Context, tx pgx.Tx, msg jetstream.Msg) error {
	var b Batch
	if err := json.Unmarshal(msg.Data(), &b); err != nil {
		return consumer.Malformed(fmt.Errorf("decode batch: %w", err))
	}
	campaignID, err := uuid.Parse(b.CampaignID)
	if err != nil {
		return consumer.Malformed(fmt.Errorf("batch campaign_id %q is not a uuid", b.CampaignID))
	}
	if b.BatchNo < 1 {
		return consumer.Malformed(fmt.Errorf("batch_no = %d, want at least 1", b.BatchNo))
	}

	q := store.New(tx)
	c, err := q.LockSendCampaign(ctx, campaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		return consumer.RetryLater(0, "campaign "+b.CampaignID+" not received yet")
	}
	if err != nil {
		return fmt.Errorf("lock campaign: %w", err)
	}
	if c.Status == "cancelled" || c.Status == "completed" {
		h.Logger.InfoContext(ctx, "batch for finished campaign dropped",
			slog.String("campaign_id", b.CampaignID), slog.Int("batch_no", b.BatchNo), slog.String("status", c.Status))
		return nil
	}
	if int64(b.BatchNo) > int64(c.BatchCount) {
		return consumer.Malformed(fmt.Errorf("batch_no %d is above the campaign's batch_count %d", b.BatchNo, c.BatchCount))
	}

	n, err := q.InsertRecipientBatch(ctx, store.InsertRecipientBatchParams{
		CampaignID: campaignID, BatchNo: int32(b.BatchNo), Recipients: int32(len(b.Recipients)),
	})
	if err != nil {
		return fmt.Errorf("insert recipient batch: %w", err)
	}
	if n == 0 {
		h.Logger.InfoContext(ctx, "duplicate batch skipped", slog.String("campaign_id", b.CampaignID), slog.Int("batch_no", b.BatchNo))
		return nil
	}

	schedules, err := q.ListLocaleSchedules(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("list schedules: %w", err)
	}
	messages, err := q.ListMessages(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("list messages: %w", err)
	}
	scheduleIDs := make(map[[2]string]uuid.UUID, len(schedules))
	for _, s := range schedules {
		scheduleIDs[[2]string{s.Language, s.Timezone}] = s.ID
	}
	messageIDs := make(map[[2]string]uuid.UUID, len(messages))
	for _, m := range messages {
		messageIDs[[2]string{m.Language, m.Channel}] = m.ID
	}

	target, err := content.CTAURL(c.CtaUrl, campaignID.String())
	if err != nil {
		return consumer.Malformed(fmt.Errorf("campaign %s cta_url: %w", b.CampaignID, err))
	}
	rows := make([]store.InsertDeliveryBatchParams, 0, len(b.Recipients))
	reqs := make([]links.Request, 0, len(b.Recipients))
	skipped := map[skipKey]int{}
	seen := make(map[[2]string]bool, len(b.Recipients))
	for _, r := range b.Recipients {
		scheduleID, hasSchedule := scheduleIDs[[2]string{r.Language, r.Timezone}]
		messageID, hasMessage := messageIDs[[2]string{r.Language, r.Channel}]
		switch {
		case r.CustomerID == "" || strings.TrimSpace(r.Contact) == "" || !channels[r.Channel]:
			skipped[skipKey{r.Language, r.Timezone, r.Channel, skipInvalid}]++
		case !hasSchedule:
			skipped[skipKey{r.Language, r.Timezone, r.Channel, skipNoSchedule}]++
		case !hasMessage:
			skipped[skipKey{r.Language, r.Timezone, r.Channel, skipNoMessage}]++
		case seen[[2]string{r.CustomerID, r.Channel}]:
			// The same person twice in a batch: one delivery (D-023), so one link.
		default:
			seen[[2]string{r.CustomerID, r.Channel}] = true
			id, err := uuid.NewV7()
			if err != nil {
				return fmt.Errorf("new delivery id: %w", err)
			}
			rows = append(rows, store.InsertDeliveryBatchParams{
				ID: id, CampaignID: campaignID, ScheduleID: scheduleID, MessageID: messageID,
				CustomerID: r.CustomerID, Channel: r.Channel,
				FirstName: pgtype.Text{String: r.FirstName, Valid: true},
				Contact:   pgtype.Text{String: r.Contact, Valid: true},
			})
			reqs = append(reqs, links.Request{
				DeliveryID: id, CampaignID: campaignID, Channel: r.Channel, Language: r.Language, Timezone: r.Timezone, TargetURL: target,
			})
		}
	}
	h.logSkipped(ctx, b, skipped)

	if len(reqs) > 0 {
		codes, err := h.Links.Create(ctx, reqs)
		switch {
		case links.IsRejected(err):
			return consumer.Malformed(fmt.Errorf("batch %d of campaign %s: %w", b.BatchNo, b.CampaignID, err))
		case err != nil:
			return fmt.Errorf("create links for batch %d of campaign %s: %w", b.BatchNo, b.CampaignID, err)
		}
		for i := range rows {
			rows[i].LinkCode = pgtype.Text{String: codes[rows[i].ID], Valid: true}
		}
	}

	var batchErr error
	q.InsertDeliveryBatch(ctx, rows).Exec(func(_ int, err error) {
		if err != nil && batchErr == nil {
			batchErr = err
		}
	})
	if batchErr != nil {
		return dbError("insert deliveries", batchErr)
	}
	h.Logger.InfoContext(ctx, "batch received",
		slog.String("campaign_id", b.CampaignID), slog.Int("batch_no", b.BatchNo),
		slog.Int("recipients", len(b.Recipients)), slog.Int("deliveries", len(rows)))
	return nil
}

// skipKey groups skipped recipients for one log line.
type skipKey struct{ language, timezone, channel, reason string }

// logSkipped writes one error record per (language, timezone, channel, reason), in a stable order.
func (h *Recipients) logSkipped(ctx context.Context, b Batch, skipped map[skipKey]int) {
	keys := make([]skipKey, 0, len(skipped))
	for k := range skipped {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, c := keys[i], keys[j]
		return a.language+"\x00"+a.timezone+"\x00"+a.channel+"\x00"+a.reason < c.language+"\x00"+c.timezone+"\x00"+c.channel+"\x00"+c.reason
	})
	for _, k := range keys {
		h.Logger.ErrorContext(ctx, "recipients skipped",
			slog.String("campaign_id", b.CampaignID), slog.Int("batch_no", b.BatchNo),
			slog.String("language", k.language), slog.String("timezone", k.timezone), slog.String("channel", k.channel),
			slog.Int("skipped", skipped[k]), slog.String("reason", k.reason))
	}
}
