// Package progress lets Proposal follow what Notification reports about a campaign (D-033, D-080):
// a durable consumer of evt.notification.> on the EVENTS stream moves the campaign from `scheduled`
// to `sending` when the first schedule fires and to `completed` when every delivery is final, and
// tells the campaign's event streams about it (flow.CampaignStatus).
package progress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/furknozg/coldbru/internal/platform/consumer"
	"github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/store"
)

// next returns the status a campaign in status current moves to for an event of type typ, and
// false when the event changes nothing (D-080):
//
//   - started: scheduled to sending; every other status is left alone, so a second started event
//     (one per schedule) and one that arrives after completed or cancelled are no-ops.
//   - completed: scheduled or sending to completed. A cancelled campaign stays cancelled; a
//     completed one stays completed; a draft cannot have been sent.
func next(current, typ string) (string, bool) {
	switch typ {
	case EventTypeStarted:
		if current == flow.CampaignStatusScheduled {
			return flow.CampaignStatusSending, true
		}
	case EventTypeCompleted:
		if current == flow.CampaignStatusScheduled || current == flow.CampaignStatusSending {
			return flow.CampaignStatusCompleted, true
		}
	}
	return "", false
}

// Consumer applies Notification's progress events to the campaigns of proposal_db.
type Consumer struct {
	pool *pgxpool.Pool
	hub  *flow.Hub
	log  *slog.Logger
	now  func() time.Time

	// pending holds the hub events of an event whose handler succeeded, until its transaction is
	// known to have committed (see process).
	mu      sync.Mutex
	pending map[string][]flow.Event
}

// Deps are what the consumer needs.
type Deps struct {
	JS     jetstream.JetStream
	Pool   *pgxpool.Pool
	Hub    *flow.Hub
	Logger *slog.Logger
	// Backoff is the redelivery delay; the zero value means consumer.DefaultBackoff. Tests shorten it.
	Backoff consumer.Backoff
	// Now replaces time.Now for the event time, in tests.
	Now func() time.Time
}

// ConsumerConfig returns the durable consumer definition: events of Notification on EVENTS.
func ConsumerConfig() jetstream.ConsumerConfig {
	cfg := consumer.ConsumerConfig(Durable, Filter)
	cfg.MaxAckPending = maxAcked
	return cfg
}

// New returns a Consumer. Hub may be nil (no event is published).
func New(d Deps) (*Consumer, error) {
	if d.Pool == nil {
		return nil, errors.New("progress: a database pool is required")
	}
	c := &Consumer{pool: d.Pool, hub: d.Hub, log: d.Logger, now: d.Now, pending: map[string][]flow.Event{}}
	if c.log == nil {
		c.log = slog.Default()
	}
	if c.now == nil {
		c.now = time.Now
	}
	return c, nil
}

// Handle applies one envelope in the Processor's transaction (consumer.Handler). Types and versions
// it does not know are ignored. A campaign it does not know is a warning and a no-op: the event is
// acked, since a campaign cannot appear later (Proposal creates it before Notification hears of it).
func (c *Consumer) Handle(ctx context.Context, tx pgx.Tx, env events.Envelope) error {
	if env.Version != EventVersion || (env.Type != EventTypeStarted && env.Type != EventTypeCompleted) {
		c.log.DebugContext(ctx, "notification event ignored", slog.String("type", env.Type), slog.Int("version", env.Version))
		return nil
	}
	campaignID, err := campaignOf(env)
	if err != nil {
		return consumer.Malformed(fmt.Errorf("%s event %s: %w", env.Type, env.ID, err))
	}

	q := store.New(tx)
	camp, err := q.GetCampaignForUpdate(ctx, campaignID)
	if errors.Is(err, pgx.ErrNoRows) {
		c.log.WarnContext(ctx, "notification event for an unknown campaign ignored",
			slog.String("type", env.Type), slog.String("campaign_id", campaignID.String()), slog.String("event_id", env.ID))
		return nil
	}
	if err != nil {
		return fmt.Errorf("lock campaign: %w", err)
	}
	to, ok := next(camp.Status, env.Type)
	if !ok {
		c.log.InfoContext(ctx, "notification event changes nothing",
			slog.String("type", env.Type), slog.String("campaign_id", campaignID.String()), slog.String("status", camp.Status))
		return nil
	}
	if _, err := q.SetCampaignStatus(ctx, store.SetCampaignStatusParams{ID: campaignID, Status: to}); err != nil {
		return fmt.Errorf("set campaign status: %w", err)
	}
	c.log.InfoContext(ctx, "campaign status updated by notification",
		slog.String("campaign_id", campaignID.String()), slog.String("from", camp.Status), slog.String("to", to), slog.String("event", env.Type))
	c.stash(env.ID, flow.CampaignStatus{Meta: flow.Meta{CampaignID: campaignID, At: c.now().UTC()}, Status: to})
	return nil
}

// campaignOf decodes the campaign id of a started or completed event.
func campaignOf(env events.Envelope) (uuid.UUID, error) {
	var d struct {
		CampaignID string `json:"campaign_id"`
	}
	if err := json.Unmarshal(env.Data, &d); err != nil {
		return uuid.Nil, fmt.Errorf("decode data: %w", err)
	}
	id, err := uuid.Parse(d.CampaignID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("campaign_id %q is not a uuid", d.CampaignID)
	}
	return id, nil
}

func (c *Consumer) stash(eventID string, evs ...flow.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending[eventID] = evs
}

func (c *Consumer) take(eventID string) []flow.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	evs := c.pending[eventID]
	delete(c.pending, eventID)
	return evs
}

// process runs one message through the Processor and publishes the hub events of the handler after
// the transaction committed: Process returns nil once it acked, which follows the commit. An error
// means the transaction was rolled back, so what the handler stashed is dropped.
func (c *Consumer) process(proc *consumer.Processor) func(context.Context, jetstream.Msg) error {
	return func(ctx context.Context, msg jetstream.Msg) error {
		err := proc.Process(ctx, msg)
		var head struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(msg.Data(), &head) != nil || head.ID == "" {
			return err
		}
		evs := c.take(head.ID)
		if err == nil && c.hub != nil {
			for _, ev := range evs {
				c.hub.Publish(ev)
			}
		}
		return err
	}
}

// Start creates the durable consumer on EVENTS (the stream must exist) and starts consuming. It
// returns the function that stops it: it drains the consumer, waiting for the handlers in flight,
// until ctx ends. Messages are processed with a context that ctx's cancellation does not reach, so
// a handler in flight finishes its transaction when the service is told to stop.
func Start(ctx context.Context, d Deps) (stop func(context.Context) error, err error) {
	c, err := New(d)
	if err != nil {
		return nil, err
	}
	if d.JS == nil {
		return nil, errors.New("progress: JetStream is required")
	}
	proc := &consumer.Processor{Pool: d.Pool, Logger: c.log, Backoff: d.Backoff, Handler: c.Handle}
	process := c.process(proc)

	cfg := ConsumerConfig()
	cons, err := d.JS.CreateOrUpdateConsumer(ctx, Stream, cfg)
	if err != nil {
		return nil, fmt.Errorf("create consumer %s on %s: %w", cfg.Durable, Stream, err)
	}
	procCtx, cancelProc := context.WithCancel(context.WithoutCancel(ctx))
	cc, err := cons.Consume(func(msg jetstream.Msg) {
		// Process settles the message itself; an error it returns is a failed attempt that was nak'd.
		if err := process(procCtx, msg); err != nil {
			c.log.Error("message failed, will be redelivered", "consumer", cfg.Durable, "subject", msg.Subject(), "error", err)
		}
	})
	if err != nil {
		cancelProc()
		return nil, fmt.Errorf("consume %s: %w", cfg.Durable, err)
	}
	c.log.Info("consumer started", "consumer", cfg.Durable, "stream", Stream, "filter", cfg.FilterSubject,
		"max_ack_pending", cfg.MaxAckPending, "ack_wait", cfg.AckWait.String(), "max_deliver", cfg.MaxDeliver)

	return func(stopCtx context.Context) error {
		defer cancelProc()
		cc.Drain()
		select {
		case <-cc.Closed():
			return nil
		case <-stopCtx.Done():
			return fmt.Errorf("wait for consumer %s to drain: %w", cfg.Durable, stopCtx.Err())
		}
	}, nil
}
