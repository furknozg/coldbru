package intake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/furknozg/coldbru/internal/notification/progress"
	"github.com/furknozg/coldbru/internal/notification/store"
	"github.com/furknozg/coldbru/internal/platform/consumer"
	"github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/platform/outbox"
)

// enqueueSends writes one outbox message per pending delivery of a schedule ($1): subject
// send.<channel>, payload {"delivery_id"} (D-078). It is the set form of outbox.WriteMessage for a
// whole schedule in one statement, as a large campaign has one row per delivery. The subject is
// built from the delivery's channel, which the delivery_channel_check constraint limits to sms and
// email, and the rows are ordered by delivery id (UUIDv7, so by creation).
const enqueueSends = `
INSERT INTO outbox (subject, payload)
SELECT $2 || channel, jsonb_build_object('delivery_id', id::text)
FROM delivery
WHERE schedule_id = $1 AND status = 'pending'
ORDER BY id`

// Schedules handles the SCHEDULES stream: the firing of one (language, timezone) slot (D-078). Use
// Handle as the consumer.RawHandler of a consumer.RawProcessor.
type Schedules struct {
	Logger *slog.Logger
}

// Handle fires a schedule in the RawProcessor's transaction.
//
//   - Campaign or schedule cancelled, or campaign completed: dropped.
//   - Schedule already fired: nothing is written (the status is the dedup key).
//   - Fewer recipient batches than the campaign announced: RetryLater.
//   - Otherwise the schedule becomes fired, the campaign sending (if it was still scheduled) and
//     every pending delivery of the schedule gets a message for the SEND stream, and
//     evt.notification.started.v1 goes to the outbox (D-080), all in the same transaction. The
//     deliveries stay pending; the send worker moves them.
//   - Undecodable payload or unknown schedule: Malformed.
func (h *Schedules) Handle(ctx context.Context, tx pgx.Tx, msg jetstream.Msg) error {
	var p FirePayload
	if err := json.Unmarshal(msg.Data(), &p); err != nil {
		return consumer.Malformed(fmt.Errorf("decode fire payload: %w", err))
	}
	scheduleID, err := uuid.Parse(p.ScheduleID)
	if err != nil {
		return consumer.Malformed(fmt.Errorf("fire payload schedule_id %q is not a uuid", p.ScheduleID))
	}
	campaignID, err := uuid.Parse(p.CampaignID)
	if err != nil {
		return consumer.Malformed(fmt.Errorf("fire payload campaign_id %q is not a uuid", p.CampaignID))
	}

	q := store.New(tx)
	// Campaign first, schedule second: the same order as the cancellation and the batch intake.
	c, err := q.LockSendCampaign(ctx, campaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		return consumer.Malformed(fmt.Errorf("fire for unknown campaign %s", p.CampaignID))
	}
	if err != nil {
		return fmt.Errorf("lock campaign: %w", err)
	}
	s, err := q.LockLocaleSchedule(ctx, scheduleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return consumer.Malformed(fmt.Errorf("fire for unknown schedule %s", p.ScheduleID))
	}
	if err != nil {
		return fmt.Errorf("lock schedule: %w", err)
	}
	if s.CampaignID != campaignID {
		return consumer.Malformed(fmt.Errorf("schedule %s belongs to campaign %s, not %s", p.ScheduleID, s.CampaignID, p.CampaignID))
	}

	switch {
	case c.Status == "cancelled" || s.Status == "cancelled":
		h.Logger.InfoContext(ctx, "fire for cancelled campaign dropped",
			slog.String("campaign_id", p.CampaignID), slog.String("schedule_id", p.ScheduleID))
		return nil
	case s.Status == "fired":
		h.Logger.InfoContext(ctx, "duplicate fire skipped",
			slog.String("campaign_id", p.CampaignID), slog.String("schedule_id", p.ScheduleID))
		return nil
	case c.Status == "completed":
		// Nothing should be left to fire in a finished campaign; do not send anything for it.
		h.Logger.WarnContext(ctx, "fire for completed campaign dropped",
			slog.String("campaign_id", p.CampaignID), slog.String("schedule_id", p.ScheduleID))
		return nil
	}

	received, err := q.CountRecipientBatches(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("count recipient batches: %w", err)
	}
	if received < int64(c.BatchCount) {
		return consumer.RetryLater(0, fmt.Sprintf("campaign %s has %d of %d recipient batches", p.CampaignID, received, c.BatchCount))
	}

	if _, err := q.MarkScheduleFired(ctx, scheduleID); err != nil {
		return fmt.Errorf("mark schedule fired: %w", err)
	}
	if _, err := q.StartSendCampaign(ctx, campaignID); err != nil {
		return fmt.Errorf("start campaign: %w", err)
	}
	tag, err := tx.Exec(ctx, enqueueSends, scheduleID, SendSubjectPrefix)
	if err != nil {
		return fmt.Errorf("enqueue sends: %w", err)
	}
	started, err := events.New(progress.EventTypeStarted, progress.EventVersion, progress.EventProducer, p.CampaignID,
		progress.StartedData{
			CampaignID: p.CampaignID, ScheduleID: p.ScheduleID, Language: s.Language, Timezone: s.Timezone,
			StartedAt: time.Now().UTC(),
		})
	if err != nil {
		return fmt.Errorf("build started event: %w", err)
	}
	if _, err := outbox.Write(ctx, tx, started); err != nil {
		return fmt.Errorf("write started event: %w", err)
	}
	h.Logger.InfoContext(ctx, "schedule fired",
		slog.String("campaign_id", p.CampaignID), slog.String("schedule_id", p.ScheduleID),
		slog.String("language", s.Language), slog.String("timezone", s.Timezone), slog.Int64("deliveries", tag.RowsAffected()))
	return nil
}
