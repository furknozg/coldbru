package intake

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/furknozg/coldbru/internal/notification/links"
	"github.com/furknozg/coldbru/internal/platform/consumer"
)

// Streams, durable consumer names and their limits (D-078). Streams come from infra/nats/streams;
// the service only creates its consumers on them.
const (
	EventsStream     = "EVENTS"
	RecipientsStream = "RECIPIENTS"
	SchedulesStream  = "SCHEDULES"

	EventsConsumer     = "notification-events"
	RecipientsConsumer = "notification-recipients"
	SchedulesConsumer  = "notification-schedules"

	// Messages in flight per consumer. A message waiting for RetryLater counts while it waits, so the
	// RECIPIENTS limit leaves room for every batch of a large campaign to arrive before its event.
	eventsMaxAckPending     = 100
	recipientsMaxAckPending = 500
	schedulesMaxAckPending  = 100

	// A recipient batch is 500 inserts in one round trip; a firing writes one outbox row per delivery
	// in one statement, which takes longer on a large campaign.
	recipientsAckWait = 30 * time.Second
	schedulesAckWait  = time.Minute
)

// Deps are what the consumers need.
type Deps struct {
	JS     jetstream.JetStream
	Pool   *pgxpool.Pool
	Logger *slog.Logger
	// Links creates the short link of every delivery (D-106); required.
	Links links.Creator
	// Backoff is the redelivery delay; the zero value means consumer.DefaultBackoff. Tests shorten it.
	Backoff consumer.Backoff
}

// ConsumerConfigs returns the durable consumer definitions: events, recipients, schedules.
func ConsumerConfigs() (ev, rc, sc jetstream.ConsumerConfig) {
	ev = consumer.ConsumerConfig(EventsConsumer, EventsFilter)
	ev.MaxAckPending = eventsMaxAckPending
	rc = consumer.ConsumerConfig(RecipientsConsumer, RecipientsFilter)
	rc.AckWait = recipientsAckWait
	rc.MaxAckPending = recipientsMaxAckPending
	sc = consumer.ConsumerConfig(SchedulesConsumer, FireFilter)
	sc.AckWait = schedulesAckWait
	sc.MaxAckPending = schedulesMaxAckPending
	return ev, rc, sc
}

// Start creates the three durable consumers and starts consuming. The streams must exist. It returns
// the function that stops them: it drains each consumer, waiting for the handlers in flight, until
// ctx ends. Messages are processed with a context that ctx's cancellation does not reach, so a
// handler in flight finishes its transaction when the service is told to stop.
func Start(ctx context.Context, d Deps) (stop func(context.Context) error, err error) {
	if d.Links == nil {
		return nil, errors.New("intake: a links client is required")
	}
	evCfg, rcCfg, scCfg := ConsumerConfigs()
	procCtx, cancelProc := context.WithCancel(context.WithoutCancel(ctx))

	evHandler := &Events{Logger: d.Logger}
	rcHandler := &Recipients{Logger: d.Logger, Links: d.Links}
	scHandler := &Schedules{Logger: d.Logger}
	evProc := &consumer.Processor{Pool: d.Pool, Logger: d.Logger, Backoff: d.Backoff, Handler: evHandler.Handle}
	rcProc := &consumer.RawProcessor{Pool: d.Pool, Logger: d.Logger, Backoff: d.Backoff, Handler: rcHandler.Handle}
	scProc := &consumer.RawProcessor{Pool: d.Pool, Logger: d.Logger, Backoff: d.Backoff, Handler: scHandler.Handle}
	schedules, err := d.JS.Stream(ctx, SchedulesStream)
	if err != nil {
		cancelProc()
		return nil, fmt.Errorf("stream %s: %w", SchedulesStream, err)
	}
	scProc.AfterAck = deleteFired(d.Logger, schedules)

	var running []jetstream.ConsumeContext
	fail := func(err error) (func(context.Context) error, error) {
		for _, cc := range running {
			cc.Stop()
		}
		cancelProc()
		return nil, err
	}
	for _, c := range []struct {
		stream  string
		cfg     jetstream.ConsumerConfig
		process func(context.Context, jetstream.Msg) error
	}{
		{EventsStream, evCfg, evProc.Process},
		{RecipientsStream, rcCfg, rcProc.Process},
		{SchedulesStream, scCfg, scProc.Process},
	} {
		cons, err := d.JS.CreateOrUpdateConsumer(ctx, c.stream, c.cfg)
		if err != nil {
			return fail(fmt.Errorf("create consumer %s on %s: %w", c.cfg.Durable, c.stream, err))
		}
		process, name := c.process, c.cfg.Durable
		cc, err := cons.Consume(func(msg jetstream.Msg) {
			// Process settles the message itself; an error it returns is a failed attempt that was nak'd.
			if err := process(procCtx, msg); err != nil {
				d.Logger.Error("message failed, will be redelivered", "consumer", name, "subject", msg.Subject(), "error", err)
			}
		})
		if err != nil {
			return fail(fmt.Errorf("consume %s: %w", name, err))
		}
		running = append(running, cc)
		d.Logger.Info("consumer started", "consumer", name, "stream", c.stream, "filter", c.cfg.FilterSubject,
			"max_ack_pending", c.cfg.MaxAckPending, "ack_wait", c.cfg.AckWait.String(), "max_deliver", c.cfg.MaxDeliver)
	}

	return func(ctx context.Context) error {
		defer cancelProc()
		var errs []error
		for _, cc := range running {
			cc.Drain()
		}
		for _, cc := range running {
			select {
			case <-cc.Closed():
			case <-ctx.Done():
				errs = append(errs, fmt.Errorf("wait for consumers to drain: %w", ctx.Err()))
				return errors.Join(errs...)
			}
		}
		return errors.Join(errs...)
	}, nil
}

// deleteFired returns the AfterAck hook of the SCHEDULES consumer: once a firing is committed and
// acked, its message is deleted from the stream, since the limits stream would keep it forever
// (D-078). The pending sched.* messages are other messages and stay. A failed delete is logged at
// warn level and ignored; the firing is done and a repeat of it is a no-op.
func deleteFired(logger *slog.Logger, stream jetstream.Stream) func(context.Context, jetstream.Msg) {
	return func(ctx context.Context, msg jetstream.Msg) {
		meta, err := msg.Metadata()
		if err != nil {
			logger.WarnContext(ctx, "fired message not deleted: no metadata", slog.String("subject", msg.Subject()), slog.Any("error", err))
			return
		}
		if err := stream.DeleteMsg(ctx, meta.Sequence.Stream); err != nil {
			logger.WarnContext(ctx, "fired message not deleted", slog.String("subject", msg.Subject()),
				slog.Uint64("sequence", meta.Sequence.Stream), slog.Any("error", err))
		}
	}
}
