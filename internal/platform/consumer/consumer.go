// Package consumer processes JetStream events exactly once in effect (D-023,
// D-054): each event is recorded in processed_event in the same transaction as
// the handler's own writes, and redelivery is driven by NakWithDelay.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/furknozg/coldbru/internal/platform/events"
)

// ackWait is how long JetStream waits for an ack before redelivering.
const ackWait = 30 * time.Second

// invalidTextRepresentation is the Postgres error code for a malformed uuid.
const invalidTextRepresentation = "22P02"

// Handler applies one event. It must do all its database writes through tx: if
// it returns an error the transaction, including the processed_event row,
// rolls back and the event is redelivered.
type Handler func(ctx context.Context, tx pgx.Tx, e events.Envelope) error

// ConsumerConfig returns the durable pull consumer definition for a service:
// explicit acks, MaxDeliver attempts and a 30 second ack wait. Redelivery
// delays are set by the Processor with NakWithDelay, not by the server
// (D-054).
func ConsumerConfig(durable, filterSubject string) jetstream.ConsumerConfig {
	return jetstream.ConsumerConfig{
		Durable:       durable,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    MaxDeliver,
		AckWait:       ackWait,
		FilterSubject: filterSubject,
	}
}

// Processor runs a Handler for every JetStream message it is given.
type Processor struct {
	Pool    *pgxpool.Pool
	Logger  *slog.Logger
	Backoff Backoff // zero value means DefaultBackoff
	Handler Handler
}

// Process handles one message and settles it with JetStream. It can be used
// directly as the callback of jetstream.Consumer.Consume.
//
// Outcomes:
//   - invalid JSON or empty event id: logged and terminated, nil returned.
//   - event already in processed_event: committed, acked, nil returned.
//   - handler success: committed, then acked.
//   - failure (handler or database): rolled back; below MaxDeliver the message
//     is nak'd with Backoff delay and the error is returned for the caller to
//     log; at MaxDeliver it is logged here and terminated, nil returned.
//
// The ack always follows the commit, so a crash in between causes a
// redelivery that processed_event turns into a no-op. A returned error is not
// logged by Process (single handling rule).
func (p *Processor) Process(ctx context.Context, msg jetstream.Msg) error {
	var env events.Envelope
	if err := json.Unmarshal(msg.Data(), &env); err != nil {
		return p.terminateMalformed(ctx, msg, "invalid json", err)
	}
	if env.ID == "" {
		return p.terminateMalformed(ctx, msg, "empty event id", nil)
	}

	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return p.fail(ctx, msg, env, fmt.Errorf("begin transaction: %w", err))
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }() // no-op after Commit

	tag, err := tx.Exec(ctx,
		`INSERT INTO processed_event (event_id) VALUES ($1::uuid) ON CONFLICT DO NOTHING`, env.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == invalidTextRepresentation {
			return p.terminateMalformed(ctx, msg, "event id is not a uuid", err)
		}
		return p.fail(ctx, msg, env, fmt.Errorf("record processed event: %w", err))
	}
	if tag.RowsAffected() == 0 {
		if err := tx.Commit(ctx); err != nil {
			return p.fail(ctx, msg, env, fmt.Errorf("commit duplicate check: %w", err))
		}
		p.Logger.InfoContext(ctx, "duplicate event skipped",
			slog.String("event_id", env.ID), slog.String("type", env.Type))
		return ack(msg)
	}

	if err := p.Handler(ctx, tx, env); err != nil {
		return p.fail(ctx, msg, env, fmt.Errorf("handle %s %s: %w", env.Type, env.ID, err))
	}
	if err := tx.Commit(ctx); err != nil {
		return p.fail(ctx, msg, env, fmt.Errorf("commit event %s: %w", env.ID, err))
	}
	return ack(msg)
}

func ack(msg jetstream.Msg) error {
	if err := msg.Ack(); err != nil {
		return fmt.Errorf("ack message: %w", err)
	}
	return nil
}

// terminateMalformed drops a message that can never succeed.
func (p *Processor) terminateMalformed(ctx context.Context, msg jetstream.Msg, reason string, cause error) error {
	attrs := []any{slog.String("reason", reason), slog.String("subject", msg.Subject())}
	if cause != nil {
		attrs = append(attrs, slog.Any("error", cause))
	}
	p.Logger.ErrorContext(ctx, "malformed event terminated", attrs...)
	if err := msg.Term(); err != nil {
		return fmt.Errorf("terminate message: %w", err)
	}
	return nil
}

// fail settles a message whose transaction was rolled back: delayed nak while
// attempts remain, terminate on the last one.
func (p *Processor) fail(ctx context.Context, msg jetstream.Msg, env events.Envelope, cause error) error {
	attempt := 1
	meta, metaErr := msg.Metadata()
	if metaErr == nil {
		attempt = int(meta.NumDelivered)
	}

	if metaErr == nil && attempt >= MaxDeliver {
		p.Logger.ErrorContext(ctx, "event failed, delivery limit reached, terminated",
			slog.String("event_id", env.ID), slog.String("type", env.Type),
			slog.Int("attempt", attempt), slog.Any("error", cause))
		if err := msg.Term(); err != nil {
			return fmt.Errorf("terminate message: %w", err)
		}
		return nil
	}

	backoff := p.Backoff
	if backoff == (Backoff{}) {
		backoff = DefaultBackoff
	}
	if err := msg.NakWithDelay(backoff.Delay(attempt)); err != nil {
		return errors.Join(cause, fmt.Errorf("nak message: %w", err))
	}
	return errors.Join(cause, metaErr)
}
