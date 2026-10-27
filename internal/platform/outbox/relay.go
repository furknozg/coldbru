package outbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	defaultPollInterval = time.Second
	defaultBatchSize    = 100
	reconnectPause      = time.Second
	notifyChannel       = "outbox"
)

// Relay moves unpublished outbox rows to JetStream. A trigger on the outbox
// table sends NOTIFY on every insert statement and the Relay LISTENs on a
// dedicated connection; PollInterval is the fallback when no notification
// arrives (D-054).
//
// Publishing is at least once: if the process dies between PubAck and the
// commit that sets published_at, the row is published again. The outbox id is
// sent as Nats-Msg-Id, so JetStream drops the repeat inside its dedup window,
// and consumers dedup on processed_event beyond it.
type Relay struct {
	Pool         *pgxpool.Pool
	JS           jetstream.JetStream
	Logger       *slog.Logger
	PollInterval time.Duration // default 1s
	BatchSize    int           // default 100
}

func (r *Relay) pollInterval() time.Duration {
	if r.PollInterval > 0 {
		return r.PollInterval
	}
	return defaultPollInterval
}

func (r *Relay) batchSize() int {
	if r.BatchSize > 0 {
		return r.BatchSize
	}
	return defaultBatchSize
}

// Run publishes outbox rows until ctx is done, then returns nil. A broken
// LISTEN connection is logged and replaced after a short pause; Run does not
// exit for it.
func (r *Relay) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		err := r.serve(ctx)
		if ctx.Err() != nil {
			break
		}
		r.Logger.ErrorContext(ctx, "outbox relay connection lost", slog.Any("error", err))
		select {
		case <-ctx.Done():
		case <-time.After(reconnectPause):
		}
	}
	return nil
}

// serve holds one LISTEN connection and publishes until it breaks or ctx ends.
func (r *Relay) serve(ctx context.Context) error {
	pc, err := r.Pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire listen connection: %w", err)
	}
	// Take the connection out of the pool: a LISTENing session must not be
	// handed to other callers. The pool opens a replacement on demand.
	conn := pc.Hijack()
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = conn.Close(closeCtx) // best effort; the session is discarded either way
	}()

	if _, err := conn.Exec(ctx, "LISTEN "+notifyChannel); err != nil {
		return fmt.Errorf("listen %s: %w", notifyChannel, err)
	}
	r.Logger.InfoContext(ctx, "outbox relay listening")

	for {
		if err := r.drain(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			// Logged once here; the batch is retried on the next wake-up.
			r.Logger.ErrorContext(ctx, "outbox batch failed", slog.Any("error", err))
		}

		r.Logger.DebugContext(ctx, "outbox relay waiting")
		if err := r.wait(ctx, conn); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
	}
}

// wait blocks until a notification arrives or the poll interval passes. It
// returns an error only when the connection is unusable or ctx is done.
func (r *Relay) wait(ctx context.Context, conn *pgx.Conn) error {
	wctx, cancel := context.WithTimeout(ctx, r.pollInterval())
	defer cancel()
	_, err := conn.WaitForNotification(wctx)
	if err == nil {
		return nil
	}
	if wctx.Err() != nil && ctx.Err() == nil && !conn.IsClosed() {
		return nil // poll interval elapsed
	}
	return fmt.Errorf("wait for notification: %w", err)
}

// drain publishes batches until one returns fewer rows than the batch size.
// A publish failure ends the drain with an error; successful rows of that
// batch are already marked.
func (r *Relay) drain(ctx context.Context) error {
	size := r.batchSize()
	for {
		n, err := r.publishBatch(ctx, size)
		if err != nil {
			return err
		}
		if n < size {
			return nil
		}
	}
}

// publishBatch publishes up to size rows in one transaction and returns how
// many rows it read. Rows are published in created order; on the first publish
// error the rest of the batch is left for the next attempt (order is kept) and
// the rows already acknowledged are still marked published.
func (r *Relay) publishBatch(ctx context.Context, size int) (int, error) {
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin batch: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit

	rows, err := tx.Query(ctx, `
		SELECT id::text, subject, payload || jsonb_build_object('id', id::text)
		FROM outbox
		WHERE published_at IS NULL
		ORDER BY created_at, id
		LIMIT $1
		FOR UPDATE SKIP LOCKED`, size)
	if err != nil {
		return 0, fmt.Errorf("select outbox batch: %w", err)
	}
	type item struct {
		id, subject string
		data        []byte
	}
	var batch []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.subject, &it.data); err != nil {
			rows.Close()
			return 0, fmt.Errorf("scan outbox row: %w", err)
		}
		batch = append(batch, it)
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("read outbox batch: %w", err)
	}
	if len(batch) == 0 {
		return 0, nil
	}

	published := make([]string, 0, len(batch))
	var publishErr error
	for _, it := range batch {
		if _, err := r.JS.Publish(ctx, it.subject, it.data, jetstream.WithMsgID(it.id)); err != nil {
			publishErr = fmt.Errorf("publish outbox event %s to %s: %w", it.id, it.subject, err)
			break
		}
		published = append(published, it.id)
	}

	if len(published) > 0 {
		if _, err := tx.Exec(ctx,
			`UPDATE outbox SET published_at = now() WHERE id = ANY($1::uuid[])`, published); err != nil {
			return 0, errors.Join(publishErr, fmt.Errorf("mark outbox rows published: %w", err))
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, errors.Join(publishErr, fmt.Errorf("commit outbox batch: %w", err))
	}

	if publishErr != nil {
		r.Logger.WarnContext(ctx, "outbox batch partially published",
			slog.Int("read", len(batch)), slog.Int("published", len(published)))
		return 0, publishErr
	}
	r.Logger.InfoContext(ctx, "outbox batch published",
		slog.Int("read", len(batch)), slog.Int("published", len(published)))
	return len(batch), nil
}
