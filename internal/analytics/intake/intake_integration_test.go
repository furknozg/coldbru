//go:build integration

package intake_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/testcontainers/testcontainers-go"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"

	"github.com/furknozg/coldbru/internal/analytics/api"
	"github.com/furknozg/coldbru/internal/analytics/atest"
	"github.com/furknozg/coldbru/internal/analytics/intake"
	"github.com/furknozg/coldbru/internal/analytics/pricing"
	"github.com/furknozg/coldbru/internal/platform/consumer"
	"github.com/furknozg/coldbru/internal/platform/events"
)

var (
	pg      *atest.Postgres
	natsURL string
)

func TestMain(m *testing.M) { os.Exit(run(m)) }

func run(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	var err error
	if pg, err = atest.Start(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer pg.Close()
	ns, err := tcnats.Run(ctx, "nats:2.15-alpine")
	if err != nil {
		fmt.Fprintf(os.Stderr, "start nats container: %v\n", err)
		return 1
	}
	defer func() { _ = testcontainers.TerminateContainer(ns) }()
	if natsURL, err = ns.ConnectionString(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "nats connection string: %v\n", err)
		return 1
	}
	return m.Run()
}

// logCapture keeps the log records for assertions.
type logCapture struct {
	mu      sync.Mutex
	records []capturedRecord
}

type capturedRecord struct {
	level slog.Level
	msg   string
	attrs map[string]any
}

func (l *logCapture) Enabled(context.Context, slog.Level) bool { return true }
func (l *logCapture) WithGroup(string) slog.Handler            { return l }
func (l *logCapture) WithAttrs([]slog.Attr) slog.Handler       { return l }
func (l *logCapture) Handle(_ context.Context, r slog.Record) error {
	rec := capturedRecord{level: r.Level, msg: r.Message, attrs: map[string]any{}}
	r.Attrs(func(a slog.Attr) bool { rec.attrs[a.Key] = a.Value.Any(); return true })
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, rec)
	return nil
}

func (l *logCapture) find(msg string) []capturedRecord {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []capturedRecord
	for _, r := range l.records {
		if r.msg == msg {
			out = append(out, r)
		}
	}
	return out
}

var fastBackoff = consumer.Backoff{Base: 20 * time.Millisecond, Factor: 1.5, Max: 150 * time.Millisecond}

// harness is one test's database, EVENTS stream and, once started, the consumer.
type harness struct {
	t      *testing.T
	pool   *pgxpool.Pool
	js     jetstream.JetStream
	stream jetstream.Stream
	logs   *logCapture
	stop   func(context.Context) error
}

// newHarness creates the database and the real EVENTS stream; the consumer is not started yet.
func newHarness(t *testing.T) *harness {
	t.Helper()
	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "infra", "nats", "streams", "EVENTS.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cfg jetstream.StreamConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	st, err := js.CreateStream(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = js.DeleteStream(context.WithoutCancel(t.Context()), cfg.Name) })
	return &harness{t: t, pool: pg.NewMigratedPool(t), js: js, stream: st, logs: &logCapture{}}
}

func (h *harness) start(prices pricing.Table) {
	h.t.Helper()
	stop, err := intake.Start(h.t.Context(), intake.Deps{
		JS: h.js, Pool: h.pool, Logger: slog.New(h.logs), Prices: prices, Backoff: fastBackoff,
	})
	if err != nil {
		h.t.Fatalf("intake.Start() error = %v", err)
	}
	h.stop = stop
	h.t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(h.t.Context()), 10*time.Second)
		defer cancel()
		if err := stop(ctx); err != nil {
			h.t.Errorf("stop consumer: %v", err)
		}
	})
}

// publish sends an envelope the way a relay does. withMsgID false skips the stream's duplicate
// window, so a repeat is stored again and the consumer sees the same event id twice.
func (h *harness) publish(typ string, version int, producer string, data any, id string, withMsgID bool) string {
	h.t.Helper()
	env, err := events.New(typ, version, producer, "corr", data)
	if err != nil {
		h.t.Fatal(err)
	}
	return h.publishEnvelope(env, id, withMsgID)
}

func (h *harness) publishEnvelope(env events.Envelope, id string, withMsgID bool) string {
	h.t.Helper()
	if id == "" {
		id = uuid.Must(uuid.NewV7()).String()
	}
	env.ID = id
	subject, err := events.Subject(env)
	if err != nil {
		h.t.Fatal(err)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		h.t.Fatal(err)
	}
	var opts []jetstream.PublishOpt
	if withMsgID {
		opts = append(opts, jetstream.WithMsgID(id))
	}
	if _, err := h.js.Publish(h.t.Context(), subject, raw, opts...); err != nil {
		h.t.Fatalf("publish %s: %v", subject, err)
	}
	return id
}

func (h *harness) completed(campaign uuid.UUID, at time.Time, cost string, counts ...intake.Count) string {
	return h.publish(intake.EventTypeCompleted, 1, "notification",
		intake.CompletedData{CampaignID: campaign.String(), CompletedAt: at, Counts: counts, SMSSegments: 6, EstimatedCostUSD: cost}, "", true)
}

func (h *harness) token(campaign, call uuid.UUID, mutate func(*intake.TokenCompletedData)) string {
	d := intake.TokenCompletedData{
		LLMCallID: call.String(), CampaignID: campaign.String(), Purpose: "content", Model: "claude-sonnet-5-5",
		PromptName: "content", PromptVersion: "1.2.0", Outcome: "accepted", LatencyMs: 700,
		Tokens:    intake.TokenCounts{Input: 1000, Output: 500, CacheRead: 20000, CacheCreation: 4000},
		CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	if mutate != nil {
		mutate(&d)
	}
	return h.publish(intake.EventTypeTokenCompleted, 1, "proposal", d, "", true)
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// idle waits until the consumer has nothing pending and nothing waiting for a retry.
func (h *harness) idle() {
	h.t.Helper()
	eventually(h.t, "the consumer to be idle", func() bool {
		c, err := h.js.Consumer(h.t.Context(), intake.Stream, intake.Durable)
		if err != nil {
			return false
		}
		info, err := c.Info(h.t.Context())
		return err == nil && info.NumPending == 0 && info.NumAckPending == 0 && info.Delivered.Stream > 0
	})
}

func (h *harness) count(sql string, args ...any) int {
	h.t.Helper()
	var n int
	if err := h.pool.QueryRow(h.t.Context(), sql, args...).Scan(&n); err != nil {
		h.t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func (h *harness) text(sql string, args ...any) string {
	h.t.Helper()
	var s string
	if err := h.pool.QueryRow(h.t.Context(), sql, args...).Scan(&s); err != nil {
		h.t.Fatalf("%s: %v", sql, err)
	}
	return s
}

func TestConsumerConfig(t *testing.T) {
	h := newHarness(t)
	h.start(pricing.Defaults())
	c, err := h.js.Consumer(t.Context(), intake.Stream, intake.Durable)
	if err != nil {
		t.Fatal(err)
	}
	info, err := c.Info(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	cfg := info.Config
	if cfg.Durable != "analytics-events" || cfg.FilterSubject != "evt.>" || cfg.AckPolicy != jetstream.AckExplicitPolicy ||
		cfg.DeliverPolicy != jetstream.DeliverAllPolicy || cfg.MaxDeliver != consumer.MaxDeliver {
		t.Errorf("consumer config = %+v", cfg)
	}
}

func TestCompletedStoresSummaryAndCounts(t *testing.T) {
	h := newHarness(t)
	h.start(pricing.Defaults())
	id := uuid.New()
	at := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	h.completed(id, at, "0.2420",
		intake.Count{Channel: "sms", Status: "sent", Count: 3}, intake.Count{Channel: "email", Status: "sent", Count: 5},
		intake.Count{Channel: "email", Status: "skipped_no_consent", Count: 2})
	h.idle()

	if got := h.text(`SELECT delivery_cost_usd::text FROM campaign_summary WHERE campaign_id = $1`, id); got != "0.242000" {
		t.Errorf("delivery_cost_usd = %s", got)
	}
	if got := h.count(`SELECT sms_segments FROM campaign_summary WHERE campaign_id = $1 AND completed_at = $2`, id, at); got != 6 {
		t.Errorf("sms_segments = %d (or completed_at differs)", got)
	}
	if got := h.text(`SELECT string_agg(channel || '/' || status || '=' || count, ',' ORDER BY channel, status) FROM campaign_delivery_count WHERE campaign_id = $1`, id); got != "email/sent=5,email/skipped_no_consent=2,sms/sent=3" {
		t.Errorf("counts = %s", got)
	}

	// A later completed event for the same campaign replaces the summary and the counts.
	h.completed(id, at.Add(time.Hour), "0.5000", intake.Count{Channel: "sms", Status: "failed", Count: 9})
	eventually(t, "the replacement", func() bool {
		return h.text(`SELECT delivery_cost_usd::text FROM campaign_summary WHERE campaign_id = $1`, id) == "0.500000"
	})
	h.idle()
	if got := h.text(`SELECT string_agg(channel || '/' || status || '=' || count, ',') FROM campaign_delivery_count WHERE campaign_id = $1`, id); got != "sms/failed=9" {
		t.Errorf("counts after the replacement = %s, want only the new ones", got)
	}
	if got := h.count(`SELECT count(*) FROM campaign_summary`); got != 1 {
		t.Errorf("%d summaries", got)
	}
}

func TestRedeliveredEventIsANoOp(t *testing.T) {
	h := newHarness(t)
	h.start(pricing.Defaults())
	campaign, call := uuid.New(), uuid.New()
	// The same event id published twice, bypassing the stream's own deduplication: the consumer's
	// processed_event row must make the second a no-op.
	d := intake.TokenCompletedData{
		LLMCallID: call.String(), CampaignID: campaign.String(), Purpose: "screen", Model: "claude-haiku-5-5", PromptName: "screen",
		PromptVersion: "1.0.0", Outcome: "accepted", LatencyMs: 100, Tokens: intake.TokenCounts{Input: 10}, CreatedAt: time.Now().UTC(),
	}
	id := h.publish(intake.EventTypeTokenCompleted, 1, "proposal", d, "", false)
	h.publish(intake.EventTypeTokenCompleted, 1, "proposal", d, id, false)
	eventually(t, "the duplicate to be skipped", func() bool { return len(h.logs.find("duplicate event skipped")) == 1 })
	h.idle()
	if got := h.count(`SELECT count(*) FROM llm_usage`); got != 1 {
		t.Errorf("%d llm_usage rows, want 1", got)
	}
	if got := h.count(`SELECT count(*) FROM processed_event`); got != 1 {
		t.Errorf("%d processed_event rows, want 1", got)
	}
}

func TestTokenCompletedStoresPricedUsage(t *testing.T) {
	h := newHarness(t)
	h.start(pricing.Defaults())
	campaign, call := uuid.New(), uuid.New()
	h.token(campaign, call, nil)
	h.idle()

	var (
		purpose, model, name, version, outcome, cost string
		in, out, cr, cc, latency                     int
		created                                      time.Time
	)
	err := h.pool.QueryRow(t.Context(), `SELECT purpose, model, prompt_name, prompt_version, outcome, cost_usd::text,
		input_tokens, output_tokens, cache_read_tokens, cache_creation_tokens, latency_ms, created_at FROM llm_usage WHERE llm_call_id = $1`, call).
		Scan(&purpose, &model, &name, &version, &outcome, &cost, &in, &out, &cr, &cc, &latency, &created)
	if err != nil {
		t.Fatal(err)
	}
	// 1000*2 + 500*10 + 20000*0.20 + 4000*2.5 = 21000 USD-micro per MTok scale -> 0.021
	if purpose != "content" || model != "claude-sonnet-5-5" || name != "content" || version != "1.2.0" || outcome != "accepted" ||
		cost != "0.02100000" || in != 1000 || out != 500 || cr != 20000 || cc != 4000 || latency != 700 {
		t.Errorf("row = %s %s %s %s %s cost %s tokens %d/%d/%d/%d latency %d", purpose, model, name, version, outcome, cost, in, out, cr, cc, latency)
	}
	if created.IsZero() {
		t.Error("created_at not stored")
	}

	// The same llm_call_id in another event (a producer replay): the first row stands.
	h.token(campaign, call, func(d *intake.TokenCompletedData) { d.Tokens.Input = 5_000_000 })
	other := uuid.New()
	h.token(campaign, other, func(d *intake.TokenCompletedData) { d.Model = "mock"; d.Purpose = "segment" })
	eventually(t, "the second call to be stored", func() bool { return h.count(`SELECT count(*) FROM llm_usage`) == 2 })
	h.idle()
	if got := h.text(`SELECT cost_usd::text FROM llm_usage WHERE llm_call_id = $1`, call); got != "0.02100000" {
		t.Errorf("cost after a repeated llm_call_id = %s, want the first", got)
	}
	if got := h.text(`SELECT cost_usd::text FROM llm_usage WHERE llm_call_id = $1`, other); got != "0.00000000" {
		t.Errorf("mock cost = %s, want 0", got)
	}
	if got := len(h.logs.find("llm usage already stored, event ignored")); got != 1 {
		t.Errorf("%d 'already stored' records, want 1", got)
	}
}

func TestUnknownModelCostsZeroAndWarns(t *testing.T) {
	h := newHarness(t)
	h.start(pricing.Defaults())
	call := uuid.New()
	h.token(uuid.New(), call, func(d *intake.TokenCompletedData) { d.Model = "claude-future-9" })
	h.idle()
	if got := h.text(`SELECT cost_usd::text FROM llm_usage WHERE llm_call_id = $1`, call); got != "0.00000000" {
		t.Errorf("cost = %s, want 0", got)
	}
	warns := h.logs.find("no price for the model, cost recorded as 0")
	if len(warns) != 1 || warns[0].level != slog.LevelWarn || warns[0].attrs["model"] != "claude-future-9" {
		t.Errorf("warnings = %+v", warns)
	}
}

func TestOtherEventsAreAckedAndIgnored(t *testing.T) {
	h := newHarness(t)
	h.start(pricing.Defaults())
	campaign := uuid.New()
	h.publish("proposal.confirmed", 1, "proposal", map[string]any{"campaign_id": campaign.String()}, "", true)
	h.publish("proposal.cancelled", 1, "proposal", map[string]any{"campaign_id": campaign.String()}, "", true)
	h.publish("notification.started", 1, "notification", map[string]any{"campaign_id": campaign.String()}, "", true)
	h.publish(intake.EventTypeCompleted, 2, "notification", map[string]any{"campaign_id": "not even checked"}, "", true) // a future version
	h.idle()
	for _, table := range []string{"campaign_summary", "campaign_delivery_count", "llm_usage"} {
		if got := h.count(`SELECT count(*) FROM ` + table); got != 0 {
			t.Errorf("%s has %d rows", table, got)
		}
	}
	c, err := h.js.Consumer(t.Context(), intake.Stream, intake.Durable)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := c.Info(t.Context())
	if info.Delivered.Stream != 4 || info.NumAckPending != 0 || info.NumRedelivered != 0 || info.AckFloor.Stream != 4 {
		t.Errorf("consumer info = delivered %d, ack floor %d, ack pending %d, redelivered %d; want 4 acked once",
			info.Delivered.Stream, info.AckFloor.Stream, info.NumAckPending, info.NumRedelivered)
	}
	if n := len(h.logs.find("malformed event terminated")); n != 0 {
		t.Errorf("%d events terminated as malformed", n)
	}
}

func TestMalformedEventsAreTerminatedNotRetried(t *testing.T) {
	h := newHarness(t)
	h.start(pricing.Defaults())
	h.token(uuid.New(), uuid.New(), func(d *intake.TokenCompletedData) { d.Outcome = "timeout" })
	h.publish(intake.EventTypeCompleted, 1, "notification", map[string]any{"campaign_id": "x"}, "", true)
	eventually(t, "both events to be terminated", func() bool { return len(h.logs.find("malformed event terminated")) == 2 })
	h.idle()
	if got := h.count(`SELECT count(*) FROM llm_usage`) + h.count(`SELECT count(*) FROM campaign_summary`); got != 0 {
		t.Errorf("%d rows stored from malformed events", got)
	}
	if got := h.count(`SELECT count(*) FROM processed_event`); got != 0 {
		t.Errorf("%d processed_event rows: a terminated event must be rolled back", got)
	}
}

// A campaign that completed before Analytics existed is still on the EVENTS stream (7 days): the
// durable consumer starts from the first message.
func TestConsumerStartsFromTheBeginningOfTheStream(t *testing.T) {
	h := newHarness(t)
	id, call := uuid.New(), uuid.New()
	h.token(id, call, nil)
	h.completed(id, time.Now().UTC(), "1.5000", intake.Count{Channel: "sms", Status: "sent", Count: 4})
	h.start(pricing.Defaults())
	eventually(t, "the old events to be processed", func() bool {
		return h.count(`SELECT count(*) FROM campaign_summary`) == 1 && h.count(`SELECT count(*) FROM llm_usage`) == 1
	})
}

// The whole path to the report API: events in, the three endpoints out, over real HTTP.
func TestReportsFromEvents(t *testing.T) {
	h := newHarness(t)
	h.start(pricing.Defaults())
	srvMux := chi.NewRouter()
	api.New(api.Config{Source: api.NewPoolSource(h.pool), Logger: atest.Discard()}).Mount(srvMux)
	srv := httptest.NewServer(srvMux)
	t.Cleanup(srv.Close)
	getJSON := func(path string, into any) int {
		t.Helper()
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if into != nil && resp.StatusCode == 200 {
			if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
				t.Fatal(err)
			}
		}
		return resp.StatusCode
	}

	done, pending := uuid.New(), uuid.New()
	h.token(done, uuid.New(), nil)
	h.token(done, uuid.New(), func(d *intake.TokenCompletedData) {
		d.Purpose, d.PromptName, d.PromptVersion, d.Outcome = "segment", "segment", "1.0.0", "validation_failed"
		d.Model, d.Tokens = "claude-haiku-5-5", intake.TokenCounts{Input: 1_000_000}
	})
	h.token(pending, uuid.New(), func(d *intake.TokenCompletedData) { d.Model = "mock" })
	h.completed(done, time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC), "0.2420",
		intake.Count{Channel: "sms", Status: "sent", Count: 3}, intake.Count{Channel: "email", Status: "failed", Count: 1})
	eventually(t, "all events", func() bool {
		return h.count(`SELECT count(*) FROM llm_usage`) == 3 && h.count(`SELECT count(*) FROM campaign_summary`) == 1
	})

	var rep api.CampaignReport
	if code := getJSON("/campaigns/"+done.String()+"/summary", &rep); code != 200 {
		t.Fatalf("summary status %d", code)
	}
	if rep.CompletedAt == nil || !rep.CompletedAt.Equal(time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)) || rep.SMSSegments != 6 || rep.DeliveryCostUSD != "0.242000" ||
		len(rep.Deliveries) != 2 || rep.Deliveries[0] != (api.DeliveryCount{Channel: "email", Status: "failed", Count: 1}) ||
		rep.Deliveries[1] != (api.DeliveryCount{Channel: "sms", Status: "sent", Count: 3}) {
		t.Errorf("summary = %+v", rep)
	}
	// 0.021 (sonnet call) + 0.1 (1M haiku input tokens)
	if rep.LLM.Calls != 2 || rep.LLM.CostUSD != "0.12100000" ||
		rep.LLM.Tokens != (api.Tokens{Input: 1_001_000, Output: 500, CacheRead: 20000, CacheCreation: 4000}) || len(rep.LLM.ByPurpose) != 2 ||
		rep.LLM.ByPurpose[0].Purpose != "content" || rep.LLM.ByPurpose[0].CostUSD != "0.02100000" ||
		rep.LLM.ByPurpose[1].Purpose != "segment" || rep.LLM.ByPurpose[1].CostUSD != "0.10000000" || rep.LLM.ByPurpose[1].InputTokens != 1_000_000 {
		t.Errorf("llm = %+v", rep.LLM)
	}

	var notYet api.CampaignReport
	if code := getJSON("/campaigns/"+pending.String()+"/summary", &notYet); code != 200 || notYet.CompletedAt != nil || len(notYet.Deliveries) != 0 ||
		notYet.LLM.Calls != 1 || notYet.SMSSegments != 0 {
		t.Errorf("campaign with usage only: status %d, %+v", code, notYet)
	}
	if code := getJSON("/campaigns/"+uuid.NewString()+"/summary", nil); code != 404 {
		t.Errorf("unknown campaign status %d, want 404", code)
	}

	var list api.CampaignList
	if code := getJSON("/campaigns", &list); code != 200 || len(list.Items) != 1 || list.Items[0].CampaignID != done ||
		list.Items[0].LLMCostUSD != "0.12100000" || list.Items[0].DeliveryCostUSD != "0.242000" {
		t.Errorf("list: status %d, %+v", code, list)
	}

	var versions []api.PromptVersionUsage
	if code := getJSON("/llm-usage?group=prompt_version", &versions); code != 200 || len(versions) != 2 {
		t.Fatalf("llm-usage: status %d, %+v", code, versions)
	}
	byKey := map[string]api.PromptVersionUsage{}
	for _, v := range versions {
		byKey[v.PromptName+"@"+v.PromptVersion] = v
	}
	if v := byKey["segment@1.0.0"]; v.Calls != 1 || v.ValidationFailed != 1 || v.RepairRate != 1 || v.CostUSD != "0.10000000" || v.AvgLatencyMs != 700 {
		t.Errorf("segment@1.0.0 = %+v", v)
	}
	if v := byKey["content@1.2.0"]; v.Calls != 2 || v.Accepted != 2 || v.RepairRate != 0 || v.CostUSD != "0.02100000" {
		t.Errorf("content@1.2.0 = %+v", v)
	}
	if code := getJSON("/llm-usage?group=model", nil); code != 422 {
		t.Errorf("group=model status %d, want 422", code)
	}
}
