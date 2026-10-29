//go:build integration

package sender_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/testcontainers/testcontainers-go"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/customer/audience"
	"github.com/furknozg/coldbru/internal/customer/auth"
	custconsent "github.com/furknozg/coldbru/internal/customer/consent"
	"github.com/furknozg/coldbru/internal/customer/graph"
	"github.com/furknozg/coldbru/internal/customer/pgtest"
	nconsent "github.com/furknozg/coldbru/internal/notification/consent"
	"github.com/furknozg/coldbru/internal/notification/intake"
	"github.com/furknozg/coldbru/internal/notification/provider"
	"github.com/furknozg/coldbru/internal/notification/sender"
	"github.com/furknozg/coldbru/internal/notification/store"
	"github.com/furknozg/coldbru/internal/platform/consumer"
	"github.com/furknozg/coldbru/internal/platform/db"
	"github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/platform/migrate"
	"github.com/furknozg/coldbru/internal/platform/outbox"
	"github.com/furknozg/coldbru/internal/proposal/flow"
)

const (
	notificationToken = "notification-it-token"
	proposalToken     = "proposal-it-token"
	adminToken        = "admin-it-token"

	istanbul = "Europe/Istanbul"
)

var (
	baseDSN   string
	adminPool *pgxpool.Pool
	natsURL   string

	// customerEnv is a Postgres seeded like the compose stack; the real Customer handler runs on it.
	customerEnv *pgtest.Env
	// smsCustomers and emailCustomers are seeded customers who are eligible on the channel.
	smsCustomers, emailCustomers []cust

	dbSeq   int
	dbSeqMu sync.Mutex
)

// cust is a seeded customer as a recipient.
type cust struct {
	ID, FirstName, Contact string
}

func TestMain(m *testing.M) { os.Exit(run(m)) }

func run(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
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

	customerEnv, err = pgtest.Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start seeded customer postgres: %v\n", err)
		return 1
	}
	defer customerEnv.Close()
	smsCustomers, err = loadCustomers(ctx, "phone_e164", audience.SQLSMSOK)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load sms customers: %v\n", err)
		return 1
	}
	emailCustomers, err = loadCustomers(ctx, "email", audience.SQLEmailOK)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load email customers: %v\n", err)
		return 1
	}
	if len(smsCustomers) < 400 || len(emailCustomers) < 100 {
		fmt.Fprintf(os.Stderr, "seed has %d sms and %d email customers, want 400 and 100\n", len(smsCustomers), len(emailCustomers))
		return 1
	}
	return m.Run()
}

// loadCustomers returns active customers eligible on a channel, by id; contact is the column the
// channel sends to. At most 600 are returned.
func loadCustomers(ctx context.Context, contactColumn, channelOK string) ([]cust, error) {
	rows, err := customerEnv.Pool.Query(ctx, `SELECT customer_id, first_name, `+contactColumn+`
		FROM dwh.dim_customer c WHERE `+audience.SQLActive+` AND `+channelOK+` ORDER BY customer_id LIMIT 600`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []cust
	for rows.Next() {
		var c cust
		if err := rows.Scan(&c.ID, &c.FirstName, &c.Contact); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// streamDir holds the real stream definitions the compose stack loads (D-049).
var streamDir = filepath.Join("..", "..", "..", "infra", "nats", "streams")

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

// logCapture is a slog handler that keeps every record for assertions.
type logCapture struct {
	mu      sync.Mutex
	records []logRecord
}

type logRecord struct {
	level slog.Level
	msg   string
	attrs map[string]any
}

func (l *logCapture) Enabled(context.Context, slog.Level) bool      { return true }
func (l *logCapture) WithGroup(string) slog.Handler                 { return l }
func (l *logCapture) WithAttrs(a []slog.Attr) slog.Handler          { return &logView{parent: l, attrs: a} }
func (l *logCapture) Handle(_ context.Context, r slog.Record) error { return l.add(r, nil) }

func (l *logCapture) add(r slog.Record, extra []slog.Attr) error {
	rec := logRecord{level: r.Level, msg: r.Message, attrs: map[string]any{}}
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

func (l *logCapture) find(msg string) []logRecord {
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []logRecord
	for _, r := range l.records {
		if r.msg == msg {
			out = append(out, r)
		}
	}
	return out
}

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

// provRequest is one request the fake provider received.
type provRequest struct {
	Path    string
	Key     string
	Body    map[string]string
	At      time.Time
	Attempt int // how many requests with this key came before, plus one
}

// provResponse is what the fake provider answers; nil Respond means 202 with a message id that is
// the same for every request with the same Idempotency-Key, like the real contract.
type provResponse struct {
	Status int
	Header http.Header
	Body   string
}

type fakeProvider struct {
	srv *httptest.Server

	mu       sync.Mutex
	reqs     []provRequest
	attempts map[string]int
	// respond decides the answer to a request; nil answers every request with 202.
	respond func(provRequest) provResponse
}

func newFakeProvider(t *testing.T) *fakeProvider {
	t.Helper()
	p := &fakeProvider{attempts: map[string]int{}}
	p.srv = httptest.NewServer(http.HandlerFunc(p.handle))
	t.Cleanup(p.srv.Close)
	return p
}

func (p *fakeProvider) handle(w http.ResponseWriter, r *http.Request) {
	var body map[string]string
	_ = json.NewDecoder(r.Body).Decode(&body)
	key := r.Header.Get("Idempotency-Key")
	p.mu.Lock()
	p.attempts[key]++
	req := provRequest{Path: r.URL.Path, Key: key, Body: body, At: time.Now(), Attempt: p.attempts[key]}
	p.reqs = append(p.reqs, req)
	respond := p.respond
	p.mu.Unlock()

	resp := provResponse{Status: http.StatusAccepted, Body: fmt.Sprintf(`{"message_id":"prov-%s"}`, key)}
	if respond != nil {
		resp = respond(req)
	}
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.Status)
	_, _ = w.Write([]byte(resp.Body))
}

func (p *fakeProvider) setRespond(f func(provRequest) provResponse) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.respond = f
}

func (p *fakeProvider) requests() []provRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]provRequest(nil), p.reqs...)
}

func (p *fakeProvider) count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.reqs)
}

// byKey groups the requests by Idempotency-Key.
func (p *fakeProvider) byKey() map[string][]provRequest {
	out := map[string][]provRequest{}
	for _, r := range p.requests() {
		out[r.Key] = append(out[r.Key], r)
	}
	return out
}

// customerStub serves the real Customer GraphQL handler over the seeded database, behind the real
// bearer-token middleware, and can be switched to answer 503.
type customerStub struct {
	srv   *httptest.Server
	down  atomic.Bool
	calls atomic.Int32
}

func newCustomerStub(t *testing.T) *customerStub {
	t.Helper()
	cities, err := audience.LoadCities(t.Context(), customerEnv.Pool)
	if err != nil {
		t.Fatalf("LoadCities: %v", err)
	}
	svc := audience.NewService(customerEnv.Pool, cities)
	h := graph.NewHandler(graph.NewResolver(svc, custconsent.NewService(customerEnv.Pool), slog.New(slog.NewTextHandler(io.Discard, nil))))
	tokens, err := auth.NewTokenSet(map[string]string{
		auth.CallerProposal: proposalToken, auth.CallerNotification: notificationToken, auth.CallerAdmin: adminToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	real := tokens.Middleware(h)
	s := &customerStub{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.calls.Add(1)
		if s.down.Load() {
			http.Error(w, "customer is down", http.StatusServiceUnavailable)
			return
		}
		real.ServeHTTP(w, r)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

// envOptions are what a test changes about the service it runs.
type envOptions struct {
	deferSender bool
	backoff     consumer.Backoff
	workers     int
	lease       time.Duration
	breaker     nconsent.Config
}

type envOption func(*envOptions)

func withDeferredSender() envOption            { return func(o *envOptions) { o.deferSender = true } }
func withBackoff(b consumer.Backoff) envOption { return func(o *envOptions) { o.backoff = b } }
func withWorkers(n int) envOption              { return func(o *envOptions) { o.workers = n } }
func withBreaker(maxFailures uint32, openFor time.Duration) envOption {
	return func(o *envOptions) { o.breaker.MaxFailures, o.breaker.OpenFor = maxFailures, openFor }
}

var fastBackoff = consumer.Backoff{Base: 20 * time.Millisecond, Factor: 1.5, Max: 150 * time.Millisecond}

// env is one test's database (platform and notification migrations), streams, relay, intake
// consumers, fake provider, real Customer and the send workers.
type env struct {
	t        *testing.T
	pool     *pgxpool.Pool
	js       jetstream.JetStream
	streams  map[string]jetstream.Stream
	logs     *logCapture
	q        *store.Queries
	prov     *fakeProvider
	customer *customerStub
	consent  *nconsent.Client
	reg      *prometheus.Registry

	opts     envOptions
	logger   *slog.Logger
	stopSend func(context.Context) error
}

func newEnv(t *testing.T, opts ...envOption) *env {
	t.Helper()
	ctx := t.Context()
	o := envOptions{backoff: fastBackoff, breaker: nconsent.Config{MaxFailures: 5, OpenFor: 400 * time.Millisecond}}
	for _, f := range opts {
		f(&o)
	}

	dbSeqMu.Lock()
	dbSeq++
	name := fmt.Sprintf("sender_test_%d", dbSeq)
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

	e := &env{
		t: t, pool: pool, js: js, streams: streams, logs: logs, q: store.New(pool),
		prov: newFakeProvider(t), customer: newCustomerStub(t), reg: prometheus.NewRegistry(),
		opts: o, logger: logger,
	}

	o.breaker.URL, o.breaker.Token, o.breaker.Logger = e.customer.srv.URL, notificationToken, logger
	e.consent, err = nconsent.New(o.breaker)
	if err != nil {
		t.Fatal(err)
	}

	relay := &outbox.Relay{Pool: pool, JS: js, Logger: quiet, PollInterval: 50 * time.Millisecond}
	stopRelay := relay.Start(ctx)
	stopIntake, err := intake.Start(ctx, intake.Deps{JS: js, Pool: pool, Logger: logger, Backoff: fastBackoff})
	if err != nil {
		t.Fatalf("intake.Start() error = %v", err)
	}
	t.Cleanup(func() {
		// Send workers first, then the intake consumers, then the relay, as boot.Setup does.
		cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		if e.stopSend != nil {
			if err := e.stopSend(cctx); err != nil {
				t.Errorf("stop send workers: %v", err)
			}
		}
		if err := stopIntake(cctx); err != nil {
			t.Errorf("stop intake consumers: %v", err)
		}
		if err := stopRelay(cctx); err != nil {
			t.Errorf("stop relay: %v", err)
		}
	})
	if !o.deferSender {
		e.startSender()
	}
	return e
}

// startSender starts the send workers, with the real provider client and the real consent client.
func (e *env) startSender() {
	e.t.Helper()
	pc, err := provider.New(provider.Config{BaseURL: e.prov.srv.URL})
	if err != nil {
		e.t.Fatal(err)
	}
	stop, err := sender.Start(e.t.Context(), sender.Deps{
		JS: e.js, Pool: e.pool, Logger: e.logger, Provider: pc, Consent: e.consent, Registerer: e.reg,
		Backoff: e.opts.backoff, Workers: e.opts.workers, Lease: e.opts.lease,
	})
	if err != nil {
		e.t.Fatalf("sender.Start() error = %v", err)
	}
	e.stopSend = stop
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

func (e *env) count(query string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.pool.QueryRow(e.t.Context(), query, args...).Scan(&n); err != nil {
		e.t.Fatalf("query %q: %v", query, err)
	}
	return n
}

// statusCounts returns the number of deliveries per status of a campaign.
func (e *env) statusCounts(campaignID string) map[string]int {
	e.t.Helper()
	rows, err := e.pool.Query(e.t.Context(), `SELECT status, count(*) FROM delivery WHERE campaign_id = $1::uuid GROUP BY status`, campaignID)
	if err != nil {
		e.t.Fatalf("status counts: %v", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			e.t.Fatal(err)
		}
		out[s] = n
	}
	return out
}

// waitStatuses waits until the campaign's deliveries have exactly the given count per status.
func (e *env) waitStatuses(campaignID string, timeout time.Duration, want map[string]int) {
	e.t.Helper()
	eventually(e.t, timeout, fmt.Sprintf("deliveries of %s to reach %v (now %v)", campaignID, want, e.statusCounts(campaignID)), func() bool {
		got := e.statusCounts(campaignID)
		if len(got) != len(want) {
			return false
		}
		for s, n := range want {
			if got[s] != n {
				return false
			}
		}
		return true
	})
}

// deliveryRow is a delivery as the tests look at it.
type deliveryRow struct {
	ID                string
	Status            string
	Attempts          int
	LeaseUntil        *time.Time
	ProviderMessageID *string
	LastError         *string
	SentAt            *time.Time
	FirstName         *string
	Contact           *string
}

func (e *env) delivery(campaignID, customerID, channel string) deliveryRow {
	e.t.Helper()
	var d deliveryRow
	err := e.pool.QueryRow(e.t.Context(), `SELECT id::text, status, attempts, lease_until, provider_message_id, last_error, sent_at, first_name, contact
		FROM delivery WHERE campaign_id = $1::uuid AND customer_id = $2 AND channel = $3`, campaignID, customerID, channel).
		Scan(&d.ID, &d.Status, &d.Attempts, &d.LeaseUntil, &d.ProviderMessageID, &d.LastError, &d.SentAt, &d.FirstName, &d.Contact)
	if err != nil {
		e.t.Fatalf("delivery %s/%s: %v", customerID, channel, err)
	}
	return d
}

// sendIdle waits until the two SEND consumers have nothing pending and nothing waiting for a retry.
func (e *env) sendIdle() {
	e.t.Helper()
	eventually(e.t, 20*time.Second, "SEND consumers to be idle", func() bool {
		for _, ch := range []string{"sms", "email"} {
			c, err := e.js.Consumer(e.t.Context(), sender.Stream, sender.ConsumerPrefix+ch)
			if err != nil {
				return false
			}
			info, err := c.Info(e.t.Context())
			if err != nil || info.NumPending != 0 || info.NumAckPending != 0 {
				return false
			}
		}
		return true
	})
}

// sendQueued returns how many messages the SEND stream holds.
func (e *env) sendQueued() uint64 {
	e.t.Helper()
	info, err := e.streams["SEND"].Info(e.t.Context())
	if err != nil {
		e.t.Fatalf("stream info: %v", err)
	}
	return info.State.Msgs
}

func (e *env) publishEvent(typ, campaignID string, data any) {
	e.t.Helper()
	env, err := events.New(typ, flow.EventVersion, flow.EventProducer, campaignID, data)
	if err != nil {
		e.t.Fatalf("events.New() error = %v", err)
	}
	env.ID = uuid.Must(uuid.NewV7()).String()
	subject, err := events.Subject(env)
	if err != nil {
		e.t.Fatalf("events.Subject() error = %v", err)
	}
	raw, err := json.Marshal(env)
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := e.js.Publish(e.t.Context(), subject, raw, jetstream.WithMsgID(env.ID)); err != nil {
		e.t.Fatalf("publish %s: %v", subject, err)
	}
}

func (e *env) publishBatch(b flow.RecipientBatch) {
	e.t.Helper()
	raw, err := json.Marshal(b)
	if err != nil {
		e.t.Fatal(err)
	}
	if _, err := e.js.Publish(e.t.Context(), flow.RecipientsSubject(b.CampaignID), raw); err != nil {
		e.t.Fatalf("publish batch: %v", err)
	}
}

const (
	smsTemplate = "Merhaba {{first_name}}, sepetindeki ürünler seni bekliyor. %30 indirim için acele et!"
)

var emailBlocks = content.EmailContent{
	Subject:    "Sepetin seni bekliyor, {{first_name}}",
	Preheader:  "%30 indirim seni bekliyor",
	Headline:   "Merhaba {{first_name}}",
	Paragraphs: []string{"Sepetindeki ürünler seni bekliyor.", "Bugüne özel %30 indirim."},
	CTALabel:   "Alışverişe dön",
}

func newCampaignID() string { return uuid.Must(uuid.NewV7()).String() }

// campaignOf publishes the confirmed event (one immediate Turkish schedule, an sms and an email
// message) and the recipient batches of the sms and email recipients: the same messages Proposal
// publishes when the marketer confirms. The schedule fires as soon as the batches are in, and the
// deliveries are queued on SEND.
func (e *env) campaignOf(id string, sms, email []cust) {
	e.t.Helper()
	smsContent, err := json.Marshal(content.SMSContent{Text: smsTemplate})
	if err != nil {
		e.t.Fatal(err)
	}
	emailContent, err := json.Marshal(emailBlocks)
	if err != nil {
		e.t.Fatal(err)
	}
	var all []flow.BatchRecipient
	for _, c := range sms {
		all = append(all, flow.BatchRecipient{CustomerID: c.ID, FirstName: c.FirstName, Language: "tr", Timezone: istanbul, Channel: "sms", Contact: c.Contact})
	}
	for _, c := range email {
		all = append(all, flow.BatchRecipient{CustomerID: c.ID, FirstName: c.FirstName, Language: "tr", Timezone: istanbul, Channel: "email", Contact: c.Contact})
	}
	var batches [][]flow.BatchRecipient
	for len(all) > 0 {
		n := min(len(all), flow.MaxBatchRecipients)
		batches = append(batches, all[:n])
		all = all[n:]
	}
	e.publishEvent(flow.EventTypeConfirmed, id, flow.ConfirmedData{
		CampaignID: id,
		Schedules:  []flow.ConfirmedSchedule{{Language: "tr", Timezone: istanbul}},
		Messages: []flow.ConfirmedMessage{
			{Language: "tr", Channel: "sms", Content: smsContent},
			{Language: "tr", Channel: "email", Content: emailContent},
		},
		CTAURL:           "https://shop.example.com/cart?utm_campaign=" + id,
		RecipientBatches: len(batches),
	})
	for i, b := range batches {
		e.publishBatch(flow.RecipientBatch{CampaignID: id, BatchNo: i + 1, BatchCount: len(batches), Recipients: b})
	}
}

// waitFired waits until the schedule has fired and the campaign has the given number of deliveries.
// While the send workers are deferred it also waits until the relay has put every delivery on SEND,
// where it stays until startSender.
func (e *env) waitFired(id string, deliveries int) {
	e.t.Helper()
	eventually(e.t, 20*time.Second, "the schedule to fire", func() bool {
		return e.count(`SELECT count(*) FROM locale_schedule WHERE campaign_id = $1::uuid AND status = 'fired'`, id) == 1 &&
			e.count(`SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid`, id) == deliveries
	})
	if e.opts.deferSender && e.stopSend == nil {
		eventually(e.t, 20*time.Second, "the deliveries to be queued on SEND", func() bool { return e.sendQueued() == uint64(deliveries) })
	}
}

// scalar reads one nullable text value.
func (e *env) scalar(query string, args ...any) string {
	e.t.Helper()
	var s *string
	err := e.pool.QueryRow(e.t.Context(), query, args...).Scan(&s)
	if errors.Is(err, pgx.ErrNoRows) {
		return "<no row>"
	}
	if err != nil {
		e.t.Fatalf("query %q: %v", query, err)
	}
	if s == nil {
		return "<null>"
	}
	return *s
}

// metric reads the value of a counter series of the send worker's registry.
func (e *env) metric(name string, labels map[string]string) float64 {
	e.t.Helper()
	families, err := e.reg.Gather()
	if err != nil {
		e.t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
	metrics:
		for _, m := range f.GetMetric() {
			got := map[string]string{}
			for _, l := range m.GetLabel() {
				got[l.GetName()] = l.GetValue()
			}
			for k, v := range labels {
				if got[k] != v {
					continue metrics
				}
			}
			if m.GetCounter() != nil {
				return m.GetCounter().GetValue()
			}
			if m.GetHistogram() != nil {
				return float64(m.GetHistogram().GetSampleCount())
			}
		}
	}
	return 0
}
