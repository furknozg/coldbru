//go:build integration

package flow_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furknozg/coldbru/internal/customer/audience"
	"github.com/furknozg/coldbru/internal/customer/auth"
	"github.com/furknozg/coldbru/internal/customer/consent"
	"github.com/furknozg/coldbru/internal/customer/graph"
	"github.com/furknozg/coldbru/internal/customer/pgtest"
	"github.com/furknozg/coldbru/internal/platform/db"
	"github.com/furknozg/coldbru/internal/platform/migrate"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/flow/flowtest"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/llmtest"
	"github.com/furknozg/coldbru/internal/proposal/store"
)

const proposalToken = "flow-it-token"

var (
	pgOnce sync.Once
	pgEnv  *pgtest.Env
	pgErr  error

	dbSeqMu sync.Mutex
	dbSeq   int
)

// TestMain only closes the database; the first test that needs it starts it, so the unit tests of
// this package (same binary) do not pay for a container.
func TestMain(m *testing.M) {
	code := m.Run()
	if pgEnv != nil {
		pgEnv.Close()
	}
	os.Exit(code)
}

// env starts the seeded Postgres once: customer_db for the real Customer handler, and the same
// server hosts one proposal database per test.
func env(t *testing.T) *pgtest.Env {
	t.Helper()
	pgOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		pgEnv, pgErr = pgtest.Start(ctx)
	})
	if pgErr != nil {
		t.Fatalf("start seeded postgres: %v", pgErr)
	}
	return pgEnv
}

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// proposalPool creates a database with a proposal schema on the seeded server, migrates it like the
// service does at startup, and returns a pool whose search_path is "proposal,public" (D-048).
func proposalPool(t *testing.T) *pgxpool.Pool { return proposalPoolMax(t, 0) }

// proposalPoolMax is proposalPool with pool_max_conns set (0 keeps the default).
func proposalPoolMax(t *testing.T, maxConns int) *pgxpool.Pool {
	t.Helper()
	cc := env(t).Pool.Config().ConnConfig
	hostPort := net.JoinHostPort(cc.Host, fmt.Sprint(cc.Port))
	admin, err := db.Open(t.Context(), fmt.Sprintf("postgres://postgres:postgres@%s/postgres?sslmode=disable", hostPort))
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	defer admin.Close()
	dbSeqMu.Lock()
	dbSeq++
	name := fmt.Sprintf("flow_test_%d_%d", os.Getpid(), dbSeq)
	dbSeqMu.Unlock()
	if _, err := admin.Exec(t.Context(), "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	base := fmt.Sprintf("postgres://postgres:postgres@%s/%s?sslmode=disable", hostPort, name)
	setup, err := db.Open(t.Context(), base)
	if err != nil {
		t.Fatalf("open setup pool: %v", err)
	}
	if _, err := setup.Exec(t.Context(), "CREATE SCHEMA proposal"); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	setup.Close()
	u, _ := url.Parse(base)
	q := u.Query()
	q.Set("search_path", "proposal,public")
	if maxConns > 0 {
		q.Set("pool_max_conns", fmt.Sprint(maxConns))
	}
	u.RawQuery = q.Encode()
	pool, err := db.Open(t.Context(), u.String())
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := migrate.UpPlatform(t.Context(), pool, discard()); err != nil {
		t.Fatalf("UpPlatform: %v", err)
	}
	if err := migrate.Up(t.Context(), pool, store.Migrations(), discard()); err != nil {
		t.Fatalf("Up: %v", err)
	}
	return pool
}

// customerClient serves the real Customer GraphQL handler over the seeded database behind the real
// bearer-token middleware and returns Proposal's client for it (as internal/proposal/llm/anthropic's
// integration test does).
func customerClient(t *testing.T) *customer.Client {
	t.Helper()
	p := env(t).Pool
	cities, err := audience.LoadCities(t.Context(), p)
	if err != nil {
		t.Fatalf("LoadCities: %v", err)
	}
	h := graph.NewHandler(graph.NewResolver(audience.NewService(p, cities), consent.NewService(p), discard()))
	tokens, err := auth.NewTokenSet(map[string]string{
		auth.CallerProposal: proposalToken, auth.CallerNotification: "notification-it-token", auth.CallerAdmin: "admin-it-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(tokens.Middleware(h))
	t.Cleanup(srv.Close)
	c, err := customer.New(customer.Config{URL: srv.URL, Token: proposalToken})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// The sample brief of the case and its audience (D-010, D-062): 46 reachable, coverage 20/13/13/3.
const (
	sampleBrief = "Son 30 günde sepetine ürün ekleyip alışveriş yapmayan İstanbul'daki kullanıcılara %30 indirim hatırlatması gönder. Samimi ama kısa olsun."
	sampleDSL   = `city === "İstanbul" && cartItems.some(c => c.addedAt >= ago("30d")) && orders.none(o => o.orderDate >= ago("30d"))`
)

// fakeClock is a settable clock.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// hookLLM wraps an llm.LLM so a test can hold a call open.
type hookLLM struct {
	llm.LLM
	mu            sync.Mutex
	beforeSegment func()
	beforeScreen  func()
}

func (h *hookLLM) Screen(ctx context.Context, brief string) (llm.ScreenResult, error) {
	h.mu.Lock()
	f := h.beforeScreen
	h.mu.Unlock()
	if f != nil {
		f()
	}
	return h.LLM.Screen(ctx, brief)
}

func (h *hookLLM) ProposeSegment(ctx context.Context, in llm.SegmentInput, tools llm.SegmentTools) (llm.SegmentResult, error) {
	h.mu.Lock()
	f := h.beforeSegment
	h.mu.Unlock()
	if f != nil {
		f()
	}
	return h.LLM.ProposeSegment(ctx, in, tools)
}

func (h *hookLLM) set(screen, segment func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.beforeScreen, h.beforeSegment = screen, segment
}

func (h *hookLLM) setScreen(screen func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.beforeScreen = screen
}

func (h *hookLLM) setSegment(segment func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.beforeSegment = segment
}

// harness is a Service over a fresh proposal database with scripted doubles.
type harness struct {
	t         *testing.T
	pool      *pgxpool.Pool
	svc       *flow.Service
	llm       *llmtest.Scripted
	hooks     *hookLLM
	content   *flowtest.Content
	confirmer *flowtest.Confirmer
	clock     *fakeClock
	tools     llm.SegmentTools
	q         *store.Queries
}

func newHarness(t *testing.T) *harness { return newHarnessWith(t, 0, nil) }

// newHarnessWith builds a harness over a pool of maxConns connections (0: the default) and lets
// mutate change the service dependencies.
func newHarnessWith(t *testing.T, maxConns int, mutate func(*flow.Deps)) *harness {
	t.Helper()
	pool := proposalPoolMax(t, maxConns)
	h := &harness{
		t: t, pool: pool, q: store.New(pool),
		llm: llmtest.NewScripted(), content: &flowtest.Content{}, confirmer: &flowtest.Confirmer{},
		clock: &fakeClock{now: time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)},
		tools: llm.NewCustomerTools(customerClient(t)),
	}
	h.hooks = &hookLLM{LLM: h.llm}
	deps := flow.Deps{
		Pool: pool, LLM: h.hooks, Tools: h.tools, Content: h.content, Confirmer: h.confirmer,
		Clock: h.clock, Hub: flow.NewHub(1024), Logger: discard(),
	}
	if mutate != nil {
		mutate(&deps)
	}
	svc, err := flow.New(deps)
	if err != nil {
		t.Fatal(err)
	}
	h.svc = svc
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := svc.Close(ctx); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return h
}

func (h *harness) idle() {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := h.svc.WaitIdle(ctx); err != nil {
		h.t.Fatalf("generation did not finish: %v", err)
	}
}

func (h *harness) get(c uuid.UUID) *flow.CampaignView {
	h.t.Helper()
	v, err := h.svc.Get(h.t.Context(), c)
	if err != nil {
		h.t.Fatalf("Get: %v", err)
	}
	return v
}

// step returns the current step of a kind, failing the test when there is none.
func (h *harness) step(c uuid.UUID, kind string) flow.StepView {
	h.t.Helper()
	for _, s := range h.get(c).Steps {
		if s.Kind == kind {
			return s
		}
	}
	h.t.Fatalf("campaign has no current %s step", kind)
	return flow.StepView{}
}

func (h *harness) mustApprove(v flow.VariantView) {
	h.t.Helper()
	if err := h.svc.Approve(h.t.Context(), v.ID); err != nil {
		h.t.Fatalf("Approve(%s %s/%s): %v", v.Kind, v.Language, v.Channel, err)
	}
}

// approveAll approves every draft variant of the step of kind and waits for the next generation.
func (h *harness) approveAll(c uuid.UUID, kind string) {
	h.t.Helper()
	for _, v := range h.step(c, kind).Variants {
		if v.Status == flow.VariantStatusDraft {
			h.mustApprove(v)
		}
	}
	h.idle()
}

// preview asks the real Customer service for the audience of src.
func (h *harness) preview(src string) *customer.Preview {
	h.t.Helper()
	p, err := h.tools.CountAudience(h.t.Context(), src)
	if err != nil {
		h.t.Fatalf("CountAudience(%s): %v", src, err)
	}
	return p
}

// scriptHappyPath queues the screen, segment and plan answers of the sample brief and returns
// the audience preview.
func (h *harness) scriptAudienceAndPlan(mode llm.PlanMode, primary llm.Channel) *customer.Preview {
	h.llm.AllowScreen().WithRecords(llm.PurposeScreen, rec(llm.PurposeScreen, llm.OutcomeAccepted, 1, ""))
	h.llm.ProposeDSLs(sampleDSL).WithRecords(llm.PurposeSegment, rec(llm.PurposeSegment, llm.OutcomeAccepted, 1, ""))
	h.llm.ProposePlans(llm.ChannelPlan{Mode: mode, Primary: primary, Rationale: "scripted"})
	return h.preview(sampleDSL)
}

// rec is a well-formed llm_call record for a scripted step.
func rec(purpose llm.Purpose, outcome llm.Outcome, attempt int, errText string) llm.CallRecord {
	r := llm.CallRecord{
		Purpose: purpose, Model: "scripted", Attempt: attempt, StopReason: "end_turn", LatencyMs: 1,
		Prompt:  llm.PromptRef{Name: string(purpose), Version: "0.0.0", SHA256: strings.Repeat("a", 64)},
		Outcome: outcome, Error: errText,
	}
	if outcome == llm.OutcomeAccepted {
		r.Response = json.RawMessage(`{"ok":true}`)
	}
	if outcome == llm.OutcomeAPIError && errText == "" {
		r.Error = "boom"
	}
	return r
}

// scheduleFor returns a valid schedule answer for the languages: the first at now + 24h in
// Istanbul time, the others immediately.
func (h *harness) scheduleFor(langs ...string) []llm.ScheduleEntry {
	at := h.clock.Now().Add(24 * time.Hour).In(time.FixedZone("+03", 3*3600))
	var out []llm.ScheduleEntry
	for i, l := range langs {
		e := llm.ScheduleEntry{Language: l, Rationale: "scripted"}
		if i == 0 {
			e.SendAt = &at
		}
		out = append(out, e)
	}
	return out
}

// toSlot decodes the language/channel of a variant view.
func slotKey(v flow.VariantView) string { return v.Language + "/" + v.Channel }

func decodeInto(t *testing.T, raw json.RawMessage, out any) {
	t.Helper()
	if err := json.Unmarshal(raw, out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
}

// startCampaign creates the sample campaign with a screen that waits for the returned release
// function, so a subscription made right after CreateCampaign sees every event.
func (h *harness) startCampaign(brief string) (flow.CampaignInfo, *flow.Subscription) {
	h.t.Helper()
	return h.startCampaignHooked(brief, nil)
}

// startCampaignHooked is startCampaign with a hook that runs before the first segment call.
func (h *harness) startCampaignHooked(brief string, beforeSegment func()) (flow.CampaignInfo, *flow.Subscription) {
	h.t.Helper()
	gate := make(chan struct{})
	h.hooks.set(func() { <-gate }, beforeSegment)
	c, err := h.svc.CreateCampaign(h.t.Context(), brief)
	if err != nil {
		h.t.Fatalf("CreateCampaign: %v", err)
	}
	sub := h.svc.Hub().Subscribe(c.ID)
	h.t.Cleanup(sub.Close)
	h.hooks.setScreen(nil)
	close(gate)
	return c, sub
}

// toContent runs the sample campaign up to the content step with its drafts generated: the audience
// and the plan are approved.
func (h *harness) toContent(mode llm.PlanMode, primary llm.Channel) (flow.CampaignInfo, *flow.Subscription, *customer.Preview) {
	h.t.Helper()
	prev := h.scriptAudienceAndPlan(mode, primary)
	camp, sub := h.startCampaign(sampleBrief)
	h.idle()
	h.approveAll(camp.ID, flow.KindAudience)
	h.approveAll(camp.ID, flow.KindChannelPlan)
	return camp, sub, prev
}

// events drains what a subscription holds.
func events(sub *flow.Subscription) []flow.Event {
	var out []flow.Event
	for {
		select {
		case e := <-sub.C:
			out = append(out, e)
		default:
			return out
		}
	}
}

func typesOf(evs []flow.Event) []string {
	out := make([]string, len(evs))
	for i, e := range evs {
		out[i] = e.Type()
	}
	return out
}

func countType(evs []flow.Event, typ string) int {
	n := 0
	for _, e := range evs {
		if e.Type() == typ {
			n++
		}
	}
	return n
}
