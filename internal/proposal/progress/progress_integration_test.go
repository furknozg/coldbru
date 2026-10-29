//go:build integration

package progress_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
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
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/progress"
	"github.com/furknozg/coldbru/internal/proposal/store"
)

var (
	baseDSN   string
	adminPool *pgxpool.Pool
	natsURL   string

	dbSeq   int
	dbSeqMu sync.Mutex
)

func TestMain(m *testing.M) { os.Exit(run(m)) }

func run(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pg, err := tcpostgres.Run(ctx, "postgres:18-alpine", tcpostgres.BasicWaitStrategies())
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres container: %v\n", err)
		return 1
	}
	defer func() { _ = testcontainers.TerminateContainer(pg) }()
	ns, err := tcnats.Run(ctx, "nats:2.15-alpine")
	if err != nil {
		fmt.Fprintf(os.Stderr, "start nats container: %v\n", err)
		return 1
	}
	defer func() { _ = testcontainers.TerminateContainer(ns) }()
	if baseDSN, err = pg.ConnectionString(ctx, "sslmode=disable"); err != nil {
		fmt.Fprintf(os.Stderr, "postgres connection string: %v\n", err)
		return 1
	}
	if natsURL, err = ns.ConnectionString(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "nats connection string: %v\n", err)
		return 1
	}
	if adminPool, err = db.Open(ctx, baseDSN); err != nil {
		fmt.Fprintf(os.Stderr, "open admin pool: %v\n", err)
		return 1
	}
	defer adminPool.Close()
	return m.Run()
}

// logCapture keeps the warnings and errors the consumer logs.
type logCapture struct {
	mu   sync.Mutex
	msgs []string
}

func (l *logCapture) Enabled(_ context.Context, lv slog.Level) bool { return lv >= slog.LevelWarn }
func (l *logCapture) WithGroup(string) slog.Handler                 { return l }
func (l *logCapture) WithAttrs([]slog.Attr) slog.Handler            { return l }
func (l *logCapture) Handle(_ context.Context, r slog.Record) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.msgs = append(l.msgs, r.Level.String()+" "+r.Message)
	return nil
}

func (l *logCapture) has(prefix string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, m := range l.msgs {
		if strings.HasPrefix(m, prefix) {
			return true
		}
	}
	return false
}

// harness is one test's proposal database, EVENTS stream, hub and running consumer.
type harness struct {
	t    *testing.T
	pool *pgxpool.Pool
	js   jetstream.JetStream
	hub  *flow.Hub
	q    *store.Queries
	logs *logCapture
	// lastSeq is the stream sequence of the newest published event.
	lastSeq uint64
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	ctx := t.Context()
	dbSeqMu.Lock()
	dbSeq++
	name := fmt.Sprintf("progress_test_%d", dbSeq)
	dbSeqMu.Unlock()
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	pool, err := db.Open(ctx, strings.Replace(baseDSN, "/postgres?", "/"+name+"?", 1))
	if err != nil {
		t.Fatalf("db.Open() error = %v", err)
	}
	t.Cleanup(pool.Close)
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := migrate.UpPlatform(ctx, pool, quiet); err != nil {
		t.Fatalf("UpPlatform() error = %v", err)
	}
	if err := migrate.Up(ctx, pool, store.Migrations(), quiet); err != nil {
		t.Fatalf("Up() error = %v", err)
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
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "infra", "nats", "streams", "EVENTS.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg jetstream.StreamConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := js.CreateStream(ctx, cfg); err != nil {
		t.Fatalf("create stream: %v", err)
	}
	t.Cleanup(func() { _ = js.DeleteStream(context.WithoutCancel(ctx), cfg.Name) })

	h := &harness{t: t, pool: pool, js: js, hub: flow.NewHub(0), q: store.New(pool), logs: &logCapture{}}
	stop, err := progress.Start(ctx, progress.Deps{
		JS: js, Pool: pool, Hub: h.hub, Logger: slog.New(h.logs),
		Backoff: consumer.Backoff{Base: 20 * time.Millisecond, Factor: 1.5, Max: 150 * time.Millisecond},
	})
	if err != nil {
		t.Fatalf("progress.Start() error = %v", err)
	}
	t.Cleanup(func() {
		sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		if err := stop(sctx); err != nil {
			t.Errorf("stop consumer: %v", err)
		}
	})
	return h
}

// campaign creates a campaign in the given status.
func (h *harness) campaign(status string) uuid.UUID {
	h.t.Helper()
	c, err := h.q.CreateCampaign(h.t.Context(), "brief")
	if err != nil {
		h.t.Fatal(err)
	}
	if status != flow.CampaignStatusDrafting {
		if _, err := h.q.SetCampaignStatus(h.t.Context(), store.SetCampaignStatusParams{ID: c.ID, Status: status}); err != nil {
			h.t.Fatal(err)
		}
	}
	return c.ID
}

func (h *harness) status(id uuid.UUID) string {
	h.t.Helper()
	c, err := h.q.GetCampaign(h.t.Context(), id)
	if err != nil {
		h.t.Fatal(err)
	}
	return c.Status
}

// envelope builds a Notification envelope; each call has a new event id.
func envelope(t *testing.T, typ string, campaignID uuid.UUID, data any) events.Envelope {
	t.Helper()
	env, err := events.New(typ, progress.EventVersion, "notification", campaignID.String(), data)
	if err != nil {
		t.Fatal(err)
	}
	env.ID = uuid.Must(uuid.NewV7()).String()
	return env
}

func started(t *testing.T, id uuid.UUID, tz string) events.Envelope {
	return envelope(t, progress.EventTypeStarted, id, progress.StartedData{
		CampaignID: id.String(), ScheduleID: uuid.NewString(), Language: "tr", Timezone: tz, StartedAt: time.Now().UTC(),
	})
}

func completed(t *testing.T, id uuid.UUID) events.Envelope {
	return envelope(t, progress.EventTypeCompleted, id, progress.CompletedData{
		CampaignID: id.String(), CompletedAt: time.Now().UTC(), Counts: []progress.Count{{Channel: "sms", Status: "sent", Count: 3}},
		SMSSegments: 3, EstimatedCostGBP: "0.1200",
	})
}

// publish sends the envelope to EVENTS; a repeat of the same envelope gets a new stream message but
// keeps its event id, like a redelivery after a lost ack would.
func (h *harness) publish(env events.Envelope) {
	h.t.Helper()
	subject, err := events.Subject(env)
	if err != nil {
		h.t.Fatal(err)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		h.t.Fatal(err)
	}
	ack, err := h.js.Publish(h.t.Context(), subject, raw)
	if err != nil {
		h.t.Fatalf("publish %s: %v", subject, err)
	}
	h.lastSeq = ack.Sequence
}

// idle waits until the consumer has acked everything published so far.
func (h *harness) idle() {
	h.t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		cons, err := h.js.Consumer(h.t.Context(), progress.Stream, progress.Durable)
		if err == nil {
			if info, err := cons.Info(h.t.Context()); err == nil && info.AckFloor.Stream >= h.lastSeq && info.NumAckPending == 0 {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatalf("the consumer did not ack up to stream sequence %d in time", h.lastSeq)
}

// statusEvent waits for the next campaign_status event on sub.
func statusEvent(t *testing.T, sub *flow.Subscription) flow.CampaignStatus {
	t.Helper()
	select {
	case ev := <-sub.C:
		cs, ok := ev.(flow.CampaignStatus)
		if !ok {
			t.Fatalf("event = %T %+v, want campaign_status", ev, ev)
		}
		return cs
	case <-time.After(10 * time.Second):
		t.Fatal("no campaign_status event within 10s")
		return flow.CampaignStatus{}
	}
}

// noEvent fails if an event arrives shortly.
func noEvent(t *testing.T, sub *flow.Subscription) {
	t.Helper()
	select {
	case ev := <-sub.C:
		t.Fatalf("unexpected event %T %+v", ev, ev)
	case <-time.After(200 * time.Millisecond):
	}
}

func TestStartedThenCompletedMovesTheCampaign(t *testing.T) {
	h := newHarness(t)
	id := h.campaign(flow.CampaignStatusScheduled)
	sub := h.hub.Subscribe(id)
	defer sub.Close()

	h.publish(started(t, id, "Europe/Istanbul"))
	if ev := statusEvent(t, sub); ev.Status != "sending" || ev.CampaignID != id || ev.Type() != "campaign_status" || ev.At.IsZero() {
		t.Errorf("event = %+v", ev)
	}
	if got := h.status(id); got != "sending" {
		t.Errorf("status = %s, want sending", got)
	}

	// The second schedule of the campaign fires: nothing changes, nothing is announced.
	h.publish(started(t, id, "Europe/London"))
	h.idle()
	noEvent(t, sub)
	if got := h.status(id); got != "sending" {
		t.Errorf("status after the second started = %s, want sending", got)
	}

	done := completed(t, id)
	h.publish(done)
	if ev := statusEvent(t, sub); ev.Status != "completed" {
		t.Errorf("event = %+v", ev)
	}
	if got := h.status(id); got != "completed" {
		t.Errorf("status = %s, want completed", got)
	}

	// The same event again (redelivery) and a late started event change nothing.
	h.publish(done)
	h.publish(started(t, id, "Europe/Istanbul"))
	h.idle()
	noEvent(t, sub)
	if got := h.status(id); got != "completed" {
		t.Errorf("status after the repeats = %s, want completed", got)
	}
	if n := h.processed(done.ID); n != 1 {
		t.Errorf("processed_event rows for the completed event = %d, want 1", n)
	}
}

func (h *harness) processed(eventID string) int {
	h.t.Helper()
	var n int
	if err := h.pool.QueryRow(h.t.Context(), `SELECT count(*) FROM processed_event WHERE event_id = $1::uuid`, eventID).Scan(&n); err != nil {
		h.t.Fatal(err)
	}
	return n
}

func TestCompletedWithoutStartedStillCompletes(t *testing.T) {
	h := newHarness(t)
	id := h.campaign(flow.CampaignStatusScheduled)
	sub := h.hub.Subscribe(id)
	defer sub.Close()

	h.publish(completed(t, id))
	if ev := statusEvent(t, sub); ev.Status != "completed" {
		t.Errorf("event = %+v", ev)
	}
	if got := h.status(id); got != "completed" {
		t.Errorf("status = %s, want completed", got)
	}
}

func TestEventsOfACancelledCampaignAreIgnored(t *testing.T) {
	h := newHarness(t)
	id := h.campaign(flow.CampaignStatusCancelled)
	sub := h.hub.Subscribe(id)
	defer sub.Close()

	h.publish(started(t, id, "Europe/Istanbul"))
	h.publish(completed(t, id))
	h.idle()
	noEvent(t, sub)
	if got := h.status(id); got != "cancelled" {
		t.Errorf("status = %s, want cancelled", got)
	}
	// A campaign that is still a draft does not move either.
	draft := h.campaign(flow.CampaignStatusDrafting)
	h.publish(started(t, draft, "Europe/Istanbul"))
	h.publish(completed(t, draft))
	h.idle()
	if got := h.status(draft); got != "drafting" {
		t.Errorf("draft status = %s, want drafting", got)
	}
}

func TestEventsOfAnUnknownCampaignAreAckedWithAWarning(t *testing.T) {
	h := newHarness(t)
	ghost := uuid.Must(uuid.NewV7())
	h.publish(started(t, ghost, "Europe/Istanbul"))
	h.idle()
	if !h.logs.has("WARN notification event for an unknown campaign ignored") {
		t.Errorf("no warning logged: %v", h.logs.msgs)
	}

	// The consumer carries on with the next event.
	id := h.campaign(flow.CampaignStatusScheduled)
	sub := h.hub.Subscribe(id)
	defer sub.Close()
	h.publish(started(t, id, "Europe/Istanbul"))
	if ev := statusEvent(t, sub); ev.Status != "sending" {
		t.Errorf("event = %+v", ev)
	}
}

func TestMalformedAndUnknownEventsAreDropped(t *testing.T) {
	h := newHarness(t)
	id := h.campaign(flow.CampaignStatusScheduled)
	sub := h.hub.Subscribe(id)
	defer sub.Close()

	bad := envelope(t, progress.EventTypeStarted, id, map[string]string{"campaign_id": "not-a-uuid"})
	other := envelope(t, "notification.something_else", id, map[string]string{"campaign_id": id.String()})
	h.publish(bad)
	h.publish(other)
	h.idle()
	noEvent(t, sub)
	if got := h.status(id); got != "scheduled" {
		t.Errorf("status = %s, want scheduled", got)
	}
}
