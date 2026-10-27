//go:build integration

package consumer_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/testcontainers/testcontainers-go"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/furknozg/coldbru/internal/platform/consumer"
	"github.com/furknozg/coldbru/internal/platform/db"
	"github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/platform/migrate"
)

const (
	streamName = "EVENTS"
	subject    = "evt.test.thing.v1"
)

var (
	baseDSN   string
	adminPool *pgxpool.Pool
	natsURL   string

	dbSeq   int
	dbSeqMu sync.Mutex
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	pg, err := tcpostgres.Run(ctx, "postgres:18-alpine", tcpostgres.BasicWaitStrategies())
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres container: %v\n", err)
		return 1
	}
	defer func() {
		if err := testcontainers.TerminateContainer(pg); err != nil {
			fmt.Fprintf(os.Stderr, "terminate postgres container: %v\n", err)
		}
	}()
	ns, err := tcnats.Run(ctx, "nats:2.15-alpine")
	if err != nil {
		fmt.Fprintf(os.Stderr, "start nats container: %v\n", err)
		return 1
	}
	defer func() {
		if err := testcontainers.TerminateContainer(ns); err != nil {
			fmt.Fprintf(os.Stderr, "terminate nats container: %v\n", err)
		}
	}()

	baseDSN, err = pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres connection string: %v\n", err)
		return 1
	}
	natsURL, err = ns.ConnectionString(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "nats connection string: %v\n", err)
		return 1
	}
	adminPool, err = db.Open(ctx, baseDSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open admin pool: %v\n", err)
		return 1
	}
	defer adminPool.Close()

	return m.Run()
}

// settled counts how each message was settled with JetStream.
type settled struct {
	acks, naks, terms atomic.Int32
}

// spyMsg wraps a JetStream message and counts how it is settled.
type spyMsg struct {
	jetstream.Msg
	s *settled
}

func (m spyMsg) Ack() error { m.s.acks.Add(1); return m.Msg.Ack() }
func (m spyMsg) NakWithDelay(d time.Duration) error {
	m.s.naks.Add(1)
	return m.Msg.NakWithDelay(d)
}
func (m spyMsg) Term() error { m.s.terms.Add(1); return m.Msg.Term() }

type env struct {
	pool   *pgxpool.Pool
	js     jetstream.JetStream
	stream jetstream.Stream
	cons   jetstream.Consumer
	set    *settled
	errs   *errLog
}

// errLog collects errors Process returns (the caller logs them, per its contract).
type errLog struct {
	mu   sync.Mutex
	errs []error
}

func (l *errLog) add(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errs = append(l.errs, err)
}

func (l *errLog) len() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.errs)
}

// newEnv creates a database with platform migrations and a probe table, an
// EVENTS stream and a durable consumer. The returned Processor callback is
// started by consume.
func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := t.Context()

	dbSeqMu.Lock()
	dbSeq++
	name := fmt.Sprintf("consumer_test_%d", dbSeq)
	dbSeqMu.Unlock()
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	pool, err := db.Open(ctx, strings.Replace(baseDSN, "/postgres?", "/"+name+"?", 1))
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	t.Cleanup(pool.Close)
	if err := migrate.UpPlatform(ctx, pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatalf("UpPlatform() error = %v", err)
	}
	if _, err := pool.Exec(ctx, `CREATE TABLE side_effect (event_id uuid NOT NULL)`); err != nil {
		t.Fatalf("create side_effect: %v", err)
	}

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("nats.Connect() error = %v", err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatalf("jetstream.New() error = %v", err)
	}
	stream, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: streamName, Subjects: []string{"evt.>"}})
	if err != nil {
		t.Fatalf("create stream: %v", err)
	}
	t.Cleanup(func() {
		if err := js.DeleteStream(context.WithoutCancel(ctx), streamName); err != nil {
			t.Errorf("delete stream: %v", err)
		}
	})
	cons, err := stream.CreateOrUpdateConsumer(ctx, consumer.ConsumerConfig("svc", "evt.>"))
	if err != nil {
		t.Fatalf("create consumer: %v", err)
	}
	return &env{pool: pool, js: js, stream: stream, cons: cons, set: &settled{}, errs: &errLog{}}
}

var fastBackoff = consumer.Backoff{Base: 10 * time.Millisecond, Factor: 2, Max: 50 * time.Millisecond, Jitter: 0.2}

// consume starts consuming with a Processor running h and stops on cleanup.
func (e *env) consume(t *testing.T, h consumer.Handler) {
	t.Helper()
	p := &consumer.Processor{
		Pool:    e.pool,
		Logger:  slog.New(slog.NewTextHandler(io.Discard, nil)),
		Backoff: fastBackoff,
		Handler: h,
	}
	cc, err := e.cons.Consume(func(msg jetstream.Msg) {
		if err := p.Process(context.Background(), spyMsg{Msg: msg, s: e.set}); err != nil {
			e.errs.add(err)
		}
	})
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	t.Cleanup(cc.Stop)
}

// sideEffectHandler records the event in side_effect through the handler's tx.
func sideEffectHandler(calls *atomic.Int32, failFirst int32) consumer.Handler {
	return func(ctx context.Context, tx pgx.Tx, ev events.Envelope) error {
		n := calls.Add(1)
		if _, err := tx.Exec(ctx, `INSERT INTO side_effect (event_id) VALUES ($1::uuid)`, ev.ID); err != nil {
			return err
		}
		if n <= failFirst {
			return errors.New("boom")
		}
		return nil
	}
}

func (e *env) publish(t *testing.T, subj, body, msgID string) {
	t.Helper()
	var opts []jetstream.PublishOpt
	if msgID != "" {
		opts = append(opts, jetstream.WithMsgID(msgID))
	}
	if _, err := e.js.Publish(t.Context(), subj, []byte(body), opts...); err != nil {
		t.Fatalf("publish: %v", err)
	}
}

func envelopeJSON(id string) string {
	return fmt.Sprintf(`{"id":%q,"type":"test.thing","version":1,"occurred_at":"2026-10-27T09:00:00Z","producer":"test","correlation_id":"c1","data":{}}`, id)
}

func (e *env) count(t *testing.T, table string) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(t.Context(), "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func eventually(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s", timeout, what)
}

// quiet waits until the consumer has nothing pending or unacked.
func (e *env) quiet(t *testing.T) {
	t.Helper()
	eventually(t, 10*time.Second, "consumer to drain", func() bool {
		info, err := e.cons.Info(t.Context())
		return err == nil && info.NumPending == 0 && info.NumAckPending == 0 && info.Delivered.Stream > 0
	})
}

const eventID = "0199f3a2-7c4e-7b1a-9d2e-5f6a7b8c9d0e"

func TestProcess_DuplicateEventRunsHandlerOnce(t *testing.T) {
	e := newEnv(t)
	var calls atomic.Int32
	e.consume(t, sideEffectHandler(&calls, 0))

	// Different Msg-Ids, so JetStream does not dedup: the consumer sees the event twice.
	e.publish(t, subject, envelopeJSON(eventID), "first")
	e.publish(t, subject, envelopeJSON(eventID), "second")

	eventually(t, 10*time.Second, "both messages acked", func() bool { return e.set.acks.Load() == 2 })
	e.quiet(t)

	if got := calls.Load(); got != 1 {
		t.Errorf("handler calls = %d, want 1", got)
	}
	if got := e.count(t, "processed_event"); got != 1 {
		t.Errorf("processed_event rows = %d, want 1", got)
	}
	if got := e.count(t, "side_effect"); got != 1 {
		t.Errorf("side_effect rows = %d, want 1", got)
	}
	if e.set.naks.Load() != 0 || e.set.terms.Load() != 0 {
		t.Errorf("naks/terms = %d/%d, want 0/0", e.set.naks.Load(), e.set.terms.Load())
	}
	if got := e.errs.len(); got != 0 {
		t.Errorf("Process errors = %d, want 0", got)
	}
}

func TestProcess_HandlerFailsTwiceThenSucceeds(t *testing.T) {
	e := newEnv(t)
	var calls atomic.Int32
	e.consume(t, sideEffectHandler(&calls, 2))

	e.publish(t, subject, envelopeJSON(eventID), eventID)

	eventually(t, 10*time.Second, "message acked", func() bool { return e.set.acks.Load() == 1 })
	e.quiet(t)

	if got := calls.Load(); got != 3 {
		t.Errorf("handler calls = %d, want 3", got)
	}
	if got := e.set.naks.Load(); got != 2 {
		t.Errorf("naks = %d, want 2", got)
	}
	if got := e.count(t, "processed_event"); got != 1 {
		t.Errorf("processed_event rows = %d, want 1", got)
	}
	// The two failed attempts inserted too, but rolled back with their transaction.
	if got := e.count(t, "side_effect"); got != 1 {
		t.Errorf("side_effect rows = %d, want 1 (failed attempts rolled back)", got)
	}
	if got := e.errs.len(); got != 2 {
		t.Errorf("Process errors = %d, want 2 (one per failed attempt)", got)
	}
}

func TestProcess_AlwaysFailingIsTerminatedAtMaxDeliver(t *testing.T) {
	e := newEnv(t)
	var calls atomic.Int32
	e.consume(t, sideEffectHandler(&calls, 1<<30))

	e.publish(t, subject, envelopeJSON(eventID), eventID)

	eventually(t, 10*time.Second, "message terminated", func() bool { return e.set.terms.Load() == 1 })
	e.quiet(t)
	time.Sleep(300 * time.Millisecond) // a sixth delivery would show up here
	e.quiet(t)

	if got := calls.Load(); got != consumer.MaxDeliver {
		t.Errorf("handler calls = %d, want %d", got, consumer.MaxDeliver)
	}
	if got := e.set.naks.Load(); got != consumer.MaxDeliver-1 {
		t.Errorf("naks = %d, want %d", got, consumer.MaxDeliver-1)
	}
	if e.set.acks.Load() != 0 {
		t.Errorf("acks = %d, want 0", e.set.acks.Load())
	}
	if got := e.count(t, "processed_event"); got != 0 {
		t.Errorf("processed_event rows = %d, want 0 (every attempt rolled back)", got)
	}
	if got := e.count(t, "side_effect"); got != 0 {
		t.Errorf("side_effect rows = %d, want 0", got)
	}
}

func TestProcess_MalformedPayloadIsTerminated(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "not json", body: "this is not json"},
		{name: "empty id", body: envelopeJSON("")},
		{name: "id is not a uuid", body: envelopeJSON("not-a-uuid")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			var calls atomic.Int32
			e.consume(t, sideEffectHandler(&calls, 0))

			e.publish(t, subject, tt.body, "")

			eventually(t, 10*time.Second, "message terminated", func() bool { return e.set.terms.Load() == 1 })
			e.quiet(t)

			if got := calls.Load(); got != 0 {
				t.Errorf("handler calls = %d, want 0", got)
			}
			if e.set.acks.Load() != 0 || e.set.naks.Load() != 0 {
				t.Errorf("acks/naks = %d/%d, want 0/0", e.set.acks.Load(), e.set.naks.Load())
			}
			if got := e.count(t, "processed_event"); got != 0 {
				t.Errorf("processed_event rows = %d, want 0", got)
			}
		})
	}
}
