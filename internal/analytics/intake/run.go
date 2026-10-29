// Package intake is the Analytics service's event intake (D-083): one durable consumer of the
// EVENTS stream that keeps campaign summaries, delivery counts and LLM usage.
package intake

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/furknozg/coldbru/internal/analytics/pricing"
	"github.com/furknozg/coldbru/internal/platform/consumer"
)

// Deps are what the consumer needs.
type Deps struct {
	JS     jetstream.JetStream
	Pool   *pgxpool.Pool
	Logger *slog.Logger
	Prices pricing.Table
	// Backoff is the redelivery delay; the zero value means consumer.DefaultBackoff. Tests shorten it.
	Backoff consumer.Backoff
}

// ConsumerConfig returns the durable consumer definition. It delivers all messages of the stream
// from the start, so a campaign that completed before Analytics existed is picked up as long as
// EVENTS still holds its events (seven days).
func ConsumerConfig() jetstream.ConsumerConfig {
	cfg := consumer.ConsumerConfig(Durable, Filter)
	cfg.MaxAckPending = maxAckPending
	return cfg
}

// Start creates the durable consumer on the existing EVENTS stream and starts consuming. It returns
// the function that stops it: it drains the consumer, waiting for the handlers in flight, until ctx
// ends. Messages are processed with a context that ctx's cancellation does not reach, so a handler
// in flight finishes its transaction when the service is told to stop.
func Start(ctx context.Context, d Deps) (stop func(context.Context) error, err error) {
	cfg := ConsumerConfig()
	procCtx, cancelProc := context.WithCancel(context.WithoutCancel(ctx))

	handler := &Events{Logger: d.Logger, Prices: d.Prices}
	proc := &consumer.Processor{Pool: d.Pool, Logger: d.Logger, Backoff: d.Backoff, Handler: handler.Handle}

	cons, err := d.JS.CreateOrUpdateConsumer(ctx, Stream, cfg)
	if err != nil {
		cancelProc()
		return nil, fmt.Errorf("create consumer %s on %s: %w", Durable, Stream, err)
	}
	cc, err := cons.Consume(func(msg jetstream.Msg) {
		// Process settles the message itself; an error it returns is a failed attempt that was nak'd.
		if err := proc.Process(procCtx, msg); err != nil {
			d.Logger.Error("message failed, will be redelivered", "consumer", Durable, "subject", msg.Subject(), "error", err)
		}
	})
	if err != nil {
		cancelProc()
		return nil, fmt.Errorf("consume %s: %w", Durable, err)
	}
	d.Logger.Info("consumer started", "consumer", Durable, "stream", Stream, "filter", Filter,
		"max_ack_pending", cfg.MaxAckPending, "ack_wait", cfg.AckWait.String(), "max_deliver", cfg.MaxDeliver)

	return func(ctx context.Context) error {
		defer cancelProc()
		cc.Drain()
		select {
		case <-cc.Closed():
			return nil
		case <-ctx.Done():
			return errors.Join(fmt.Errorf("wait for consumer to drain: %w", ctx.Err()))
		}
	}, nil
}
