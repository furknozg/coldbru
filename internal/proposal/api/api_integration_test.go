//go:build integration

package api_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/customer/audience"
	"github.com/furknozg/coldbru/internal/customer/auth"
	"github.com/furknozg/coldbru/internal/customer/consent"
	"github.com/furknozg/coldbru/internal/customer/graph"
	"github.com/furknozg/coldbru/internal/customer/pgtest"
	"github.com/furknozg/coldbru/internal/platform/db"
	"github.com/furknozg/coldbru/internal/platform/migrate"
	"github.com/furknozg/coldbru/internal/proposal/api"
	"github.com/furknozg/coldbru/internal/proposal/contentgen"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/mock"
	"github.com/furknozg/coldbru/internal/proposal/store"
)

// The harness below is the one of internal/proposal/flow's integration tests (a test file cannot
// be imported): a seeded Postgres with the real Customer handler, and one proposal database per test.

const proposalToken = "api-it-token"

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

func proposalPool(t *testing.T) *pgxpool.Pool {
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
	name := fmt.Sprintf("api_test_%d_%d", os.Getpid(), dbSeq)
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

// ---- the system under test: real flow, real contentgen, mock LLM, real HTTP server ----

type system struct {
	t    *testing.T
	srv  *httptest.Server
	svc  *flow.Service
	cust *customer.Client
}

func newSystem(t *testing.T) *system {
	t.Helper()
	pool := proposalPool(t)
	cust := customerClient(t)
	model := mock.New(mock.WithChunkDelay(0))
	sms, _ := content.ParseMoney("0.04")
	email, _ := content.ParseMoney("0.001")
	svc, err := flow.New(flow.Deps{
		Pool: pool, LLM: model, Customer: cust, Content: contentgen.New(model, content.Prices{SMSSegment: sms, Email: email}), Logger: discard(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = svc.Close(ctx)
	})
	ctx, cancel := context.WithCancel(context.Background())
	r := chi.NewRouter()
	api.New(api.Config{Flow: svc, Logger: discard(), Shutdown: ctx}).Mount(r)
	srv := httptest.NewServer(r)
	t.Cleanup(func() { cancel(); srv.Close() })
	return &system{t: t, srv: srv, svc: svc, cust: cust}
}

// do sends a request and decodes a 2xx JSON answer into out (when not nil); it returns the status
// and the raw body.
func (s *system) do(method, path, body string, out any) (int, []byte) {
	s.t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(s.t.Context(), method, s.srv.URL+path, rd)
	if err != nil {
		s.t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if out != nil && resp.StatusCode/100 == 2 {
		if err := json.Unmarshal(raw, out); err != nil {
			s.t.Fatalf("%s %s: decode %s: %v", method, path, raw, err)
		}
	}
	return resp.StatusCode, raw
}

func (s *system) create(brief string) flow.CampaignInfo {
	s.t.Helper()
	b, _ := json.Marshal(map[string]string{"brief": brief})
	var info flow.CampaignInfo
	if code, raw := s.do("POST", "/campaigns", string(b), &info); code != http.StatusAccepted {
		s.t.Fatalf("POST /campaigns = %d: %s", code, raw)
	}
	return info
}

func (s *system) view(id uuid.UUID) flow.CampaignView {
	s.t.Helper()
	var v flow.CampaignView
	if code, raw := s.do("GET", "/campaigns/"+id.String(), "", &v); code != 200 {
		s.t.Fatalf("GET campaign = %d: %s", code, raw)
	}
	return v
}

func stepOf(v flow.CampaignView, kind string) *flow.StepView {
	for i := range v.Steps {
		if v.Steps[i].Kind == kind {
			return &v.Steps[i]
		}
	}
	return nil
}

func draftsOf(st *flow.StepView) []flow.VariantView {
	var out []flow.VariantView
	for _, v := range st.Variants {
		if v.Status == flow.VariantStatusDraft {
			out = append(out, v)
		}
	}
	return out
}

// waitDrafts polls the campaign until the open step of kind has n draft variants and returns them.
func (s *system) waitDrafts(id uuid.UUID, kind string, n int) []flow.VariantView {
	s.t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for {
		v := s.view(id)
		if st := stepOf(v, kind); st != nil && st.Status == flow.StepStatusOpen {
			if st.Failed {
				s.t.Fatalf("the %s step failed: %s", kind, st.Reason)
			}
			if d := draftsOf(st); len(d) >= n {
				return d
			}
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("no %d drafts of the %s step in time: %+v", n, kind, v.Steps)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (s *system) approve(variant uuid.UUID) flow.CampaignView {
	s.t.Helper()
	var v flow.CampaignView
	if code, raw := s.do("POST", "/variants/"+variant.String()+"/approve", "", &v); code != 200 {
		s.t.Fatalf("approve = %d: %s", code, raw)
	}
	return v
}

func asProblem(t *testing.T, code int, raw []byte, wantStatus int, wantCode string) api.Problem {
	t.Helper()
	var p api.Problem
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("decode problem %s: %v", raw, err)
	}
	if code != wantStatus || p.Code != wantCode || p.Status != wantStatus {
		t.Fatalf("status %d problem %+v, want %d %s", code, p, wantStatus, wantCode)
	}
	return p
}

// sampleBrief is the brief of the case study (D-010): 46 reachable people, coverage 20/13/13/3.
const sampleBriefText = "Son 30 günde sepetine ürün ekleyip alışveriş yapmayan İstanbul'daki kullanıcılara %30 indirim hatırlatması gönder. Samimi ama kısa olsun."

// collector keeps every frame of an event stream.
type collector struct {
	mu     sync.Mutex
	frames []frame
}

func (c *collector) run(s *stream) {
	go func() {
		for fr := range s.frames {
			c.mu.Lock()
			c.frames = append(c.frames, fr)
			c.mu.Unlock()
		}
	}()
}

func (c *collector) snapshot() []frame {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]frame(nil), c.frames...)
}

func TestSampleBriefOverRESTAndSSE(t *testing.T) {
	s := newSystem(t)
	info := s.create(sampleBriefText)
	if info.Status != flow.CampaignStatusDrafting || info.Brief != sampleBriefText {
		t.Fatalf("created = %+v", info)
	}

	// Read the stream until the audience variant arrives: either in an event or already in the snapshot.
	st := openStream(t, s.srv, info.ID)
	var coll collector
	var audienceVariant flow.VariantView
	deadline := time.After(90 * time.Second)
	for audienceVariant.ID == uuid.Nil {
		var fr frame
		select {
		case f, ok := <-st.frames:
			if !ok {
				t.Fatal("the stream ended before the audience arrived")
			}
			fr = f
		case <-deadline:
			t.Fatal("no audience variant on the stream in time")
		}
		coll.mu.Lock()
		coll.frames = append(coll.frames, fr)
		coll.mu.Unlock()
		switch fr.event {
		case api.EventSnapshot:
			var v flow.CampaignView
			if err := json.Unmarshal([]byte(fr.data), &v); err != nil {
				t.Fatal(err)
			}
			if a := stepOf(v, flow.KindAudience); a != nil {
				if d := draftsOf(a); len(d) > 0 {
					audienceVariant = d[0]
				}
			}
		case flow.TypeVariantCreated:
			var ev flow.VariantCreated
			if err := json.Unmarshal([]byte(fr.data), &ev); err != nil {
				t.Fatal(err)
			}
			if ev.Kind == flow.KindAudience {
				audienceVariant = flow.VariantView{ID: ev.VariantID, StepID: ev.StepID, Kind: ev.Kind, Content: ev.Content, Status: flow.VariantStatusDraft}
			}
		}
	}
	coll.run(st)

	var aud flow.AudienceContent
	if err := json.Unmarshal(audienceVariant.Content, &aud); err != nil {
		t.Fatal(err)
	}
	if aud.Count != 46 || aud.Coverage != (flow.Coverage{Both: 20, EmailOnly: 13, SMSOnly: 13, Unreachable: 3}) {
		t.Errorf("audience = %d %+v, want 46 with coverage 20/13/13/3", aud.Count, aud.Coverage)
	}
	if !strings.Contains(aud.DSL, `city === "İstanbul"`) {
		t.Errorf("dsl = %q", aud.DSL)
	}

	// Approve step by step, as the UI does, until the schedule closes.
	v := s.approve(audienceVariant.ID)
	if a := stepOf(v, flow.KindAudience); a == nil || a.Status != flow.StepStatusApproved {
		t.Fatalf("after approving the audience: %+v", v.Steps)
	}
	plan := s.waitDrafts(info.ID, flow.KindChannelPlan, 1)
	var pc flow.PlanContent
	if err := json.Unmarshal(plan[0].Content, &pc); err != nil {
		t.Fatal(err)
	}
	if pc.Mode != llm.ModePrimaryFallback || pc.Primary != llm.ChannelEmail || pc.Recipients.Email != 33 || pc.Recipients.SMS != 13 ||
		!strings.Contains(pc.Rationale, "mock yanıt") {
		t.Errorf("plan = %+v", pc)
	}
	s.approve(plan[0].ID)

	want := 0
	for _, l := range pc.ByLanguage {
		if l.Email > 0 {
			want++
		}
		if l.SMS > 0 {
			want++
		}
	}
	texts := s.waitDrafts(info.ID, flow.KindContent, want)
	if len(texts) != want || want < 3 {
		t.Fatalf("%d content drafts, want %d (at least 3)", len(texts), want)
	}
	for _, tv := range texts {
		if strings.Contains(string(tv.Content), "[mock]") || !strings.Contains(string(tv.Content), "{{first_name}}") {
			t.Errorf("%s/%s content = %s: want the hand-written text with the name variable", tv.Language, tv.Channel, tv.Content)
		}
	}
	if cost := s.view(info.ID).Cost; cost.TotalUSD == "" || cost.TotalUSD == "0.0000" {
		t.Errorf("cost = %+v", cost)
	}
	for _, tv := range texts {
		s.approve(tv.ID)
	}
	// One schedule slot per (language, timezone) pair of the audience; the sample brief's audience
	// is all in Istanbul, so there is one per language (D-076).
	locales := 0
	for _, l := range aud.ByLocale {
		if l.Count > 0 {
			locales++
		}
	}
	langs := s.waitDrafts(info.ID, flow.KindSchedule, locales)
	if len(langs) != locales || locales != len(pc.ByLanguage) {
		t.Errorf("%d schedule drafts for %d pairs and %d languages", len(langs), locales, len(pc.ByLanguage))
	}
	for _, sv := range langs {
		var sc flow.ScheduleContent
		if err := json.Unmarshal(sv.Content, &sc); err != nil || sc.SendAt != nil || !strings.Contains(sc.Rationale, "mock") {
			t.Errorf("schedule %s = %s (%v)", sv.Language, sv.Content, err)
		}
		if sv.Timezone != "Europe/Istanbul" {
			t.Errorf("schedule %s timezone = %q over REST, want Europe/Istanbul", sv.Language, sv.Timezone)
		}
	}
	var final flow.CampaignView
	for _, sv := range langs {
		final = s.approve(sv.ID)
	}

	// Everything is approved.
	got := s.view(info.ID)
	if len(got.Steps) != 4 || len(final.Steps) != 4 {
		t.Fatalf("steps = %d", len(got.Steps))
	}
	for _, step := range got.Steps {
		if step.Status != flow.StepStatusApproved {
			t.Errorf("step %s is %s", step.Kind, step.Status)
		}
		for _, variant := range step.Variants {
			if variant.Status != flow.VariantStatusApproved {
				t.Errorf("%s variant %s is %s", step.Kind, variant.ID, variant.Status)
			}
		}
	}

	// The stream saw it all, in order, starting with the snapshot.
	time.Sleep(200 * time.Millisecond)
	frames := coll.snapshot()
	counts := map[string]int{}
	for _, fr := range frames {
		counts[fr.event]++
	}
	if frames[0].event != api.EventSnapshot {
		t.Errorf("first frame = %s", frames[0].event)
	}
	if counts[flow.TypeStepOpened] < 3 || counts[flow.TypeStepApproved] != 4 || counts[flow.TypeContentDelta] < want || counts[flow.TypeVariantApproved] != 2+want+len(langs) {
		t.Errorf("event counts = %v", counts)
	}
	// The schedule slots carry their timezone on the stream as well.
	scheduleCreated := 0
	for _, fr := range frames {
		if fr.event != flow.TypeVariantCreated {
			continue
		}
		var ev flow.VariantCreated
		if err := json.Unmarshal([]byte(fr.data), &ev); err != nil {
			t.Fatal(err)
		}
		if ev.Kind == flow.KindSchedule {
			scheduleCreated++
			if ev.Slot.Timezone != "Europe/Istanbul" || ev.Slot.Language == "" {
				t.Errorf("variant_created schedule slot = %+v", ev.Slot)
			}
		}
	}
	if scheduleCreated != locales {
		t.Errorf("%d schedule variant_created events, want %d", scheduleCreated, locales)
	}
	// The text of a content slot arrives in deltas of attempt 1 that join into the stored document.
	var deltas strings.Builder
	slot := flow.Slot{Language: texts[0].Language, Channel: texts[0].Channel}
	for _, fr := range frames {
		if fr.event != flow.TypeContentDelta {
			continue
		}
		var d flow.ContentDelta
		if err := json.Unmarshal([]byte(fr.data), &d); err != nil {
			t.Fatal(err)
		}
		if d.Slot == slot {
			if d.Attempt != 1 {
				t.Errorf("delta attempt = %d", d.Attempt)
			}
			deltas.WriteString(d.Text)
		}
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(deltas.String()), &doc); err != nil || (string(doc["sms"]) == "null") == (string(doc["email"]) == "null") {
		t.Errorf("the deltas of %v do not join into a content document: %q (%v)", slot, deltas.String(), err)
	}
}

func TestRejectAndEditOverREST(t *testing.T) {
	s := newSystem(t)
	info := s.create(sampleBriefText)
	first := s.waitDrafts(info.ID, flow.KindAudience, 1)[0]

	// Reject with a reason: 202, and the slot gets a new draft.
	var v flow.CampaignView
	if code, raw := s.do("POST", "/variants/"+first.ID.String()+"/reject", `{"reason":"Çok dar bir kitle."}`, &v); code != http.StatusAccepted {
		t.Fatalf("reject = %d: %s", code, raw)
	}
	code, raw := s.do("POST", "/variants/"+first.ID.String()+"/reject", `{"reason":""}`, nil)
	asProblem(t, code, raw, 422, api.CodeInvalidRequest)
	deadline := time.Now().Add(60 * time.Second)
	var second flow.VariantView
	for second.ID == uuid.Nil {
		for _, d := range draftsOf(stepOf(s.view(info.ID), flow.KindAudience)) {
			if d.ID != first.ID {
				second = d
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("no replacement draft after the rejection")
		}
		time.Sleep(25 * time.Millisecond)
	}
	for _, vv := range stepOf(s.view(info.ID), flow.KindAudience).Variants {
		if vv.ID == first.ID {
			t.Errorf("the rejected variant is still live: %+v", vv)
		}
	}

	// Edit: a new draft with the edited filter, recompiled and recounted.
	var edited flow.VariantView
	body := `{"content":{"dsl":"city === \"İstanbul\" && language === \"tr\""}}`
	if code, raw := s.do("POST", "/variants/"+second.ID.String()+"/edit", body, &edited); code != 200 {
		t.Fatalf("edit = %d: %s", code, raw)
	}
	var ec flow.AudienceContent
	if err := json.Unmarshal(edited.Content, &ec); err != nil || edited.ID == second.ID || edited.Status != flow.VariantStatusDraft ||
		ec.DSL != `city === "İstanbul" && language === "tr"` || ec.Count == 0 {
		t.Fatalf("edited = %+v content %+v (%v)", edited, ec, err)
	}
	drafts := draftsOf(stepOf(s.view(info.ID), flow.KindAudience))
	if len(drafts) != 1 || drafts[0].ID != edited.ID {
		t.Errorf("drafts after the edit = %+v", drafts)
	}

	// Invalid edits are 422 problems that say why.
	code, raw = s.do("POST", "/variants/"+edited.ID.String()+"/edit", `{"content":{"dsl":"nosuchfield === 1"}}`, nil)
	p := asProblem(t, code, raw, 422, api.CodeInvalidContent)
	if len(p.Problems) == 0 || !strings.Contains(strings.Join(p.Problems, " "), "nosuchfield") {
		t.Errorf("problems = %q", p.Problems)
	}
	code, raw = s.do("POST", "/variants/"+edited.ID.String()+"/edit", `{"content":{"dsl":""}}`, nil)
	asProblem(t, code, raw, 422, api.CodeInvalidContent)

	// A note regenerates the draft and shows up in the campaign.
	var note flow.NoteView
	if code, raw := s.do("POST", "/campaigns/"+info.ID.String()+"/context", `{"body":"Yalnızca Türkçe konuşanlar."}`, &note); code != http.StatusAccepted || note.Body != "Yalnızca Türkçe konuşanlar." {
		t.Fatalf("context = %d: %s", code, raw)
	}
	if got := s.view(info.ID).Notes; len(got) != 1 || got[0].ID != note.ID {
		t.Errorf("notes = %+v", got)
	}
}

func TestConflictsArriveAsProblemJSON(t *testing.T) {
	s := newSystem(t)
	info := s.create(sampleBriefText)
	aud := s.waitDrafts(info.ID, flow.KindAudience, 1)[0]
	s.approve(aud.ID)

	// Approving the same variant again: its step is approved.
	code, raw := s.do("POST", "/variants/"+aud.ID.String()+"/approve", "", nil)
	p := asProblem(t, code, raw, 409, api.CodeStepNotOpen)
	if p.Instance != "/variants/"+aud.ID.String()+"/approve" || p.Detail == "" {
		t.Errorf("problem = %+v", p)
	}
	// Editing and rejecting it, too.
	code, raw = s.do("POST", "/variants/"+aud.ID.String()+"/edit", `{"content":{"dsl":"totalOrders >= 1"}}`, nil)
	asProblem(t, code, raw, 409, api.CodeStepNotOpen)
	code, raw = s.do("POST", "/variants/"+aud.ID.String()+"/reject", `{"reason":"x"}`, nil)
	asProblem(t, code, raw, 409, api.CodeStepNotOpen)
	// Reopening a step that is not approved (the plan is open) and a campaign that has steps.
	code, raw = s.do("POST", "/campaigns/"+info.ID.String()+"/reopen", `{"kind":"channel_plan"}`, nil)
	asProblem(t, code, raw, 409, api.CodeStepNotApproved)
	code, raw = s.do("POST", "/campaigns/"+info.ID.String()+"/regenerate", "", nil)
	asProblem(t, code, raw, 409, api.CodeCampaignHasSteps)
	code, raw = s.do("POST", "/campaigns/"+info.ID.String()+"/reopen", `{"kind":"nope"}`, nil)
	asProblem(t, code, raw, 422, api.CodeInvalidInput)
	// Unknown ids.
	for _, path := range []string{"/variants/" + uuid.NewString() + "/approve", "/steps/" + uuid.NewString() + "/regenerate"} {
		code, raw = s.do("POST", path, "", nil)
		asProblem(t, code, raw, 404, api.CodeNotFound)
	}
	code, raw = s.do("GET", "/campaigns/"+uuid.NewString(), "", nil)
	asProblem(t, code, raw, 404, api.CodeNotFound)
	resp, err := http.Get(s.srv.URL + "/campaigns/" + uuid.NewString() + "/events")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("events of an unknown campaign = %d", resp.StatusCode)
	}

	// Reopening the approved audience works: 202 with the new version, and the later work restarts.
	var step flow.StepInfo
	if code, raw := s.do("POST", "/campaigns/"+info.ID.String()+"/reopen", `{"kind":"audience"}`, &step); code != http.StatusAccepted || step.Version != 2 || step.Kind != flow.KindAudience {
		t.Fatalf("reopen = %d: %s", code, raw)
	}
	s.waitDrafts(info.ID, flow.KindAudience, 1)
	// A step that failed or was interrupted is retried with its regenerate endpoint (202, the view).
	var rv flow.CampaignView
	if code, raw := s.do("POST", "/steps/"+step.ID.String()+"/regenerate", "", &rv); code != http.StatusAccepted || rv.ID != info.ID {
		t.Fatalf("regenerate = %d: %s", code, raw)
	}
}

func TestListCampaignsNewestFirst(t *testing.T) {
	s := newSystem(t)
	var ids []uuid.UUID
	for i := range 3 {
		ids = append(ids, s.create(fmt.Sprintf("Kampanya numarası %d: herkese merhaba.", i)).ID)
		time.Sleep(5 * time.Millisecond)
	}
	var list api.CampaignList
	if code, raw := s.do("GET", "/campaigns?limit=2", "", &list); code != 200 {
		t.Fatalf("list = %d: %s", code, raw)
	}
	if len(list.Items) != 2 || list.Items[0].ID != ids[2] || list.Items[1].ID != ids[1] || list.Limit != 2 {
		t.Errorf("page 1 = %+v, want the newest two (%s, %s)", list, ids[2], ids[1])
	}
	if code, raw := s.do("GET", "/campaigns?limit=2&offset=2", "", &list); code != 200 || len(list.Items) != 1 || list.Items[0].ID != ids[0] || list.Offset != 2 {
		t.Errorf("page 2 = %d %+v (%s)", code, list, raw)
	}
	if code, _ := s.do("GET", "/campaigns?offset=3", "", &list); code != 200 || len(list.Items) != 0 {
		t.Errorf("past the end = %d %+v", code, list)
	}
	// Let the background work end before the pools close.
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	if err := s.svc.WaitIdle(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestAnUnknownBriefGetsTheMarkedMockAnswers(t *testing.T) {
	s := newSystem(t)
	info := s.create("Herkese kısa bir merhaba mesajı gönder.")
	aud := s.waitDrafts(info.ID, flow.KindAudience, 1)[0]
	var ac flow.AudienceContent
	if err := json.Unmarshal(aud.Content, &ac); err != nil || ac.DSL != mock.DefaultDSL || ac.Count == 0 {
		t.Fatalf("audience = %s (%v), want the default filter with an audience", aud.Content, err)
	}
	s.approve(aud.ID)
	plan := s.waitDrafts(info.ID, flow.KindChannelPlan, 1)[0]
	s.approve(plan.ID)
	var pc flow.PlanContent
	if err := json.Unmarshal(plan.Content, &pc); err != nil {
		t.Fatal(err)
	}
	texts := s.waitDrafts(info.ID, flow.KindContent, 2)
	for _, tv := range texts {
		if !strings.Contains(string(tv.Content), "[mock]") {
			t.Errorf("%s/%s content = %s: want the [mock] marker", tv.Language, tv.Channel, tv.Content)
		}
	}
}

// Every filter the mock proposes selects somebody in the seeded customer database, so a keyless demo
// never ends in "empty audience".
func TestEveryMockFilterHasAnAudienceInTheSeed(t *testing.T) {
	cust := customerClient(t)
	tools := llm.NewCustomerTools(cust)
	m := mock.New(mock.WithChunkDelay(0))
	for _, c := range append(mock.Cases(), mock.Default()) {
		brief := c.Brief
		if c.Name == "default" {
			brief = "Eşleşmeyen bir brief."
		}
		res, err := m.ProposeSegment(t.Context(), llm.SegmentInput{Brief: brief}, tools)
		if err != nil {
			t.Errorf("%s: %v", c.Name, err)
			continue
		}
		t.Logf("%-18s %4d reachable  %s", c.Name, res.Preview.Count, c.DSL)
	}
	// The filter of the mock's answered question (D-088) too.
	res, err := m.ProposeSegment(t.Context(), llm.SegmentInput{Brief: "Kim?", QuestionsLeft: 1,
		Clarifications: []llm.Clarification{{Question: mock.Question.Text, Answer: mock.Question.Options[1]}}}, tools)
	if err != nil || res.DSL != mock.QuestionDSL || res.Preview.Count == 0 {
		t.Errorf("question filter: %+v, %v", res, err)
	}
	t.Logf("%-18s %4d reachable  %s", "question", res.Preview.Count, mock.QuestionDSL)
}

// replacement waits until the open step of kind holds a draft in slot that is not old, and returns it.
func (s *system) replacement(id uuid.UUID, kind string, slot flow.Slot, old uuid.UUID) flow.VariantView {
	s.t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		if st := stepOf(s.view(id), kind); st != nil {
			for _, d := range draftsOf(st) {
				if d.ID != old && d.Language == slot.Language && d.Channel == slot.Channel {
					return d
				}
			}
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("no replacement draft of the %s step in slot %v", kind, slot)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// A rejection makes the mock regenerate; it says so in the plan rationale and in the content text.
func TestRegenerationAfterARejectionIsMarkedByTheMock(t *testing.T) {
	s := newSystem(t)
	info := s.create(sampleBriefText)
	s.approve(s.waitDrafts(info.ID, flow.KindAudience, 1)[0].ID)

	plan := s.waitDrafts(info.ID, flow.KindChannelPlan, 1)[0]
	var pc flow.PlanContent
	if err := json.Unmarshal(plan.Content, &pc); err != nil || strings.Contains(pc.Rationale, "yeniden üretim") {
		t.Fatalf("first plan = %s (%v): must not be marked as a regeneration", plan.Content, err)
	}
	if code, raw := s.do("POST", "/variants/"+plan.ID.String()+"/reject", `{"reason":"Önce SMS olsun"}`, nil); code != http.StatusAccepted {
		t.Fatalf("reject plan = %d: %s", code, raw)
	}
	plan = s.replacement(info.ID, flow.KindChannelPlan, flow.Slot{}, plan.ID)
	if err := json.Unmarshal(plan.Content, &pc); err != nil || !strings.HasPrefix(pc.Rationale, "[mock] yeniden üretim (regeneration): Önce SMS olsun. ") {
		t.Fatalf("regenerated plan rationale = %q (%v)", pc.Rationale, err)
	}
	s.approve(plan.ID)

	texts := s.waitDrafts(info.ID, flow.KindContent, 3)
	for _, tv := range texts {
		if strings.Contains(string(tv.Content), "[mock]") {
			t.Fatalf("first content %s/%s is marked: %s", tv.Language, tv.Channel, tv.Content)
		}
	}
	victim := texts[0]
	if code, raw := s.do("POST", "/variants/"+victim.ID.String()+"/reject", `{"reason":"Çok resmi."}`, nil); code != http.StatusAccepted {
		t.Fatalf("reject content = %d: %s", code, raw)
	}
	regen := s.replacement(info.ID, flow.KindContent, flow.Slot{Language: victim.Language, Channel: victim.Channel}, victim.ID)
	if !strings.Contains(string(regen.Content), "[mock]") || !strings.Contains(string(regen.Content), "{{first_name}}") {
		t.Errorf("regenerated content = %s: want the [mock] marker and the name variable", regen.Content)
	}
	// The other slots keep their unmarked text.
	for _, d := range draftsOf(stepOf(s.view(info.ID), flow.KindContent)) {
		if d.ID != regen.ID && strings.Contains(string(d.Content), "[mock]") {
			t.Errorf("%s/%s was marked though it was not regenerated: %s", d.Language, d.Channel, d.Content)
		}
	}
}
