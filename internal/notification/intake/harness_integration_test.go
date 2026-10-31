//go:build integration

package intake_test

import (
	"context"
	"encoding/json"
	"errors"
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
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/testcontainers/testcontainers-go"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/furknozg/coldbru/internal/notification/intake"
	"github.com/furknozg/coldbru/internal/notification/links/linktest"
	"github.com/furknozg/coldbru/internal/notification/store"
	"github.com/furknozg/coldbru/internal/platform/consumer"
	"github.com/furknozg/coldbru/internal/platform/db"
	"github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/platform/migrate"
	"github.com/furknozg/coldbru/internal/platform/outbox"
	"github.com/furknozg/coldbru/internal/proposal/flow"
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

// streamDir holds the real stream definitions the compose stack loads (D-049).
var streamDir = filepath.Join("..", "..", "..", "infra", "nats", "streams")

// createStreams creates EVENTS, RECIPIENTS, SCHEDULES and SEND from infra/nats/streams/*.json and
// deletes them when the test ends.
func createStreams(t *testing.T, js jetstream.JetStream) map[string]jetstream.Stream {
	t.Helper()
	out := map[string]jetstream.Stream{}
	for _, name := range []string{"EVENTS", "RECIPIENTS", "SCHEDULES", "SEND"} {
		raw, err := os.ReadFile(filepath.Join(streamDir, name+".json"))
		if err != nil {
			t.Fatalf("read stream definition %s: %v", name, err)
		}
		var cfg jetstream.StreamConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Fatalf("parse stream definition %s: %v", name, err)
		}
		st, err := js.CreateStream(t.Context(), cfg)
		if err != nil {
			t.Fatalf("create stream %s: %v", name, err)
		}
		out[name] = st
		t.Cleanup(func() {
			if err := js.DeleteStream(context.WithoutCancel(t.Context()), name); err != nil {
				t.Errorf("delete stream %s: %v", name, err)
			}
		})
	}
	return out
}

// record is one captured log record.
type record struct {
	level slog.Level
	msg   string
	attrs map[string]any
}

// logCapture is a slog handler that keeps every record for assertions.
type logCapture struct {
	mu      sync.Mutex
	records []record
	attrs   []slog.Attr
}

func (l *logCapture) Enabled(context.Context, slog.Level) bool { return true }
func (l *logCapture) WithGroup(string) slog.Handler            { return l }
func (l *logCapture) WithAttrs(a []slog.Attr) slog.Handler {
	l.mu.Lock()
	defer l.mu.Unlock()
	return &logView{parent: l, attrs: a}
}
func (l *logCapture) Handle(_ context.Context, r slog.Record) error { return l.add(r, nil) }

func (l *logCapture) add(r slog.Record, extra []slog.Attr) error {
	rec := record{level: r.Level, msg: r.Message, attrs: map[string]any{}}
	for _, a := range extra {
		rec.attrs[a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		rec.attrs[a.Key] = a.Value.Any()
		return true
	})
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, rec)
	return nil
}

type logView struct {
	parent *logCapture
	attrs  []slog.Attr
}

func (v *logView) Enabled(context.Context, slog.Level) bool { return true }
func (v *logView) WithGroup(string) slog.Handler            { return v }
func (v *logView) WithAttrs(a []slog.Attr) slog.Handler {
	return &logView{parent: v.parent, attrs: append(append([]slog.Attr{}, v.attrs...), a...)}
}
func (v *logView) Handle(_ context.Context, r slog.Record) error { return v.parent.add(r, v.attrs) }

// find returns the records with message msg.
func (l *logCapture) find(msg string) []record {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []record
	for _, r := range l.records {
		if r.msg == msg {
			out = append(out, r)
		}
	}
	return out
}

// atLevel counts records at or above level.
func (l *logCapture) atLevel(level slog.Level) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, r := range l.records {
		if r.level >= level {
			n++
		}
	}
	return n
}

// env is one test's database (platform and notification migrations), streams, relay and consumers.
type env struct {
	pool    *pgxpool.Pool
	js      jetstream.JetStream
	streams map[string]jetstream.Stream
	logs    *logCapture
	q       *store.Queries
	// links is the fake redirect service the intake asks for the short link of every delivery.
	links *linktest.Fake
}

var fastBackoff = consumer.Backoff{Base: 20 * time.Millisecond, Factor: 1.5, Max: 150 * time.Millisecond}

// newEnv builds the whole Notification intake side: the same code cmd/notification starts.
func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := t.Context()

	dbSeqMu.Lock()
	dbSeq++
	name := fmt.Sprintf("intake_test_%d", dbSeq)
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
	streams := createStreams(t, js)
	logs := &logCapture{}
	logger := slog.New(logs)

	relay := &outbox.Relay{Pool: pool, JS: js, Logger: quiet, PollInterval: 50 * time.Millisecond}
	stopRelay := relay.Start(ctx)
	fakeLinks := linktest.New()
	stop, err := intake.Start(ctx, intake.Deps{JS: js, Pool: pool, Logger: logger, Links: fakeLinks, Backoff: fastBackoff})
	if err != nil {
		t.Fatalf("intake.Start() error = %v", err)
	}
	t.Cleanup(func() {
		// Consumers first, then the relay, as boot.Setup does.
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if err := stop(cctx); err != nil {
			t.Errorf("stop consumers: %v", err)
		}
		if err := stopRelay(cctx); err != nil {
			t.Errorf("stop relay: %v", err)
		}
	})
	return &env{pool: pool, js: js, streams: streams, logs: logs, q: store.New(pool), links: fakeLinks}
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

// never fails when cond becomes true within d.
func never(t *testing.T, d time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if cond() {
			t.Fatalf("%s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// publishEvent publishes a Proposal envelope the way Proposal's relay does: id from the outbox row.
func (e *env) publishEvent(t *testing.T, typ, campaignID string, data any) string {
	t.Helper()
	env, err := events.New(typ, flow.EventVersion, flow.EventProducer, campaignID, data)
	if err != nil {
		t.Fatalf("events.New() error = %v", err)
	}
	return e.publishEnvelope(t, env)
}

func (e *env) publishEnvelope(t *testing.T, env events.Envelope) string {
	t.Helper()
	if env.ID == "" {
		env.ID = uuid.Must(uuid.NewV7()).String()
	}
	subject, err := events.Subject(env)
	if err != nil {
		t.Fatalf("events.Subject() error = %v", err)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	if _, err := e.js.Publish(t.Context(), subject, raw, jetstream.WithMsgID(env.ID)); err != nil {
		t.Fatalf("publish %s: %v", subject, err)
	}
	return env.ID
}

func (e *env) publishBatch(t *testing.T, b flow.RecipientBatch) {
	t.Helper()
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal batch: %v", err)
	}
	e.publishRaw(t, flow.RecipientsSubject(b.CampaignID), raw)
}

func (e *env) publishRaw(t *testing.T, subject string, raw []byte) {
	t.Helper()
	if _, err := e.js.Publish(t.Context(), subject, raw); err != nil {
		t.Fatalf("publish %s: %v", subject, err)
	}
}

// subjectCounts returns how many messages a stream holds per subject.
func (e *env) subjectCounts(t *testing.T, stream string) map[string]uint64 {
	t.Helper()
	info, err := e.streams[stream].Info(t.Context(), jetstream.WithSubjectFilter(">"))
	if err != nil {
		t.Fatalf("stream info %s: %v", stream, err)
	}
	return info.State.Subjects
}

func (e *env) sendTotal(t *testing.T) uint64 {
	t.Helper()
	var n uint64
	for _, c := range e.subjectCounts(t, "SEND") {
		n += c
	}
	return n
}

// sendDeliveryIDs reads every message of the SEND stream: subject to the delivery ids it carries.
func (e *env) sendDeliveryIDs(t *testing.T) map[string][]string {
	t.Helper()
	info, err := e.streams["SEND"].Info(t.Context())
	if err != nil {
		t.Fatalf("stream info: %v", err)
	}
	out := map[string][]string{}
	for seq := info.State.FirstSeq; seq <= info.State.LastSeq && info.State.Msgs > 0; seq++ {
		msg, err := e.streams["SEND"].GetMsg(t.Context(), seq)
		if err != nil {
			continue // deleted sequence
		}
		var p intake.SendPayload
		if err := json.Unmarshal(msg.Data, &p); err != nil {
			t.Fatalf("unmarshal send payload %s: %v", msg.Data, err)
		}
		out[msg.Subject] = append(out[msg.Subject], p.DeliveryID)
	}
	return out
}

// idle waits until the three consumers have nothing pending and nothing waiting for a retry.
func (e *env) idle(t *testing.T) {
	t.Helper()
	eventually(t, 15*time.Second, "consumers to be idle", func() bool {
		for stream, name := range map[string]string{
			"EVENTS": intake.EventsConsumer, "RECIPIENTS": intake.RecipientsConsumer, "SCHEDULES": intake.SchedulesConsumer,
		} {
			c, err := e.js.Consumer(t.Context(), stream, name)
			if err != nil {
				return false
			}
			info, err := c.Info(t.Context())
			if err != nil || info.NumPending != 0 || info.NumAckPending != 0 {
				return false
			}
		}
		return true
	})
}

func (e *env) scalar(t *testing.T, query string, args ...any) string {
	t.Helper()
	var s *string
	err := e.pool.QueryRow(t.Context(), query, args...).Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		return "<no row>"
	}
	if err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	if s == nil {
		return "<null>"
	}
	return *s
}

func (e *env) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(t.Context(), query, args...).Scan(&n); err != nil {
		t.Fatalf("query %q: %v", query, err)
	}
	return n
}

// Fixtures.

const (
	istanbul = "Europe/Istanbul"
	london   = "Europe/London"
)

var (
	trSMS   = json.RawMessage(`{"text":"Merhaba {{first_name}}, sepetin seni bekliyor"}`)
	trEmail = json.RawMessage(`{"subject":"Sepetin seni bekliyor","blocks":[]}`)
	enSMS   = json.RawMessage(`{"text":"Hi {{first_name}}, your cart is waiting"}`)
)

func newCampaignID() string { return uuid.Must(uuid.NewV7()).String() }

// confirmedData builds a confirmed event with the given schedules and the Turkish sms and email
// messages plus the English sms message.
func confirmedData(campaignID string, batches int, schedules ...flow.ConfirmedSchedule) flow.ConfirmedData {
	return flow.ConfirmedData{
		CampaignID: campaignID,
		Schedules:  schedules,
		Messages: []flow.ConfirmedMessage{
			{Language: "tr", Channel: "sms", Content: trSMS},
			{Language: "tr", Channel: "email", Content: trEmail},
			{Language: "en", Channel: "sms", Content: enSMS},
		},
		CTAURL:           "https://shop.example.com/cart?utm_campaign=" + campaignID,
		RecipientBatches: batches,
		RecipientCounts:  flow.Recipients{},
	}
}

func immediate(language, tz string) flow.ConfirmedSchedule {
	return flow.ConfirmedSchedule{Language: language, Timezone: tz}
}

func at(language, tz string, t time.Time) flow.ConfirmedSchedule {
	return flow.ConfirmedSchedule{Language: language, Timezone: tz, SendAt: &t}
}

func recipient(id, language, tz, channel string) flow.BatchRecipient {
	contact := id + "@example.com"
	if channel == "sms" {
		contact = "+90555" + strings.Repeat("1", 7)
	}
	return flow.BatchRecipient{
		CustomerID: id, FirstName: "Ada " + id, Language: language, Timezone: tz, Channel: channel, Contact: contact,
	}
}

func batch(campaignID string, no, count int, rs ...flow.BatchRecipient) flow.RecipientBatch {
	return flow.RecipientBatch{CampaignID: campaignID, BatchNo: no, BatchCount: count, Recipients: rs}
}
