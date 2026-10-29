package intake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/furknozg/coldbru/internal/notification/store"
	"github.com/furknozg/coldbru/internal/platform/consumer"
	"github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/platform/outbox"
)

// Events handles Proposal's envelopes on the EVENTS stream (D-078). Use Handle as the
// consumer.Handler of a consumer.Processor; the Processor records processed_event.
type Events struct {
	Logger *slog.Logger
}

// Handle applies one envelope in the Processor's transaction. Types it does not know are ignored.
func (h *Events) Handle(ctx context.Context, tx pgx.Tx, env events.Envelope) error {
	if env.Version != EventVersion {
		h.Logger.DebugContext(ctx, "event version ignored", slog.String("type", env.Type), slog.Int("version", env.Version))
		return nil
	}
	switch env.Type {
	case EventTypeConfirmed:
		return h.confirmed(ctx, tx, env)
	case EventTypeCancelled:
		return h.cancelled(ctx, tx, env)
	default:
		h.Logger.DebugContext(ctx, "event type ignored", slog.String("type", env.Type))
		return nil
	}
}

// confirmed creates the campaign, its schedules and its messages, and hands every schedule to NATS
// through the outbox, all in the one transaction. A campaign that already exists means the event was
// applied before: nothing is written.
func (h *Events) confirmed(ctx context.Context, tx pgx.Tx, env events.Envelope) error {
	var d ConfirmedData
	if err := json.Unmarshal(env.Data, &d); err != nil {
		return consumer.Malformed(fmt.Errorf("decode %s data: %w", env.Type, err))
	}
	campaignID, err := d.validate()
	if err != nil {
		return consumer.Malformed(fmt.Errorf("%s event %s: %w", env.Type, env.ID, err))
	}

	q := store.New(tx)
	n, err := q.InsertSendCampaign(ctx, store.InsertSendCampaignParams{
		ID: campaignID, CtaUrl: d.CTAURL, BatchCount: int32(d.RecipientBatches),
	})
	if err != nil {
		return dbError("insert campaign", err)
	}
	if n == 0 {
		h.Logger.InfoContext(ctx, "campaign already exists, confirmed event ignored", slog.String("campaign_id", d.CampaignID))
		return nil
	}

	for _, s := range d.Schedules {
		params := store.CreateLocaleScheduleParams{CampaignID: campaignID, Language: s.Language, Timezone: s.Timezone}
		if s.SendAt != nil {
			params.SendAt = pgtype.Timestamptz{Time: *s.SendAt, Valid: true}
		}
		sched, err := q.CreateLocaleSchedule(ctx, params)
		if err != nil {
			return dbError("insert locale schedule", err)
		}
		if err := writeFire(ctx, tx, sched, d.CampaignID); err != nil {
			return err
		}
	}
	for _, m := range d.Messages {
		if _, err := q.CreateMessage(ctx, store.CreateMessageParams{
			CampaignID: campaignID, Language: m.Language, Channel: m.Channel, Content: m.Content,
		}); err != nil {
			return dbError("insert message", err)
		}
	}
	h.Logger.InfoContext(ctx, "campaign received",
		slog.String("campaign_id", d.CampaignID), slog.Int("schedules", len(d.Schedules)),
		slog.Int("messages", len(d.Messages)), slog.Int("batches", d.RecipientBatches))
	return nil
}

// writeFire queues the firing of a schedule: a scheduled NATS message when the slot has a send time,
// a direct firing otherwise (D-078).
func writeFire(ctx context.Context, tx pgx.Tx, sched store.LocaleSchedule, campaignID string) error {
	payload, err := json.Marshal(FirePayload{ScheduleID: sched.ID.String(), CampaignID: campaignID})
	if err != nil {
		return fmt.Errorf("marshal fire payload: %w", err)
	}
	if !sched.SendAt.Valid {
		if _, err := outbox.WriteMessage(ctx, tx, FireSubject(sched.ID.String()), payload); err != nil {
			return fmt.Errorf("write fire message: %w", err)
		}
		return nil
	}
	if _, err := outbox.WriteMessage(ctx, tx, ScheduleSubject(sched.ID.String()), payload, outbox.WithHeaders(map[string]string{
		"Nats-Schedule":        scheduleHeader(sched.SendAt.Time),
		"Nats-Schedule-Target": FireSubject(sched.ID.String()),
	})); err != nil {
		return fmt.Errorf("write schedule message: %w", err)
	}
	return nil
}

// cancelled cancels a campaign that has not finished: its unfired schedules, its pending
// deliveries, and then the personal data of the deliveries that are final (D-077, D-078). A completed
// and a cancelled campaign are no-ops. An unknown campaign is RetryLater: its confirmed event is
// ahead in the stream and not applied yet.
func (h *Events) cancelled(ctx context.Context, tx pgx.Tx, env events.Envelope) error {
	var d CancelledData
	if err := json.Unmarshal(env.Data, &d); err != nil {
		return consumer.Malformed(fmt.Errorf("decode %s data: %w", env.Type, err))
	}
	campaignID, err := uuid.Parse(d.CampaignID)
	if err != nil {
		return consumer.Malformed(fmt.Errorf("%s event %s: campaign_id %q is not a uuid", env.Type, env.ID, d.CampaignID))
	}

	q := store.New(tx)
	c, err := q.LockSendCampaign(ctx, campaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Proposal publishes a cancel only after the confirmed event, which is ahead in the stream but
		// may not be applied yet (it can be waiting for a retry itself). Defer instead of dropping, so
		// the cancel takes effect once the campaign exists; after MaxDeliver it is terminated with an error.
		return consumer.RetryLater(0, "cancel for campaign "+d.CampaignID+" not received yet")
	}
	if err != nil {
		return fmt.Errorf("lock campaign: %w", err)
	}
	if c.Status == "completed" || c.Status == "cancelled" {
		h.Logger.InfoContext(ctx, "cancel for finished campaign ignored",
			slog.String("campaign_id", d.CampaignID), slog.String("status", c.Status))
		return nil
	}

	if _, err := q.CancelSendCampaign(ctx, campaignID); err != nil {
		return fmt.Errorf("cancel campaign: %w", err)
	}
	schedules, err := q.CancelScheduledSchedules(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("cancel schedules: %w", err)
	}
	deliveries, err := q.CancelPendingDeliveries(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("cancel deliveries: %w", err)
	}
	// Clears the deliveries that are final, the ones just cancelled included. A delivery that is
	// still sending is skipped: its send worker clears it when it settles it (D-077).
	cleared, err := q.ClearDeliveryPII(ctx, campaignID)
	if err != nil {
		return fmt.Errorf("clear delivery personal data: %w", err)
	}
	h.Logger.InfoContext(ctx, "campaign cancelled",
		slog.String("campaign_id", d.CampaignID), slog.String("was", c.Status), slog.String("reason", d.Reason),
		slog.Int64("schedules", schedules), slog.Int64("deliveries", deliveries), slog.Int64("pii_cleared", cleared))
	return nil
}

// dbError wraps a database error of a write that depends only on the message. A constraint
// violation (SQLSTATE class 23) is Malformed: the same data fails the same way on every retry.
// The cancellation does not use it: its statements depend on the state of other rows, so every
// failure there is retried.
func dbError(what string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && len(pgErr.Code) == 5 && pgErr.Code[:2] == "23" {
		return consumer.Malformed(fmt.Errorf("%s: %w", what, err))
	}
	return fmt.Errorf("%s: %w", what, err)
}
