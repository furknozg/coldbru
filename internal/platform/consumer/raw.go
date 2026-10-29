package consumer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go/jetstream"
)

// TxBeginner starts the transaction of one message. *pgxpool.Pool implements it; tests use a fake.
type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// RawHandler applies one message that is not an event envelope (D-078): a recipient batch or a
// schedule firing. It must do all its database writes through tx; if it returns an error the
// transaction rolls back. Repeats must be harmless by the handler's own business key, because a raw
// message has no processed_event row: a duplicate has to turn into a no-op inside the handler.
//
// A handler may return RetryLater (the message cannot be applied yet) or Malformed (it never can be);
// any other error is a failure and is retried with Backoff.
type RawHandler func(ctx context.Context, tx pgx.Tx, msg jetstream.Msg) error

// retryLaterError asks for a delayed redelivery without being a failure.
type retryLaterError struct {
	delay  time.Duration
	reason string
}

func (e *retryLaterError) Error() string { return "retry later: " + e.reason }

// RetryLater is the error a RawHandler returns when the message cannot be applied yet, for example
// a recipient batch whose campaign event has not been processed. The transaction rolls back and the
// message is nak'd with delay; a zero or negative delay means the Backoff delay of the attempt. It is
// logged at debug level only, not as a failure, but it uses a delivery like any other: at MaxDeliver
// the message is logged and terminated.
func RetryLater(delay time.Duration, reason string) error {
	return &retryLaterError{delay: delay, reason: reason}
}

// malformedError marks a message that can never succeed.
type malformedError struct{ cause error }

func (e *malformedError) Error() string { return "malformed message: " + e.cause.Error() }
func (e *malformedError) Unwrap() error { return e.cause }

// Malformed wraps err as the error a RawHandler returns for a message that can never succeed, for
// example invalid JSON or a recipient no schedule covers. The transaction rolls back and the
// message is logged at error level and terminated without a retry. A nil err gets a generic cause.
func Malformed(err error) error {
	if err == nil {
		err = errors.New("unspecified")
	}
	return &malformedError{cause: err}
}

// RawProcessor runs a RawHandler for every JetStream message it is given, the counterpart of
// Processor for subjects that carry no envelope (D-078). It records nothing in processed_event.
type RawProcessor struct {
	Pool    TxBeginner
	Logger  *slog.Logger
	Backoff Backoff // zero value means DefaultBackoff
	Handler RawHandler
	// AfterAck, if set, runs after a message whose handler succeeded was committed and acked (not
	// after RetryLater, Malformed or a failure). It is best effort cleanup: it returns nothing and
	// must log its own errors.
	AfterAck func(ctx context.Context, msg jetstream.Msg)
}

// Process handles one message and settles it with JetStream. It can be used directly as the
// callback of jetstream.Consumer.Consume.
//
// Outcomes:
//   - handler success: committed, then acked, then AfterAck if set.
//   - Malformed: rolled back, logged at error level, terminated, nil returned.
//   - RetryLater: rolled back, nak'd with the delay, nil returned; at MaxDeliver it is logged and
//     terminated instead.
//   - any other failure (handler or database): rolled back; below MaxDeliver nak'd with Backoff
//     delay and the error returned for the caller to log; at MaxDeliver it is logged here and
//     terminated, nil returned.
//
// The ack always follows the commit, so a crash in between causes a redelivery, which the handler's
// business key must turn into a no-op. A returned error is not logged by Process (single handling
// rule).
func (p *RawProcessor) Process(ctx context.Context, msg jetstream.Msg) error {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return p.fail(ctx, msg, fmt.Errorf("begin transaction: %w", err))
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }() // no-op after Commit

	if err := p.Handler(ctx, tx, msg); err != nil {
		var retry *retryLaterError
		var bad *malformedError
		switch {
		case errors.As(err, &bad):
			_ = tx.Rollback(context.WithoutCancel(ctx))
			p.Logger.ErrorContext(ctx, "malformed message terminated",
				slog.String("subject", msg.Subject()), slog.Any("error", bad.cause))
			if err := msg.Term(); err != nil {
				return fmt.Errorf("terminate message: %w", err)
			}
			return nil
		case errors.As(err, &retry):
			_ = tx.Rollback(context.WithoutCancel(ctx))
			return p.retryLater(ctx, msg, retry)
		}
		return p.fail(ctx, msg, fmt.Errorf("handle %s: %w", msg.Subject(), err))
	}
	if err := tx.Commit(ctx); err != nil {
		return p.fail(ctx, msg, fmt.Errorf("commit message on %s: %w", msg.Subject(), err))
	}
	if err := ack(msg); err != nil {
		return err
	}
	if p.AfterAck != nil {
		p.AfterAck(ctx, msg)
	}
	return nil
}

func (p *RawProcessor) backoff() Backoff {
	if p.Backoff == (Backoff{}) {
		return DefaultBackoff
	}
	return p.Backoff
}

// attempt returns the delivery number of msg (1 for the first) and whether it could be read.
func attempt(msg jetstream.Msg) (int, error) {
	meta, err := msg.Metadata()
	if err != nil {
		return 1, err
	}
	return int(meta.NumDelivered), nil
}

// retryLater defers a message that is waiting for something, without logging a failure.
func (p *RawProcessor) retryLater(ctx context.Context, msg jetstream.Msg, r *retryLaterError) error {
	return deferMessage(ctx, p.Logger, p.backoff(), msg, r)
}

// deferMessage settles a message whose handler returned RetryLater: nak with the handler's delay (or
// the backoff of the attempt), or log and terminate at MaxDeliver. Shared by Processor and RawProcessor.
func deferMessage(ctx context.Context, logger *slog.Logger, backoff Backoff, msg jetstream.Msg, r *retryLaterError) error {
	n, metaErr := attempt(msg)
	if metaErr == nil && n >= MaxDeliver {
		logger.ErrorContext(ctx, "message still not applicable, delivery limit reached, terminated",
			slog.String("subject", msg.Subject()), slog.Int("attempt", n), slog.String("reason", r.reason))
		if err := msg.Term(); err != nil {
			return fmt.Errorf("terminate message: %w", err)
		}
		return nil
	}
	delay := r.delay
	if delay <= 0 {
		delay = backoff.Delay(n)
	}
	logger.DebugContext(ctx, "message deferred",
		slog.String("subject", msg.Subject()), slog.Int("attempt", n),
		slog.Duration("delay", delay), slog.String("reason", r.reason))
	if err := msg.NakWithDelay(delay); err != nil {
		return fmt.Errorf("nak message: %w", err)
	}
	return metaErr
}

// fail settles a message whose transaction was rolled back: delayed nak while attempts remain,
// terminate on the last one.
func (p *RawProcessor) fail(ctx context.Context, msg jetstream.Msg, cause error) error {
	n, metaErr := attempt(msg)
	if metaErr == nil && n >= MaxDeliver {
		p.Logger.ErrorContext(ctx, "message failed, delivery limit reached, terminated",
			slog.String("subject", msg.Subject()), slog.Int("attempt", n), slog.Any("error", cause))
		if err := msg.Term(); err != nil {
			return fmt.Errorf("terminate message: %w", err)
		}
		return nil
	}
	if err := msg.NakWithDelay(p.backoff().Delay(n)); err != nil {
		return errors.Join(cause, fmt.Errorf("nak message: %w", err))
	}
	return errors.Join(cause, metaErr)
}
