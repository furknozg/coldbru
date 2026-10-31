//go:build integration

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/testcontainers/testcontainers-go"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"

	aapi "github.com/furknozg/coldbru/internal/analytics/api"
	aintake "github.com/furknozg/coldbru/internal/analytics/intake"
	"github.com/furknozg/coldbru/internal/analytics/pricing"
	astore "github.com/furknozg/coldbru/internal/analytics/store"
	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/customer/audience"
	"github.com/furknozg/coldbru/internal/customer/auth"
	custconsent "github.com/furknozg/coldbru/internal/customer/consent"
	"github.com/furknozg/coldbru/internal/customer/graph"
	"github.com/furknozg/coldbru/internal/customer/pgtest"
	"github.com/furknozg/coldbru/internal/mockprovider"
	nconsent "github.com/furknozg/coldbru/internal/notification/consent"
	nintake "github.com/furknozg/coldbru/internal/notification/intake"
	"github.com/furknozg/coldbru/internal/notification/links"
	nprogress "github.com/furknozg/coldbru/internal/notification/progress"
	"github.com/furknozg/coldbru/internal/notification/provider"
	"github.com/furknozg/coldbru/internal/notification/sender"
	nstore "github.com/furknozg/coldbru/internal/notification/store"
	"github.com/furknozg/coldbru/internal/platform/consumer"
	"github.com/furknozg/coldbru/internal/platform/db"
	"github.com/furknozg/coldbru/internal/platform/migrate"
	"github.com/furknozg/coldbru/internal/platform/outbox"
	"github.com/furknozg/coldbru/internal/proposal/confirm"
	"github.com/furknozg/coldbru/internal/proposal/contentgen"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/llmtest"
	pprogress "github.com/furknozg/coldbru/internal/proposal/progress"
	"github.com/furknozg/coldbru/internal/proposal/prompts"
	pstore "github.com/furknozg/coldbru/internal/proposal/store"
	rapi "github.com/furknozg/coldbru/internal/redirect/api"
	"github.com/furknozg/coldbru/internal/redirect/code"
	rstore "github.com/furknozg/coldbru/internal/redirect/store"
)

// The sample brief of the case and its audience (D-010, D-062): 46 reachable, coverage 20/13/13/3.
const (
	sampleBrief = "Son 30 günde sepetine ürün ekleyip alışveriş yapmayan İstanbul'daki kullanıcılara %30 indirim hatırlatması gönder. Samimi ama kısa olsun."
	sampleDSL   = `city === "İstanbul" && cartItems.some(c => c.addedAt >= ago("30d")) && orders.none(o => o.orderDate >= ago("30d"))`

	proposalToken     = "e2e-proposal-token"
	notificationToken = "e2e-notification-token"
	adminToken        = "e2e-admin-token"

	// mockSeed pins the random choices of the mock provider (the order concurrent requests draw in
	// is still up to the scheduler).
	mockSeed = 20261031
)

var (
	customerEnv *pgtest.Env
	natsURL     string
)

func TestMain(m *testing.M) { os.Exit(run(m)) }

func run(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	var err error
	// customerEnv is the seeded Postgres: customer_db for the real Customer handler, and the same
	// server hosts a database per other service (D-025).
	if customerEnv, err = pgtest.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "start seeded postgres: %v\n", err)
		return 1
	}
	defer customerEnv.Close()
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

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// openDB creates a database on the seeded server, optionally with a schema the pool's search_path
// points at (D-048), and returns a pool on it.
func openDB(t *testing.T, name, schema string) *pgxpool.Pool {
	t.Helper()
	cc := customerEnv.Pool.Config().ConnConfig
	hostPort := net.JoinHostPort(cc.Host, fmt.Sprint(cc.Port))
	base := "postgres://postgres:postgres@" + hostPort
	admin, err := db.Open(t.Context(), base+"/postgres?sslmode=disable")
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	defer admin.Close()
	if _, err := admin.Exec(t.Context(), "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database %s: %v", name, err)
	}
	dsn := base + "/" + name + "?sslmode=disable"
	if schema != "" {
		setup, err := db.Open(t.Context(), dsn)
		if err != nil {
			t.Fatalf("open setup pool: %v", err)
		}
		if _, err := setup.Exec(t.Context(), "CREATE SCHEMA "+schema); err != nil {
			t.Fatalf("create schema %s: %v", schema, err)
		}
		setup.Close()
		dsn += "&search_path=" + schema + ",public"
	}
	pool, err := db.Open(t.Context(), dsn)
	if err != nil {
		t.Fatalf("open pool %s: %v", name, err)
	}
	t.Cleanup(pool.Close)
	if err := migrate.UpPlatform(t.Context(), pool, discard()); err != nil {
		t.Fatalf("UpPlatform(%s): %v", name, err)
	}
	return pool
}

// createStreams creates EVENTS, RECIPIENTS, SCHEDULES and SEND from the definitions the compose
// stack loads (D-049).
func createStreams(t *testing.T, js jetstream.JetStream) {
	t.Helper()
	dir := filepath.Join("..", "..", "infra", "nats", "streams")
	for _, name := range []string{"EVENTS", "RECIPIENTS", "SCHEDULES", "SEND"} {
		raw, err := os.ReadFile(filepath.Join(dir, name+".json"))
		if err != nil {
			t.Fatalf("read stream definition %s: %v", name, err)
		}
		var cfg jetstream.StreamConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			t.Fatalf("parse stream definition %s: %v", name, err)
		}
		if _, err := js.CreateStream(t.Context(), cfg); err != nil {
			t.Fatalf("create stream %s: %v", name, err)
		}
		t.Cleanup(func() { _ = js.DeleteStream(context.WithoutCancel(t.Context()), name) })
	}
}

// customerServer serves the real Customer GraphQL handler over the seeded customer_db behind the
// real bearer-token middleware; Proposal and Notification both call it.
func customerServer(t *testing.T) *httptest.Server {
	t.Helper()
	cities, err := audience.LoadCities(t.Context(), customerEnv.Pool)
	if err != nil {
		t.Fatalf("LoadCities: %v", err)
	}
	h := graph.NewHandler(graph.NewResolver(audience.NewService(customerEnv.Pool, cities), custconsent.NewService(customerEnv.Pool), discard()))
	tokens, err := auth.NewTokenSet(map[string]string{
		auth.CallerProposal: proposalToken, auth.CallerNotification: notificationToken, auth.CallerAdmin: adminToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(tokens.Middleware(h))
	t.Cleanup(srv.Close)
	return srv
}

// inbox is the Forwarder of the mock provider: the place where an accepted message "arrives", in
// memory instead of Mailpit and MockServer. It records every forward by Idempotency-Key.
type inbox struct {
	mu   sync.Mutex
	msgs []inboxMsg
}

type inboxMsg struct {
	Key, Channel, To string
	// Body is the SMS text, or the HTML of the email.
	Body string
}

func (b *inbox) SendEmail(_ context.Context, e mockprovider.Email) error {
	b.add(inboxMsg{Key: e.IdempotencyKey, Channel: "email", To: e.To, Body: e.HTML})
	return nil
}

func (b *inbox) SendSMS(_ context.Context, s mockprovider.SMS) error {
	b.add(inboxMsg{Key: s.IdempotencyKey, Channel: "sms", To: s.To, Body: s.Text})
	return nil
}

func (b *inbox) add(m inboxMsg) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.msgs = append(b.msgs, m)
}

func (b *inbox) all() []inboxMsg {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]inboxMsg(nil), b.msgs...)
}

// mockProvider serves the real mockprovider handler with low, seeded fault rates.
func mockProvider(t *testing.T, in *inbox) *httptest.Server {
	t.Helper()
	cfg := mockprovider.DefaultConfig()
	cfg.ErrorRate, cfg.Rate429, cfg.TimeoutRate = 0.03, 0.02, 0
	cfg.LatencyMin, cfg.LatencyMax = 0, 10*time.Millisecond
	cfg.Seed, cfg.HasSeed = mockSeed, true
	h, err := mockprovider.NewHandler(mockprovider.HandlerConfig{Config: cfg, Forwarder: in, Logger: discard()})
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	h.Mount(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

// scriptedLLM is llmtest.Scripted whose content answers are chosen by the slot of the call instead
// of by the order of the calls: the content generator runs one goroutine per slot, so the order is
// not fixed. Everything else is the scripted fake as it is.
type scriptedLLM struct {
	*llmtest.Scripted
	mu   sync.Mutex
	docs map[string]string // "<language>/<channel>" to the model's answer
}

func (s *scriptedLLM) StreamStructured(ctx context.Context, pkg *prompts.Package, userJSON []byte, onDelta func(string), validate func(json.RawMessage) error) (json.RawMessage, llm.Usage, error) {
	var in struct{ Language, Channel string }
	if err := json.Unmarshal(userJSON, &in); err != nil {
		return nil, llm.Usage{}, err
	}
	doc, ok := s.docs[in.Language+"/"+in.Channel]
	if !ok {
		return nil, llm.Usage{}, fmt.Errorf("e2e: no scripted content for %s/%s", in.Language, in.Channel)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Scripted.StreamJSON(doc)
	return s.Scripted.StreamStructured(ctx, pkg, userJSON, onDelta, validate)
}

func smsDoc(text string) string {
	b, _ := json.Marshal(map[string]any{"sms": map[string]any{"text": text}, "email": nil})
	return string(b)
}

func emailDoc(subject, preheader, headline, p1, p2, cta string) string {
	b, _ := json.Marshal(map[string]any{"sms": nil, "email": map[string]any{
		"subject": subject, "preheader": preheader, "headline": headline, "paragraphs": []string{p1, p2}, "cta_label": cta,
	}})
	return string(b)
}

func eventually(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s", timeout, what)
}

// TestSampleBriefDeliversEndToEnd takes the sample brief of the case through every service in one
// process: Proposal (scripted LLM, real Customer) to the confirmed event and the recipient batches,
// Notification (intake, send workers, completion checker) to the real mock provider, the redirect
// service for the tracking links and the clicks (D-106), and Analytics from the completed and
// click events (D-102, D-108).
func TestSampleBriefDeliversEndToEnd(t *testing.T) {
	ctx := t.Context()
	fast := consumer.Backoff{Base: 20 * time.Millisecond, Factor: 1.5, Max: 150 * time.Millisecond}

	// --- infrastructure: one database per service, NATS with the real streams, the Customer service.
	proposalPool := openDB(t, "e2e_proposal", "proposal")
	notifyPool := openDB(t, "e2e_notification", "")
	analyticsPool := openDB(t, "e2e_analytics", "analytics")
	redirectPool := openDB(t, "e2e_redirect", "redirect")
	for _, m := range []struct {
		pool *pgxpool.Pool
		set  fs.FS
	}{{proposalPool, pstore.Migrations()}, {notifyPool, nstore.Migrations()}, {analyticsPool, astore.Migrations()}, {redirectPool, rstore.Migrations()}} {
		if err := migrate.Up(ctx, m.pool, m.set, discard()); err != nil {
			t.Fatalf("migrate: %v", err)
		}
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("nats.Connect: %v", err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	createStreams(t, js)
	custSrv := customerServer(t)
	cust, err := customer.New(customer.Config{URL: custSrv.URL, Token: proposalToken})
	if err != nil {
		t.Fatal(err)
	}

	// --- the mock provider and Notification's client of it.
	in := &inbox{}
	provSrv := mockProvider(t, in)
	provClient, err := provider.New(provider.Config{BaseURL: provSrv.URL})
	if err != nil {
		t.Fatal(err)
	}
	consent, err := nconsent.New(nconsent.Config{URL: custSrv.URL, Token: notificationToken, Logger: discard()})
	if err != nil {
		t.Fatal(err)
	}

	// --- the redirect service (D-106): the real handler over its own database. The tracking links
	// point at it directly, so the test can click them.
	signer, err := code.NewSigner("e2e-redirect-signing-key")
	if err != nil {
		t.Fatal(err)
	}
	redirectHandler, err := rapi.New(rapi.Config{Pool: redirectPool, Signer: signer, FallbackURL: content.DefaultCTAURL, Logger: discard()})
	if err != nil {
		t.Fatal(err)
	}
	redirectRouter := chi.NewRouter()
	redirectHandler.Mount(redirectRouter)
	redirectSrv := httptest.NewServer(redirectRouter)
	t.Cleanup(redirectSrv.Close)
	linkClient, err := links.New(links.Config{URL: redirectSrv.URL})
	if err != nil {
		t.Fatal(err)
	}
	trackingBase := redirectSrv.URL
	smsBudget := content.SMSBudget(trackingBase)

	// --- Proposal: the flow with the scripted LLM, the real content generator and confirmer.
	prev, err := llm.NewCustomerTools(cust).CountAudience(ctx, sampleDSL)
	if err != nil {
		t.Fatalf("CountAudience: %v", err)
	}
	var entries []llm.ScheduleEntry
	for _, l := range prev.ByLocale {
		if l.Count > 0 { // every pair receives a message in a primary_fallback plan; all are sent at once
			entries = append(entries, llm.ScheduleEntry{Language: l.Language, Timezone: l.Timezone, Rationale: "scripted"})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Language < entries[j].Language || entries[i].Language == entries[j].Language && entries[i].Timezone < entries[j].Timezone
	})
	script := &scriptedLLM{Scripted: llmtest.NewScripted(), docs: map[string]string{
		"tr/sms": smsDoc("Merhaba {{first_name}}, sepetindeki ürünlerde %30 indirim seni bekliyor, alışverişe dön!"),
		"tr/email": emailDoc("Sepetin seni bekliyor, {{first_name}}", "%30 indirim sadece bu hafta", "Merhaba {{first_name}}!",
			"Sepetindeki ürünler seni bekliyor.", "%30 indirim fırsatını kaçırma.", "Sepete dön"),
		"en/sms": smsDoc("Hi {{first_name}}, your cart is waiting: 30% off today only!"),
		"en/email": emailDoc("Your cart is waiting, {{first_name}}", "30% off this week only", "Hi {{first_name}}!",
			"The items in your cart are waiting for you.", "Do not miss 30% off.", "Back to cart"),
	}}
	script.AllowScreen()
	script.ProposeDSLs(sampleDSL)
	script.ProposePlans(llm.ChannelPlan{Mode: llm.ModePrimaryFallback, Primary: llm.ChannelEmail, Rationale: "scripted"})
	script.ProposeSchedules(entries)

	hub := flow.NewHub(0)
	conf, err := confirm.New(confirm.Deps{Pool: proposalPool, Customer: cust, CTAURL: content.DefaultCTAURL, Hub: hub, Logger: discard()})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := flow.New(flow.Deps{
		Pool: proposalPool, LLM: script, Customer: cust,
		Content:   contentgen.New(script, content.Prices{SMSSegment: 40_000, Email: 1_000}, contentgen.WithSMSBudget(smsBudget), contentgen.WithSMSLink(content.PlaceholderLink(trackingBase))),
		Confirmer: conf, Hub: hub, Logger: discard(),
	})
	if err != nil {
		t.Fatal(err)
	}

	// --- start the workers of every service, as the cmd/ services do (relay first, so it stops last).
	proposalRelay := (&outbox.Relay{Pool: proposalPool, JS: js, Logger: discard(), PollInterval: 50 * time.Millisecond}).Start(ctx)
	stopProgress, err := pprogress.Start(ctx, pprogress.Deps{JS: js, Pool: proposalPool, Hub: hub, Logger: discard(), Backoff: fast})
	if err != nil {
		t.Fatalf("proposal progress: %v", err)
	}
	notifyRelay := (&outbox.Relay{Pool: notifyPool, JS: js, Logger: discard(), PollInterval: 50 * time.Millisecond}).Start(ctx)
	redirectRelay := (&outbox.Relay{Pool: redirectPool, JS: js, Logger: discard(), PollInterval: 50 * time.Millisecond}).Start(ctx)
	stopIntake, err := nintake.Start(ctx, nintake.Deps{JS: js, Pool: notifyPool, Logger: discard(), Links: linkClient, Backoff: fast})
	if err != nil {
		t.Fatalf("notification intake: %v", err)
	}
	checker, err := nprogress.New(nprogress.Config{Pool: notifyPool, Logger: discard(), Prices: content.Prices{SMSSegment: 40_000, Email: 1_000}})
	if err != nil {
		t.Fatal(err)
	}
	stopSend, err := sender.Start(ctx, sender.Deps{
		JS: js, Pool: notifyPool, Logger: discard(), Provider: provClient, Consent: consent,
		Registerer: prometheus.NewRegistry(), Backoff: fast, Nudge: checker.Nudge, TrackingBaseURL: trackingBase,
	})
	if err != nil {
		t.Fatalf("sender: %v", err)
	}
	stopChecker := checker.Start(ctx)
	stopAnalytics, err := aintake.Start(ctx, aintake.Deps{JS: js, Pool: analyticsPool, Logger: discard(), Prices: pricing.Defaults(), Backoff: fast})
	if err != nil {
		t.Fatalf("analytics intake: %v", err)
	}
	analyticsAPI := chi.NewRouter()
	aapi.New(aapi.Config{Source: aapi.NewPoolSource(analyticsPool), Logger: discard()}).Mount(analyticsAPI)
	analyticsSrv := httptest.NewServer(analyticsAPI)
	t.Cleanup(analyticsSrv.Close)

	t.Cleanup(func() {
		// In the order of the services' shutdown hooks: workers that write to an outbox first, the
		// relays last.
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
		defer cancel()
		for name, stop := range map[string]func(context.Context) error{
			"flow": svc.Close, "completion checker": stopChecker, "send workers": stopSend, "notification intake": stopIntake,
			"proposal progress": stopProgress, "analytics intake": stopAnalytics,
		} {
			if err := stop(cctx); err != nil {
				t.Errorf("stop %s: %v", name, err)
			}
		}
		if err := notifyRelay(cctx); err != nil {
			t.Errorf("stop notification relay: %v", err)
		}
		if err := redirectRelay(cctx); err != nil {
			t.Errorf("stop redirect relay: %v", err)
		}
		if err := proposalRelay(cctx); err != nil {
			t.Errorf("stop proposal relay: %v", err)
		}
	})

	idle := func() {
		t.Helper()
		wctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := svc.WaitIdle(wctx); err != nil {
			t.Fatalf("generation did not finish: %v", err)
		}
	}
	step := func(id uuid.UUID, kind string) flow.StepView {
		t.Helper()
		v, err := svc.Get(ctx, id)
		if err != nil {
			t.Fatalf("Get: %v", err)
		}
		for _, s := range v.Steps {
			if s.Kind == kind {
				return s
			}
		}
		t.Fatalf("campaign has no current %s step", kind)
		return flow.StepView{}
	}
	approveStep := func(id uuid.UUID, kind string) flow.StepView {
		t.Helper()
		idle()
		s := step(id, kind)
		if s.Failed || s.Generating || len(s.Missing) != 0 {
			t.Fatalf("%s step is not ready: %+v", kind, s)
		}
		for _, v := range s.Variants {
			if v.Status == flow.VariantStatusDraft {
				if err := svc.Approve(ctx, v.ID); err != nil {
					t.Fatalf("Approve(%s %s/%s): %v", v.Kind, v.Language, v.Channel, err)
				}
			}
		}
		return s
	}

	// --- 1. Proposal: brief to confirmed campaign.
	camp, err := svc.CreateCampaign(ctx, sampleBrief)
	if err != nil {
		t.Fatalf("CreateCampaign: %v", err)
	}
	audStep := approveStep(camp.ID, flow.KindAudience)
	var aud flow.AudienceContent
	if err := json.Unmarshal(audStep.Variants[0].Content, &aud); err != nil {
		t.Fatal(err)
	}
	if aud.Count != 46 || aud.Coverage != (flow.Coverage{Both: 20, EmailOnly: 13, SMSOnly: 13, Unreachable: 3}) {
		t.Fatalf("audience = %d with %+v, want 46 with 20/13/13/3", aud.Count, aud.Coverage)
	}
	planStep := approveStep(camp.ID, flow.KindChannelPlan)
	var plan flow.PlanContent
	if err := json.Unmarshal(planStep.Variants[0].Content, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Mode != llm.ModePrimaryFallback || plan.Primary != llm.ChannelEmail || plan.Recipients != (flow.Recipients{Email: 33, SMS: 13, Total: 46}) {
		t.Fatalf("plan = %s/%s with %+v, want primary_fallback/email with 33 email and 13 sms", plan.Mode, plan.Primary, plan.Recipients)
	}
	approveStep(camp.ID, flow.KindContent)
	approveStep(camp.ID, flow.KindSchedule) // the last approval confirms the campaign
	if v, err := svc.Get(ctx, camp.ID); err != nil || v.Status == flow.CampaignStatusDrafting {
		t.Fatalf("campaign after the last schedule approval = %+v (%v), want it confirmed", v, err)
	}

	// --- 2. Notification and the mock provider: wait until every delivery is final and the campaign
	// is completed, as Proposal sees it through Notification's events.
	eventually(t, 90*time.Second, "the campaign to complete", func() bool {
		v, err := svc.Get(ctx, camp.ID)
		return err == nil && v.Status == flow.CampaignStatusCompleted
	})

	// Every (customer, channel) has exactly one delivery and it is sent.
	rows, err := notifyPool.Query(ctx, `SELECT d.id::text, d.customer_id, d.channel, d.status, d.provider_message_id IS NOT NULL
		FROM delivery d WHERE d.campaign_id = $1`, camp.ID)
	if err != nil {
		t.Fatal(err)
	}
	type key struct{ customer, channel string }
	deliveries := map[key]string{} // (customer, channel) to delivery id
	byChannel := map[string]int{}
	customers := map[string]bool{}
	for rows.Next() {
		var id, customerID, channel, status string
		var hasMessageID bool
		if err := rows.Scan(&id, &customerID, &channel, &status, &hasMessageID); err != nil {
			t.Fatal(err)
		}
		k := key{customerID, channel}
		if _, dup := deliveries[k]; dup {
			t.Errorf("two deliveries for %v", k)
		}
		deliveries[k] = id
		byChannel[channel]++
		customers[customerID] = true
		if status != "sent" || !hasMessageID {
			t.Errorf("delivery %s (%v) = %s (provider message id %v), want sent with a provider message id", id, k, status, hasMessageID)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(deliveries) != 46 || byChannel["email"] != 33 || byChannel["sms"] != 13 || len(customers) != 46 {
		t.Fatalf("%d deliveries (%v) to %d customers, want 33 email + 13 sms to 46 customers", len(deliveries), byChannel, len(customers))
	}

	// The provider's inbox got exactly one accepted message per delivery, on the delivery's channel.
	want := map[string]string{} // delivery id to channel
	for k, id := range deliveries {
		want[id] = k.channel
	}
	got := map[string]int{}
	for _, m := range in.all() {
		got[m.Key]++
		if ch, ok := want[m.Key]; !ok || ch != m.Channel {
			t.Errorf("the inbox got a %s message for key %s, which is no delivery of that channel", m.Channel, m.Key)
		}
		if m.To == "" {
			t.Errorf("the inbox got a message for key %s with no recipient", m.Key)
		}
	}
	for id := range want {
		if got[id] != 1 {
			t.Errorf("the inbox got %d messages for delivery %s, want exactly 1", got[id], id)
		}
	}
	if n := len(in.all()); n != 46 {
		t.Errorf("the inbox holds %d messages, want 46", n)
	}

	// --- 3. Analytics: the completed event arrived and its totals are Notification's.
	var wantSegments int64
	if err := notifyPool.QueryRow(ctx, `SELECT coalesce(sum((m.content ->> 'segments')::int), 0)
		FROM delivery d JOIN message m ON m.id = d.message_id
		WHERE d.campaign_id = $1 AND d.channel = 'sms' AND d.status = 'sent'`, camp.ID).Scan(&wantSegments); err != nil {
		t.Fatal(err)
	}
	var report aapi.CampaignReport
	eventually(t, 30*time.Second, "Analytics to report the campaign as completed", func() bool {
		resp, err := http.Get(analyticsSrv.URL + "/campaigns/" + camp.ID.String() + "/summary")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		report = aapi.CampaignReport{}
		return resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&report) == nil && report.CompletedAt != nil
	})
	if report.CampaignID != camp.ID {
		t.Errorf("report is for %s, want %s", report.CampaignID, camp.ID)
	}
	wantCounts := []aapi.DeliveryCount{{Channel: "email", Status: "sent", Count: 33}, {Channel: "sms", Status: "sent", Count: 13}}
	if fmt.Sprint(report.Deliveries) != fmt.Sprint(wantCounts) {
		t.Errorf("Analytics deliveries = %+v, want %+v", report.Deliveries, wantCounts)
	}
	if wantSegments < 13 || report.SMSSegments != wantSegments {
		t.Errorf("Analytics sms_segments = %d, want %d (at least one per sms)", report.SMSSegments, wantSegments)
	}

	// --- 4. Tracking links (D-106, D-109): every delivery has a link in the redirect service, and
	// the message that went out carries it.
	linkRows, err := notifyPool.Query(ctx, `SELECT id::text, customer_id, channel, link_code FROM delivery WHERE campaign_id = $1`, camp.ID)
	if err != nil {
		t.Fatal(err)
	}
	codeOf := map[string]string{} // delivery id to link code
	var emailsToClick, smsToClick []string
	for linkRows.Next() {
		var id, customerID, channel string
		var linkCode *string
		if err := linkRows.Scan(&id, &customerID, &channel, &linkCode); err != nil {
			t.Fatal(err)
		}
		if linkCode == nil || !signer.Verify(*linkCode) {
			t.Errorf("delivery %s (%s) has link_code %v, want a code that verifies", id, channel, linkCode)
			continue
		}
		codeOf[id] = *linkCode
		if channel == "email" && len(emailsToClick) < 2 {
			emailsToClick = append(emailsToClick, id)
		}
		if channel == "sms" && len(smsToClick) < 1 {
			smsToClick = append(smsToClick, id)
		}
	}
	linkRows.Close()
	if len(codeOf) != 46 {
		t.Fatalf("%d deliveries have a link code, want all 46", len(codeOf))
	}
	var storedLinks, linksOfDeliveries int
	if err := redirectPool.QueryRow(ctx, `SELECT count(*), count(DISTINCT delivery_id) FROM link WHERE campaign_id = $1`, camp.ID).Scan(&storedLinks, &linksOfDeliveries); err != nil {
		t.Fatal(err)
	}
	if storedLinks != 46 || linksOfDeliveries != 46 {
		t.Errorf("the redirect service holds %d links for %d deliveries, want 46 and 46", storedLinks, linksOfDeliveries)
	}
	for _, m := range in.all() {
		link := content.TrackingLink(trackingBase, codeOf[m.Key])
		switch m.Channel {
		case "sms":
			if !strings.HasSuffix(m.Body, " "+link) || utf8.RuneCountInString(m.Body) > content.SMSMaxChars {
				t.Errorf("sms of %s = %q, want it to end with %q within 160 characters", m.Key, m.Body, link)
			}
		case "email":
			if !strings.Contains(m.Body, `href="`+link+`"`) {
				t.Errorf("email of %s has no button with %s", m.Key, link)
			}
		}
	}

	// --- 5. Clicks: two emails (one of them twice) and one SMS are clicked, and a code nobody issued.
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	click := func(c string) *http.Response {
		t.Helper()
		resp, err := noFollow.Get(redirectSrv.URL + "/r/" + c)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}
	wantTarget := content.DefaultCTAURL + "?utm_campaign=" + camp.ID.String()
	for _, id := range []string{emailsToClick[0], emailsToClick[0], emailsToClick[1], smsToClick[0]} {
		resp := click(codeOf[id])
		if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != wantTarget {
			t.Fatalf("click on %s = %d, Location %q; want 302 to %s", id, resp.StatusCode, resp.Header.Get("Location"), wantTarget)
		}
	}
	bad, _ := signer.New() // validly signed, but no link has it
	if resp := click(bad); resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != content.DefaultCTAURL {
		t.Errorf("click on an unknown code = %d, Location %q; want 302 to the default page", resp.StatusCode, resp.Header.Get("Location"))
	}

	eventually(t, 30*time.Second, "Analytics to count the clicks", func() bool {
		resp, err := http.Get(analyticsSrv.URL + "/campaigns/" + camp.ID.String() + "/summary")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		report = aapi.CampaignReport{}
		return resp.StatusCode == http.StatusOK && json.NewDecoder(resp.Body).Decode(&report) == nil && report.Clicks.TotalClicks == 4
	})
	c := report.Clicks
	if c.Sent != 46 || c.UniqueClicks != 3 || c.TotalClicks != 4 || c.CTR != 0.0652 {
		t.Errorf("campaign clicks = sent %d, unique %d, total %d, ctr %v; want 46, 3, 4, 0.0652 (3 of 46)", c.Sent, c.UniqueClicks, c.TotalClicks, c.CTR)
	}
	wantChannels := map[string]aapi.ChannelClicks{
		"email": {Channel: "email", Sent: 33, UniqueClicks: 2, TotalClicks: 3, CTR: 0.0606},
		"sms":   {Channel: "sms", Sent: 13, UniqueClicks: 1, TotalClicks: 1, CTR: 0.0769},
	}
	for _, ch := range c.ByChannel {
		if ch != wantChannels[ch.Channel] {
			t.Errorf("clicks of %s = %+v, want %+v", ch.Channel, ch, wantChannels[ch.Channel])
		}
	}
	var localeSent, localeUnique int64
	for _, l := range c.ByLocale {
		localeSent += l.Sent
		localeUnique += l.UniqueClicks
	}
	if len(c.ByChannel) != 2 || localeSent != 46 || localeUnique != 3 {
		t.Errorf("by channel %d entries, by locale sent %d and unique %d; want 2, 46 and 3", len(c.ByChannel), localeSent, localeUnique)
	}

	var overview aapi.Overview
	resp, err := http.Get(analyticsSrv.URL + "/overview")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&overview); err != nil || overview.Clicks.Sent != 46 || overview.Clicks.UniqueClicks != 3 || overview.Clicks.CTR != 0.0652 {
		t.Errorf("overview clicks = %+v (%v), want 3 of 46", overview.Clicks, err)
	}
}
