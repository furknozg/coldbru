//go:build integration

package confirm_test

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
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/testcontainers/testcontainers-go"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/customer/audience"
	"github.com/furknozg/coldbru/internal/customer/auth"
	"github.com/furknozg/coldbru/internal/customer/consent"
	"github.com/furknozg/coldbru/internal/customer/graph"
	"github.com/furknozg/coldbru/internal/customer/pgtest"
	"github.com/furknozg/coldbru/internal/platform/db"
	"github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/platform/migrate"
	"github.com/furknozg/coldbru/internal/platform/outbox"
	"github.com/furknozg/coldbru/internal/proposal/api"
	"github.com/furknozg/coldbru/internal/proposal/compile"
	"github.com/furknozg/coldbru/internal/proposal/confirm"
	"github.com/furknozg/coldbru/internal/proposal/contentgen"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/mock"
	"github.com/furknozg/coldbru/internal/proposal/store"
)

// The harness follows the integration tests of internal/proposal/api (a test file cannot be
// imported): a seeded Postgres with the real Customer handler, one proposal database per test, the
// mock LLM, and here also a NATS server with the EVENTS and RECIPIENTS streams and the outbox relay.

const (
	proposalToken = "confirm-it-token"
	sampleBrief   = "Son 30 günde sepetine ürün ekleyip alışveriş yapmayan İstanbul'daki kullanıcılara %30 indirim hatırlatması gönder. Samimi ama kısa olsun."
)

var (
	pgOnce sync.Once
	pgEnv  *pgtest.Env
	pgErr  error

	natsOnce sync.Once
	natsCtr  *tcnats.NATSContainer
	natsURL  string
	natsErr  error

	dbSeqMu sync.Mutex
	dbSeq   int
)

func TestMain(m *testing.M) {
	code := m.Run()
	if pgEnv != nil {
		pgEnv.Close()
	}
	if natsCtr != nil {
		_ = testcontainers.TerminateContainer(natsCtr)
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

func natsAddr(t *testing.T) string {
	t.Helper()
	natsOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		natsCtr, natsErr = tcnats.Run(ctx, "nats:2.15-alpine")
		if natsErr == nil {
			natsURL, natsErr = natsCtr.ConnectionString(ctx)
		}
	})
	if natsErr != nil {
		t.Fatalf("start nats: %v", natsErr)
	}
	return natsURL
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
	name := fmt.Sprintf("confirm_test_%d_%d", os.Getpid(), dbSeq)
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

// switchable is the Customer the Confirmer sees: the real client, or a failure or an empty audience
// while the test needs the confirmation to fail.
type switchable struct {
	real *customer.Client
	mode atomic.Int32 // 0 real, 1 error, 2 empty audience
}

func (s *switchable) EachRecipientPage(ctx context.Context, f *compile.Filter, fn func([]customer.Recipient) error) error {
	switch s.mode.Load() {
	case 1:
		return &customer.StatusError{Status: 500, Body: "boom"}
	case 2:
		return nil
	}
	return s.real.EachRecipientPage(ctx, f, fn)
}

type system struct {
	t      *testing.T
	srv    *httptest.Server
	svc    *flow.Service
	pool   *pgxpool.Pool
	cust   *customer.Client
	cust2  *switchable
	conf   *confirm.Confirmer
	js     jetstream.JetStream
	events jetstream.Stream
	recips jetstream.Stream
}

func newSystem(t *testing.T) *system {
	t.Helper()
	pool := proposalPool(t)
	cust := customerClient(t)
	sw := &switchable{real: cust}

	nc, err := nats.Connect(natsAddr(t))
	if err != nil {
		t.Fatalf("nats.Connect: %v", err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	evStream, err := js.CreateStream(t.Context(), jetstream.StreamConfig{Name: "EVENTS", Subjects: []string{"evt.>"}})
	if err != nil {
		t.Fatalf("create EVENTS: %v", err)
	}
	rcStream, err := js.CreateStream(t.Context(), jetstream.StreamConfig{Name: "RECIPIENTS", Subjects: []string{"recipients.>"}, Retention: jetstream.WorkQueuePolicy})
	if err != nil {
		t.Fatalf("create RECIPIENTS: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.WithoutCancel(t.Context())
		_ = js.DeleteStream(ctx, "EVENTS")
		_ = js.DeleteStream(ctx, "RECIPIENTS")
	})

	relayCtx, stopRelay := context.WithCancel(context.Background())
	relayDone := make(chan struct{})
	go func() {
		defer close(relayDone)
		_ = (&outbox.Relay{Pool: pool, JS: js, Logger: discard(), PollInterval: 100 * time.Millisecond}).Run(relayCtx)
	}()
	t.Cleanup(func() { stopRelay(); <-relayDone })

	hub := flow.NewHub(0)
	conf, err := confirm.New(confirm.Deps{Pool: pool, Customer: sw, CTAURL: content.DefaultCTAURL, Hub: hub, Logger: discard()})
	if err != nil {
		t.Fatal(err)
	}
	model := mock.New(mock.WithChunkDelay(0))
	sms, _ := content.ParseMoney("0.04")
	email, _ := content.ParseMoney("0.001")
	svc, err := flow.New(flow.Deps{
		Pool: pool, LLM: model, Customer: cust, Content: contentgen.New(model, content.Prices{SMSSegment: sms, Email: email}),
		Confirmer: conf, Hub: hub, Logger: discard(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = svc.Close(ctx)
	})
	r := chi.NewRouter()
	api.New(api.Config{Flow: svc, Logger: discard()}).Mount(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return &system{t: t, srv: srv, svc: svc, pool: pool, cust: cust, cust2: sw, conf: conf, js: js, events: evStream, recips: rcStream}
}

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

func (s *system) waitDrafts(id uuid.UUID, kind string, n int) []flow.VariantView {
	s.t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for {
		v := s.view(id)
		if st := stepOf(v, kind); st != nil && st.Status == flow.StepStatusOpen {
			if st.Failed {
				s.t.Fatalf("the %s step failed: %s", kind, st.Reason)
			}
			var d []flow.VariantView
			for _, vv := range st.Variants {
				if vv.Status == flow.VariantStatusDraft {
					d = append(d, vv)
				}
			}
			if len(d) >= n {
				return d
			}
		}
		if time.Now().After(deadline) {
			s.t.Fatalf("no %d drafts of the %s step in time: %+v", n, kind, v.Steps)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// approve approves a variant and returns the status code and body.
func (s *system) approve(variant uuid.UUID) (int, []byte) {
	s.t.Helper()
	return s.do("POST", "/variants/"+variant.String()+"/approve", "", nil)
}

func (s *system) mustApprove(variant uuid.UUID) {
	s.t.Helper()
	if code, raw := s.approve(variant); code != 200 {
		s.t.Fatalf("approve = %d: %s", code, raw)
	}
}

// walk creates the sample campaign and approves every step but the last variant of the schedule
// step, which it returns. The audience and plan variants are returned decoded.
type walked struct {
	id        uuid.UUID
	aud       flow.AudienceContent
	plan      flow.PlanContent
	languages int
	schedules []flow.VariantView
}

func (s *system) walk() walked {
	s.t.Helper()
	var info flow.CampaignInfo
	b, _ := json.Marshal(map[string]string{"brief": sampleBrief})
	if code, raw := s.do("POST", "/campaigns", string(b), &info); code != http.StatusAccepted {
		s.t.Fatalf("POST /campaigns = %d: %s", code, raw)
	}
	w := walked{id: info.ID}
	aud := s.waitDrafts(info.ID, flow.KindAudience, 1)[0]
	if err := json.Unmarshal(aud.Content, &w.aud); err != nil {
		s.t.Fatal(err)
	}
	s.mustApprove(aud.ID)
	plan := s.waitDrafts(info.ID, flow.KindChannelPlan, 1)[0]
	if err := json.Unmarshal(plan.Content, &w.plan); err != nil {
		s.t.Fatal(err)
	}
	s.mustApprove(plan.ID)
	want := 0
	for _, l := range w.plan.ByLanguage {
		if l.Email > 0 {
			want++
		}
		if l.SMS > 0 {
			want++
		}
	}
	for _, tv := range s.waitDrafts(info.ID, flow.KindContent, want) {
		s.mustApprove(tv.ID)
	}
	w.languages = len(w.plan.ByLanguage)
	w.schedules = s.waitDrafts(info.ID, flow.KindSchedule, w.languages)
	return w
}

// approveSchedules approves every schedule variant and returns the status of the last approval,
// the one that triggers the confirmation.
func (s *system) approveSchedules(w walked) (int, []byte) {
	s.t.Helper()
	for _, sv := range w.schedules[:len(w.schedules)-1] {
		s.mustApprove(sv.ID)
	}
	return s.approve(w.schedules[len(w.schedules)-1].ID)
}

// confirmed walks the campaign all the way to `scheduled`.
func (s *system) confirmed() walked {
	s.t.Helper()
	w := s.walk()
	if code, raw := s.approveSchedules(w); code != 200 {
		s.t.Fatalf("the last schedule approval = %d: %s", code, raw)
	}
	return w
}

// ---- NATS and outbox helpers ----

func (s *system) subjectCount(stream jetstream.Stream, subject string) int {
	s.t.Helper()
	info, err := stream.Info(s.t.Context(), jetstream.WithSubjectFilter(subject))
	if err != nil {
		s.t.Fatalf("stream info: %v", err)
	}
	return int(info.State.Subjects[subject])
}

func (s *system) waitSubject(stream jetstream.Stream, subject string, n int) {
	s.t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for s.subjectCount(stream, subject) != n {
		if time.Now().After(deadline) {
			s.t.Fatalf("%s has %d messages on %s, want %d", subject, s.subjectCount(stream, subject), stream.CachedInfo().Config.Name, n)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (s *system) outboxRows() (total, unpublished int) {
	s.t.Helper()
	if err := s.pool.QueryRow(s.t.Context(), `SELECT count(*), count(*) FILTER (WHERE published_at IS NULL) FROM outbox`).Scan(&total, &unpublished); err != nil {
		s.t.Fatalf("count outbox: %v", err)
	}
	return total, unpublished
}

func (s *system) lastMsg(stream jetstream.Stream, subject string) *jetstream.RawStreamMsg {
	s.t.Helper()
	m, err := stream.GetLastMsgForSubject(s.t.Context(), subject)
	if err != nil {
		s.t.Fatalf("last message of %s: %v", subject, err)
	}
	return m
}

// settle waits long enough for the relay (100 ms poll) to publish anything still pending.
func (s *system) settle() {
	s.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, unpublished := s.outboxRows(); unpublished == 0 {
			break
		}
		if time.Now().After(deadline) {
			s.t.Fatal("the relay did not publish the outbox")
		}
		time.Sleep(25 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond) // a few more polls
}

func (s *system) status(id uuid.UUID) string { return s.view(id).Status }

// ---- tests ----

func TestSampleBriefConfirmsAndPublishes(t *testing.T) {
	s := newSystem(t)
	w := s.walk()
	if w.aud.Count != 46 || w.plan.Mode != llm.ModePrimaryFallback || w.plan.Primary != llm.ChannelEmail {
		t.Fatalf("audience %d plan %s/%s, want 46 and primary_fallback/email", w.aud.Count, w.plan.Mode, w.plan.Primary)
	}
	sub := s.svc.Hub().Subscribe(w.id)
	defer sub.Close()

	if code, raw := s.approveSchedules(w); code != 200 {
		t.Fatalf("the last schedule approval = %d: %s", code, raw)
	}
	if got := s.status(w.id); got != flow.CampaignStatusScheduled {
		t.Fatalf("status = %q, want scheduled", got)
	}

	// The hub told the event streams.
	var confirmedEv *flow.CampaignConfirmed
	timeout := time.After(5 * time.Second)
	for confirmedEv == nil {
		select {
		case ev := <-sub.C:
			if c, ok := ev.(flow.CampaignConfirmed); ok {
				confirmedEv = &c
			}
		case <-timeout:
			t.Fatal("no campaign_confirmed event on the hub")
		}
	}
	if confirmedEv.RecipientCounts != (flow.Recipients{Email: 33, SMS: 13, Total: 46}) || confirmedEv.RecipientBatches != 1 {
		t.Errorf("campaign_confirmed = %+v", confirmedEv)
	}

	// EVENTS: exactly one confirmed event.
	s.waitSubject(s.events, "evt.proposal.confirmed.v1", 1)
	msg := s.lastMsg(s.events, "evt.proposal.confirmed.v1")
	var env events.Envelope
	if err := json.Unmarshal(msg.Data, &env); err != nil {
		t.Fatal(err)
	}
	if env.Type != "proposal.confirmed" || env.Version != 1 || env.Producer != "proposal" || env.CorrelationID != w.id.String() || env.ID == "" {
		t.Errorf("envelope = %+v", env)
	}
	if got := msg.Header.Get(jetstream.MsgIDHeader); got != env.ID {
		t.Errorf("Nats-Msg-Id = %q, want the envelope id %q", got, env.ID)
	}
	var data flow.ConfirmedData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatal(err)
	}
	if data.CampaignID != w.id.String() || data.RecipientCounts != (flow.Recipients{Email: 33, SMS: 13, Total: 46}) || data.RecipientBatches != 1 {
		t.Errorf("data = %+v", data)
	}
	if len(data.Messages) != 3 || len(data.Schedules) != w.languages {
		t.Errorf("%d messages, %d schedules, want 3 and %d", len(data.Messages), len(data.Schedules), w.languages)
	}
	for _, m := range data.Messages {
		if !strings.Contains(string(m.Content), "{{first_name}}") || (m.Channel != "sms" && m.Channel != "email") || m.Language == "" {
			t.Errorf("message %s/%s content = %s", m.Language, m.Channel, m.Content)
		}
	}
	for _, sc := range data.Schedules {
		if sc.SendAt != nil { // the mock sends immediately
			t.Errorf("schedule %s send_at = %v, want null", sc.Language, sc.SendAt)
		}
	}
	wantCTA, _ := content.CTAURL(content.DefaultCTAURL, w.id.String())
	if data.CTAURL != wantCTA || !strings.Contains(data.CTAURL, "utm_campaign="+w.id.String()) {
		t.Errorf("cta_url = %q, want %q", data.CTAURL, wantCTA)
	}

	// RECIPIENTS: ceil(46/500) = 1 batch of 46, channels as planned, contacts of the right kind.
	subject := "recipients." + w.id.String()
	s.waitSubject(s.recips, subject, 1)
	bm := s.lastMsg(s.recips, subject)
	var batch flow.RecipientBatch
	if err := json.Unmarshal(bm.Data, &batch); err != nil {
		t.Fatal(err)
	}
	if batch.CampaignID != w.id.String() || batch.BatchNo != 1 || batch.BatchCount != 1 || len(batch.Recipients) != 46 {
		t.Fatalf("batch = %s %d/%d with %d recipients", batch.CampaignID, batch.BatchNo, batch.BatchCount, len(batch.Recipients))
	}
	f, errs := compile.Compile(w.aud.DSL)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	all, err := s.cust.AllRecipients(t.Context(), f)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]customer.Recipient{}
	for _, r := range all {
		byID[r.CustomerID] = r
	}
	seen := map[string]bool{}
	var email, sms int
	for _, r := range batch.Recipients {
		src, ok := byID[r.CustomerID]
		if !ok || seen[r.CustomerID] {
			t.Fatalf("recipient %s unknown or listed twice", r.CustomerID)
		}
		seen[r.CustomerID] = true
		if r.FirstName != src.FirstName || r.Language != src.Language {
			t.Errorf("recipient %+v differs from Customer's %+v", r, src)
		}
		switch r.Channel {
		case "email": // primary: everybody who can get an email
			email++
			if !src.EmailEligible || r.Contact != src.Email || r.Contact == "" {
				t.Errorf("email recipient %+v: Customer says %+v", r, src)
			}
		case "sms": // fallback: only people without an email channel
			sms++
			if src.EmailEligible || !src.SMSEligible || r.Contact != src.Phone || r.Contact == "" {
				t.Errorf("sms recipient %+v: Customer says %+v", r, src)
			}
		default:
			t.Errorf("recipient %+v has channel %q", r, r.Channel)
		}
	}
	if email != 33 || sms != 13 {
		t.Errorf("batch has %d email and %d sms, want 33 and 13", email, sms)
	}

	// The outbox row ids are the Nats-Msg-Ids, for the envelope and for the raw batch.
	var rowID string
	if err := s.pool.QueryRow(t.Context(), `SELECT id::text FROM outbox WHERE subject = $1`, subject).Scan(&rowID); err != nil {
		t.Fatal(err)
	}
	if got := bm.Header.Get(jetstream.MsgIDHeader); got != rowID {
		t.Errorf("batch Nats-Msg-Id = %q, want the outbox row id %q", got, rowID)
	}
	if body := string(bm.Data); strings.Contains(body, `"id"`) {
		t.Errorf("the raw batch has an id member: %.200s", body)
	}
	if total, _ := s.outboxRows(); total != 2 {
		t.Errorf("outbox rows = %d, want 2 (the event and one batch)", total)
	}
}

func TestSecondConfirmationIsANoOp(t *testing.T) {
	s := newSystem(t)
	w := s.confirmed()
	s.waitSubject(s.events, "evt.proposal.confirmed.v1", 1)
	s.waitSubject(s.recips, "recipients."+w.id.String(), 1)
	s.settle()
	rowsBefore, _ := s.outboxRows()

	// Through the REST endpoint: the campaign is no longer a draft.
	code, raw := s.do("POST", "/campaigns/"+w.id.String()+"/confirm", "", nil)
	assertProblem(t, code, raw, 409, api.CodeCampaignNotDrafting)
	// Straight through the Confirmer, as a racing caller would: nil and nothing written.
	if err := s.conf.Confirm(t.Context(), w.id); err != nil {
		t.Fatalf("Confirm on a scheduled campaign = %v, want nil", err)
	}
	s.settle()
	if rowsAfter, _ := s.outboxRows(); rowsAfter != rowsBefore {
		t.Errorf("outbox rows %d -> %d: the second confirmation wrote", rowsBefore, rowsAfter)
	}
	if n := s.subjectCount(s.events, "evt.proposal.confirmed.v1"); n != 1 {
		t.Errorf("confirmed events = %d, want 1", n)
	}
	if n := s.subjectCount(s.recips, "recipients."+w.id.String()); n != 1 {
		t.Errorf("recipient batches = %d, want 1", n)
	}
	if got := s.status(w.id); got != flow.CampaignStatusScheduled {
		t.Errorf("status = %q", got)
	}
}

func TestConfirmRetryAfterCustomerFailureAndNoRecipients(t *testing.T) {
	s := newSystem(t)
	w := s.walk()

	// Customer is down: the last approval stands, the confirmation fails with 502.
	s.cust2.mode.Store(1)
	code, raw := s.approveSchedules(w)
	assertProblem(t, code, raw, 502, api.CodeConfirmFailed)
	if got := s.status(w.id); got != flow.CampaignStatusDrafting {
		t.Fatalf("status after the failed confirmation = %q, want drafting", got)
	}
	if st := stepOf(s.view(w.id), flow.KindSchedule); st == nil || st.Status != flow.StepStatusApproved {
		t.Fatalf("the schedule step = %+v, want approved", st)
	}
	code, raw = s.do("POST", "/campaigns/"+w.id.String()+"/confirm", "", nil)
	assertProblem(t, code, raw, 502, api.CodeConfirmFailed)

	// Customer answers, but nobody is left: 409 no_recipients.
	s.cust2.mode.Store(2)
	code, raw = s.do("POST", "/campaigns/"+w.id.String()+"/confirm", "", nil)
	assertProblem(t, code, raw, 409, api.CodeNoRecipients)

	// Nothing was written meanwhile.
	s.settle()
	if total, _ := s.outboxRows(); total != 0 {
		t.Fatalf("outbox rows = %d after failed confirmations, want 0", total)
	}
	if n := s.subjectCount(s.events, "evt.proposal.confirmed.v1"); n != 0 {
		t.Fatalf("confirmed events = %d, want 0", n)
	}

	// Customer is back: the retry endpoint confirms.
	s.cust2.mode.Store(0)
	var v flow.CampaignView
	if code, raw := s.do("POST", "/campaigns/"+w.id.String()+"/confirm", "", &v); code != 200 || v.Status != flow.CampaignStatusScheduled {
		t.Fatalf("confirm = %d %s, status %q", code, raw, v.Status)
	}
	s.waitSubject(s.events, "evt.proposal.confirmed.v1", 1)
	s.waitSubject(s.recips, "recipients."+w.id.String(), 1)
}

func TestConfirmBeforeTheScheduleIsApprovedIs409(t *testing.T) {
	s := newSystem(t)
	w := s.walk() // the schedule step is open, nothing approved in it
	code, raw := s.do("POST", "/campaigns/"+w.id.String()+"/confirm", "", nil)
	assertProblem(t, code, raw, 409, api.CodeScheduleNotApproved)
	// A campaign that has no steps yet is the same.
	var info flow.CampaignInfo
	if code, raw := s.do("POST", "/campaigns", `{"brief":"`+sampleBrief+`"}`, &info); code != http.StatusAccepted {
		t.Fatalf("create = %d %s", code, raw)
	}
	code, raw = s.do("POST", "/campaigns/"+info.ID.String()+"/confirm", "", nil)
	if code != 409 {
		t.Errorf("confirm of a fresh campaign = %d %s, want 409", code, raw)
	}
	code, raw = s.do("POST", "/campaigns/"+uuid.NewString()+"/confirm", "", nil)
	assertProblem(t, code, raw, 404, api.CodeNotFound)
	s.settle()
	if total, _ := s.outboxRows(); total != 0 {
		t.Errorf("outbox rows = %d, want 0", total)
	}
}

func TestConcurrentConfirmationsWriteOnce(t *testing.T) {
	s := newSystem(t)
	w := s.walk()
	s.cust2.mode.Store(1) // the automatic confirmation fails, so five retries can race
	if code, _ := s.approveSchedules(w); code != 502 {
		t.Fatalf("the last approval = %d, want 502", code)
	}
	s.cust2.mode.Store(0)

	var wg sync.WaitGroup
	codes := make([]int, 5)
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i], _ = s.do("POST", "/campaigns/"+w.id.String()+"/confirm", "", nil)
		}()
	}
	wg.Wait()
	ok := 0
	for _, c := range codes {
		switch c {
		case 200:
			ok++
		case 409: // arrived after the winner committed
		default:
			t.Errorf("status %d among %v", c, codes)
		}
	}
	if ok == 0 {
		t.Errorf("statuses = %v, want at least one 200", codes)
	}
	s.waitSubject(s.events, "evt.proposal.confirmed.v1", 1)
	s.settle()
	if n := s.subjectCount(s.events, "evt.proposal.confirmed.v1"); n != 1 {
		t.Errorf("confirmed events = %d, want exactly 1", n)
	}
	if total, _ := s.outboxRows(); total != 2 {
		t.Errorf("outbox rows = %d, want 2", total)
	}
}

func TestCancelScheduledCampaignPublishesCancelled(t *testing.T) {
	s := newSystem(t)
	w := s.confirmed()
	s.waitSubject(s.events, "evt.proposal.confirmed.v1", 1)
	sub := s.svc.Hub().Subscribe(w.id)
	defer sub.Close()

	var v flow.CampaignView
	if code, raw := s.do("POST", "/campaigns/"+w.id.String()+"/cancel", `{"reason":"Yanlış indirim oranı."}`, &v); code != 200 || v.Status != flow.CampaignStatusCancelled {
		t.Fatalf("cancel = %d %s, status %q", code, raw, v.Status)
	}
	s.waitSubject(s.events, "evt.proposal.cancelled.v1", 1)
	msg := s.lastMsg(s.events, "evt.proposal.cancelled.v1")
	var env events.Envelope
	if err := json.Unmarshal(msg.Data, &env); err != nil {
		t.Fatal(err)
	}
	var data flow.CancelledData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatal(err)
	}
	if env.Type != "proposal.cancelled" || env.CorrelationID != w.id.String() || msg.Header.Get(jetstream.MsgIDHeader) != env.ID ||
		data.CampaignID != w.id.String() || data.Reason != "Yanlış indirim oranı." {
		t.Errorf("envelope %+v data %+v", env, data)
	}
	select {
	case ev := <-sub.C:
		if c, ok := ev.(flow.CampaignCancelled); !ok || c.Reason != "Yanlış indirim oranı." {
			t.Errorf("hub event = %#v", ev)
		}
	case <-time.After(5 * time.Second):
		t.Error("no campaign_cancelled event on the hub")
	}

	// A second cancel is a conflict and publishes nothing; so is any other action.
	code, raw := s.do("POST", "/campaigns/"+w.id.String()+"/cancel", "", nil)
	assertProblem(t, code, raw, 409, api.CodeNotCancellable)
	code, raw = s.do("POST", "/campaigns/"+w.id.String()+"/confirm", "", nil)
	assertProblem(t, code, raw, 409, api.CodeCampaignNotDrafting)
	s.settle()
	if n := s.subjectCount(s.events, "evt.proposal.cancelled.v1"); n != 1 {
		t.Errorf("cancelled events = %d, want 1", n)
	}
}

func TestCancelWithoutAReasonUsesTheDefault(t *testing.T) {
	s := newSystem(t)
	w := s.confirmed()
	if code, raw := s.do("POST", "/campaigns/"+w.id.String()+"/cancel", "", nil); code != 200 {
		t.Fatalf("cancel = %d %s", code, raw)
	}
	s.waitSubject(s.events, "evt.proposal.cancelled.v1", 1)
	var env events.Envelope
	if err := json.Unmarshal(s.lastMsg(s.events, "evt.proposal.cancelled.v1").Data, &env); err != nil {
		t.Fatal(err)
	}
	var data flow.CancelledData
	if err := json.Unmarshal(env.Data, &data); err != nil || data.Reason != flow.DefaultCancelReason {
		t.Errorf("data = %+v (%v), want the default reason", data, err)
	}
}

func TestCancelDraftingCampaignPublishesNothing(t *testing.T) {
	s := newSystem(t)
	var info flow.CampaignInfo
	b, _ := json.Marshal(map[string]string{"brief": sampleBrief})
	if code, raw := s.do("POST", "/campaigns", string(b), &info); code != http.StatusAccepted {
		t.Fatalf("create = %d %s", code, raw)
	}
	draft := s.waitDrafts(info.ID, flow.KindAudience, 1)[0]
	sub := s.svc.Hub().Subscribe(info.ID)
	defer sub.Close()

	var v flow.CampaignView
	if code, raw := s.do("POST", "/campaigns/"+info.ID.String()+"/cancel", `{"reason":"changed my mind"}`, &v); code != 200 || v.Status != flow.CampaignStatusCancelled {
		t.Fatalf("cancel = %d %s, status %q", code, raw, v.Status)
	}
	select {
	case ev := <-sub.C:
		if _, ok := ev.(flow.CampaignCancelled); !ok {
			t.Errorf("hub event = %#v, want campaign_cancelled", ev)
		}
	case <-time.After(5 * time.Second):
		t.Error("no campaign_cancelled event on the hub")
	}

	// Nothing leaves the service, and the flow refuses to go on.
	s.settle()
	if total, _ := s.outboxRows(); total != 0 {
		t.Errorf("outbox rows = %d, want 0", total)
	}
	if info, err := s.events.Info(t.Context()); err != nil || info.State.Msgs != 0 {
		t.Errorf("EVENTS = %+v (%v), want empty", info.State, err)
	}
	code, raw := s.approve(draft.ID)
	assertProblem(t, code, raw, 409, api.CodeCampaignNotDrafting)
	code, raw = s.do("POST", "/campaigns/"+info.ID.String()+"/cancel", "", nil)
	assertProblem(t, code, raw, 409, api.CodeNotCancellable)
	code, raw = s.do("POST", "/campaigns/"+uuid.NewString()+"/cancel", "", nil)
	assertProblem(t, code, raw, 404, api.CodeNotFound)
}

func assertProblem(t *testing.T, code int, raw []byte, wantStatus int, wantCode string) {
	t.Helper()
	var p api.Problem
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("decode problem %s: %v", raw, err)
	}
	if code != wantStatus || p.Code != wantCode {
		t.Fatalf("status %d problem %+v, want %d %s", code, p, wantStatus, wantCode)
	}
}
