// Package progress finishes campaigns (D-080): a checker that finds a sending campaign whose
// schedules have all fired and whose deliveries are all final, completes it, clears its personal
// data and writes evt.notification.completed.v1. The event types of the progress of a campaign
// (started, completed) are defined here.
//
// The checker runs every Interval and also when the send workers nudge it after a delivery was
// settled. Completion is done by the checker alone, in one transaction that locks the campaign and
// asks the predicate again, so two settlements that end at the same time cannot both miss it and
// two replicas cannot both complete it: the second finds the campaign no longer sending.
package progress

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/notification/store"
	"github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/platform/outbox"
)

const (
	// DefaultInterval is how often the checker looks for finished campaigns (D-080).
	DefaultInterval = 2 * time.Second
	// DefaultMinGap is the shortest time between two checks that nudges ask for. A busy channel
	// settles 100 deliveries a second and nudges at that rate; the checker answers at most this often.
	DefaultMinGap = 250 * time.Millisecond
)

// canComplete is the completion predicate (D-080): the campaign is sending, no schedule is still
// scheduled (every one has fired; a cancelled schedule means a cancelled campaign, which is not
// sending) and no delivery is pending or sending.
func canComplete(status string, scheduledSchedules, activeDeliveries int64) bool {
	return status == "sending" && scheduledSchedules == 0 && activeDeliveries == 0
}

// Estimate is the estimated cost of a campaign: the segments of the sent SMS at the segment price
// plus the sent emails at the email price (D-080).
func Estimate(smsSegments int64, counts []Count, p content.Prices) content.Money {
	var emails int64
	for _, c := range counts {
		if c.Channel == "email" && c.Status == "sent" {
			emails += c.Count
		}
	}
	return p.SMSSegment.Times(int(smsSegments)) + p.Email.Times(int(emails))
}

// Config holds the checker's settings.
type Config struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
	// Prices are the demo prices of an SMS segment and an email (content.LoadConfig).
	Prices content.Prices
	// Interval defaults to DefaultInterval, MinGap to DefaultMinGap. Tests shorten them.
	Interval time.Duration
	MinGap   time.Duration
}

// Checker completes finished campaigns. Create it with New, start it with Start, and give
// Nudge to the send workers.
type Checker struct {
	pool     *pgxpool.Pool
	q        *store.Queries
	log      *slog.Logger
	prices   content.Prices
	interval time.Duration
	minGap   time.Duration
	nudge    chan struct{}
}

// New returns a Checker for cfg.
func New(cfg Config) (*Checker, error) {
	if cfg.Pool == nil {
		return nil, errors.New("progress: a database pool is required")
	}
	c := &Checker{
		pool: cfg.Pool, q: store.New(cfg.Pool), log: cfg.Logger, prices: cfg.Prices,
		interval: cfg.Interval, minGap: cfg.MinGap, nudge: make(chan struct{}, 1),
	}
	if c.log == nil {
		c.log = slog.Default()
	}
	if c.interval <= 0 {
		c.interval = DefaultInterval
	}
	if c.minGap <= 0 {
		c.minGap = DefaultMinGap
	}
	return c, nil
}

// Nudge asks for a check soon. It never blocks: a nudge that finds one already waiting is dropped,
// as the waiting one covers it.
func (c *Checker) Nudge() {
	select {
	case c.nudge <- struct{}{}:
	default:
	}
}

// Start runs the checker until ctx ends or the returned stop function is called, and the function
// waits for a check in progress to finish (until its ctx ends). A check in progress is not
// aborted by ctx's cancellation: it finishes its transaction.
func (c *Checker) Start(ctx context.Context) (stop func(context.Context) error) {
	runCtx, cancel := context.WithCancel(ctx)
	work := context.WithoutCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.loop(runCtx, work)
	}()
	return func(stopCtx context.Context) error {
		cancel()
		select {
		case <-done:
			return nil
		case <-stopCtx.Done():
			return fmt.Errorf("wait for completion checker: %w", stopCtx.Err())
		}
	}
}

func (c *Checker) loop(runCtx, work context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	last := time.Time{}
	for {
		select {
		case <-runCtx.Done():
			return
		case <-ticker.C:
		case <-c.nudge:
			// Space nudge-driven checks: wait out the rest of the gap since the last check.
			if wait := c.minGap - time.Since(last); wait > 0 {
				t := time.NewTimer(wait)
				select {
				case <-runCtx.Done():
					t.Stop()
					return
				case <-t.C:
				}
			}
		}
		last = time.Now()
		if _, err := c.CheckOnce(work); err != nil {
			c.log.Warn("completion check failed", slog.Any("error", err))
		}
	}
}

// CheckOnce completes every sending campaign that is finished and returns how many it completed.
// A failure on one campaign is logged and does not stop the others; the first error is returned.
func (c *Checker) CheckOnce(ctx context.Context) (int, error) {
	ids, err := c.q.ListCompletableCampaigns(ctx)
	if err != nil {
		return 0, fmt.Errorf("list completable campaigns: %w", err)
	}
	var (
		completed int
		first     error
	)
	for _, id := range ids {
		ok, err := c.complete(ctx, id)
		if err != nil {
			c.log.Warn("campaign not completed", slog.String("campaign_id", id.String()), slog.Any("error", err))
			if first == nil {
				first = fmt.Errorf("complete campaign %s: %w", id, err)
			}
			continue
		}
		if ok {
			completed++
		}
	}
	return completed, first
}

// complete finishes one campaign in a transaction: lock, ask the predicate again, mark it
// completed, clear the personal data of its deliveries (D-077) and write the completed event to the
// outbox. It reports false when the campaign is not finished (any more).
func (c *Checker) complete(ctx context.Context, id uuid.UUID) (bool, error) {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }() // no-op after Commit
	q := c.q.WithTx(tx)

	if _, err := q.LockSendCampaign(ctx, id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("lock campaign: %w", err)
	}
	p, err := q.GetCampaignProgress(ctx, id)
	if err != nil {
		return false, fmt.Errorf("read progress: %w", err)
	}
	if !canComplete(p.Status, p.ScheduledSchedules, p.ActiveDeliveries) {
		return false, nil
	}

	at, err := q.CompleteSendCampaign(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("complete campaign: %w", err)
	}
	rows, err := q.CountDeliveriesByStatus(ctx, id)
	if err != nil {
		return false, fmt.Errorf("count deliveries: %w", err)
	}
	segments, err := q.SumSentSMSSegments(ctx, id)
	if err != nil {
		return false, fmt.Errorf("sum sms segments: %w", err)
	}
	locales, err := q.CountSentByLocale(ctx, id)
	if err != nil {
		return false, fmt.Errorf("count sent by locale: %w", err)
	}
	cleared, err := q.ClearDeliveryPII(ctx, id)
	if err != nil {
		return false, fmt.Errorf("clear delivery personal data: %w", err)
	}

	counts := make([]Count, 0, len(rows))
	for _, r := range rows {
		counts = append(counts, Count{Channel: r.Channel, Status: r.Status, Count: r.Total})
	}
	sentByLocale := make([]LocaleCount, 0, len(locales))
	for _, l := range locales {
		sentByLocale = append(sentByLocale, LocaleCount{Channel: l.Channel, Language: l.Language, Timezone: l.Timezone, Sent: l.Sent})
	}
	data := CompletedData{
		CampaignID:       id.String(),
		CompletedAt:      at.Time.UTC(),
		Counts:           counts,
		SMSSegments:      segments,
		EstimatedCostUSD: Estimate(segments, counts, c.prices).String(),
		SentByLocale:     sentByLocale,
	}
	env, err := events.New(EventTypeCompleted, EventVersion, EventProducer, id.String(), data)
	if err != nil {
		return false, err
	}
	if _, err := outbox.Write(ctx, tx, env); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit: %w", err)
	}
	c.log.Info("campaign completed", slog.String("campaign_id", data.CampaignID), slog.Int64("sms_segments", data.SMSSegments),
		slog.String("estimated_cost_usd", data.EstimatedCostUSD), slog.Int64("pii_cleared", cleared))
	return true, nil
}
