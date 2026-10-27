//go:build integration

package outbox_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/testcontainers/testcontainers-go"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/furknozg/coldbru/internal/platform/db"
	"github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/platform/migrate"
	"github.com/furknozg/coldbru/internal/platform/outbox"
)

const streamName = "EVENTS"

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

// env is one test's isolated database (platform migrations applied) and EVENTS stream.
type env struct {
	pool   *pgxpool.Pool
	js     jetstream.JetStream
	stream jetstream.Stream
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := t.Context()

	dbSeqMu.Lock()
	dbSeq++
	name := fmt.Sprintf("outbox_test_%d", dbSeq)
	dbSeqMu.Unlock()
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	dsn := strings.Replace(baseDSN, "/postgres?", "/"+name+"?", 1)
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	t.Cleanup(pool.Close)
	if err := migrate.UpPlatform(ctx, pool, discardLogger()); err != nil {
		t.Fatalf("UpPlatform() error = %v", err)
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
	return &env{pool: pool, js: js, stream: stream}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(nopWriter{}, nil))
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }

// logBuffer is a concurrency safe slog sink that tests can poll.
type logBuffer struct {
	mu sync.Mutex
	sb strings.Builder
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.sb.Write(p)
}

func (b *logBuffer) count(s string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.Count(b.sb.String(), s)
}

func (b *logBuffer) logger() *slog.Logger {
	return slog.New(slog.NewTextHandler(b, &slog.HandlerOptions{Level: slog.LevelDebug}))
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

// startRelay runs r until the returned stop func is called (also on cleanup).
func startRelay(t *testing.T, r *outbox.Relay) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- r.Run(ctx) }()
	var once sync.Once
	stop = func() {
		once.Do(func() {
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("Relay.Run() error = %v", err)
				}
			case <-time.After(10 * time.Second):
				t.Error("Relay.Run() did not return after cancel")
			}
		})
	}
	t.Cleanup(stop)
	return stop
}

func writeEvent(t *testing.T, pool *pgxpool.Pool, typ string, commit bool) string {
	t.Helper()
	ctx := t.Context()
	e, err := events.New(typ, 1, "test", "corr-1", map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("events.New() error = %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	id, err := outbox.Write(ctx, tx, e)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("Write() error = %v", err)
	}
	if commit {
		err = tx.Commit(ctx)
	} else {
		err = tx.Rollback(ctx)
	}
	if err != nil {
		t.Fatalf("finish tx: %v", err)
	}
	return id
}

func streamMsgCount(t *testing.T, e *env) uint64 {
	t.Helper()
	info, err := e.stream.Info(t.Context())
	if err != nil {
		t.Fatalf("stream info: %v", err)
	}
	return info.State.Msgs
}

func isPublished(t *testing.T, e *env, id string) bool {
	t.Helper()
	var ok bool
	err := e.pool.QueryRow(t.Context(),
		`SELECT published_at IS NOT NULL FROM outbox WHERE id = $1::uuid`, id).Scan(&ok)
	if err != nil {
		t.Fatalf("query published_at: %v", err)
	}
	return ok
}

func TestRelay_NotifyWakesRelay(t *testing.T) {
	e := newEnv(t)
	var logs logBuffer
	startRelay(t, &outbox.Relay{Pool: e.pool, JS: e.js, Logger: logs.logger(), PollInterval: time.Hour})

	// "waiting" is logged after the first drain: from here only NOTIFY can wake the relay.
	eventually(t, 5*time.Second, "relay to idle", func() bool { return logs.count("outbox relay waiting") >= 1 })

	id := writeEvent(t, e.pool, "proposal.confirmed", true)

	eventually(t, 5*time.Second, "event on stream", func() bool { return streamMsgCount(t, e) == 1 })

	msg, err := e.stream.GetMsg(t.Context(), 1)
	if err != nil {
		t.Fatalf("GetMsg() error = %v", err)
	}
	if msg.Subject != "evt.proposal.confirmed.v1" {
		t.Errorf("subject = %q, want evt.proposal.confirmed.v1", msg.Subject)
	}
	if got := msg.Header.Get(jetstream.MsgIDHeader); got != id {
		t.Errorf("%s header = %q, want %q", jetstream.MsgIDHeader, got, id)
	}
	var env events.Envelope
	if err := json.Unmarshal(msg.Data, &env); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if env.ID != id {
		t.Errorf("envelope id = %q, want %q", env.ID, id)
	}
	if env.Type != "proposal.confirmed" || env.Version != 1 || env.Producer != "test" || env.CorrelationID != "corr-1" ||
		!jsonEqual(env.Data, `{"k":"v"}`) { // jsonb normalises whitespace
		t.Errorf("envelope = %+v", env)
	}
	eventually(t, 5*time.Second, "published_at set", func() bool { return isPublished(t, e, id) })
}

func TestRelay_PollPublishesBacklogInOrder(t *testing.T) {
	e := newEnv(t)
	types := []string{"proposal.first", "proposal.second", "proposal.third"}
	ids := make([]string, len(types))
	for i, typ := range types {
		ids[i] = writeEvent(t, e.pool, typ, true)
	}

	startRelay(t, &outbox.Relay{Pool: e.pool, JS: e.js, Logger: discardLogger(), PollInterval: 200 * time.Millisecond})

	eventually(t, 5*time.Second, "3 events on stream", func() bool { return streamMsgCount(t, e) == 3 })

	for i := range types {
		msg, err := e.stream.GetMsg(t.Context(), uint64(i+1))
		if err != nil {
			t.Fatalf("GetMsg(%d) error = %v", i+1, err)
		}
		if want := "evt." + types[i] + ".v1"; msg.Subject != want {
			t.Errorf("message %d subject = %q, want %q", i+1, msg.Subject, want)
		}
		if got := msg.Header.Get(jetstream.MsgIDHeader); got != ids[i] {
			t.Errorf("message %d msg id = %q, want %q", i+1, got, ids[i])
		}
	}
}

func TestRelay_BatchSizeDrainsBacklog(t *testing.T) {
	e := newEnv(t)
	const n = 7
	for range n {
		writeEvent(t, e.pool, "proposal.confirmed", true)
	}
	// PollInterval is long and BatchSize small: only draining can publish all of them.
	startRelay(t, &outbox.Relay{Pool: e.pool, JS: e.js, Logger: discardLogger(), PollInterval: time.Hour, BatchSize: 3})

	eventually(t, 5*time.Second, "all events on stream", func() bool { return streamMsgCount(t, e) == n })
}

func TestRelay_RolledBackWritePublishesNothing(t *testing.T) {
	e := newEnv(t)
	var logs logBuffer
	startRelay(t, &outbox.Relay{Pool: e.pool, JS: e.js, Logger: logs.logger(), PollInterval: 100 * time.Millisecond})

	writeEvent(t, e.pool, "proposal.confirmed", false)

	// Several polls after the rollback.
	eventually(t, 5*time.Second, "relay polls", func() bool { return logs.count("outbox relay waiting") >= 4 })

	if got := streamMsgCount(t, e); got != 0 {
		t.Errorf("stream messages = %d, want 0", got)
	}
	var rows int
	if err := e.pool.QueryRow(t.Context(), `SELECT count(*) FROM outbox`).Scan(&rows); err != nil {
		t.Fatalf("count outbox: %v", err)
	}
	if rows != 0 {
		t.Errorf("outbox rows = %d, want 0", rows)
	}
}

func TestRelay_RepublishIsDeduplicated(t *testing.T) {
	e := newEnv(t)
	id := writeEvent(t, e.pool, "proposal.confirmed", true)
	r := &outbox.Relay{Pool: e.pool, JS: e.js, Logger: discardLogger(), PollInterval: 100 * time.Millisecond}

	stop := startRelay(t, r)
	eventually(t, 5*time.Second, "first publish", func() bool { return isPublished(t, e, id) })
	stop()

	// Simulate a crash between PubAck and commit: the row looks unpublished again.
	if _, err := e.pool.Exec(t.Context(), `UPDATE outbox SET published_at = NULL`); err != nil {
		t.Fatalf("reset published_at: %v", err)
	}
	startRelay(t, r)
	eventually(t, 5*time.Second, "second publish", func() bool { return isPublished(t, e, id) })

	if got := streamMsgCount(t, e); got != 1 {
		t.Errorf("stream messages = %d, want 1 (Nats-Msg-Id dedup)", got)
	}
}

func TestWrite_InvalidEnvelope(t *testing.T) {
	e := newEnv(t)
	ctx := t.Context()
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = outbox.Write(ctx, tx, events.Envelope{Type: "Bad Type", Version: 1})
	if err == nil {
		t.Fatal("Write() error = nil, want subject validation error")
	}
}

func jsonEqual(raw json.RawMessage, want string) bool {
	var a, b any
	if json.Unmarshal(raw, &a) != nil || json.Unmarshal([]byte(want), &b) != nil {
		return false
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}
