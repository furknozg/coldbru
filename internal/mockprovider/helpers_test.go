package mockprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// fakeForwarder records what reached the "inbox".
type fakeForwarder struct {
	mu     sync.Mutex
	emails []Email
	sms    []SMS
	fail   error         // returned instead of delivering when set
	delay  time.Duration // slows every forward down, to overlap concurrent requests
}

func (f *fakeForwarder) SendEmail(_ context.Context, e Email) error {
	time.Sleep(f.delay)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return f.fail
	}
	f.emails = append(f.emails, e)
	return nil
}

func (f *fakeForwarder) SendSMS(_ context.Context, s SMS) error {
	time.Sleep(f.delay)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return f.fail
	}
	f.sms = append(f.sms, s)
	return nil
}

func (f *fakeForwarder) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.emails) + len(f.sms)
}

func (f *fakeForwarder) setFail(err error) {
	f.mu.Lock()
	f.fail = err
	f.mu.Unlock()
}

// clock is a fake clock.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *clock { return &clock{t: time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)} }

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// quietConfig has no faults, no latency and a limit that is out of the way.
func quietConfig() Config {
	c := DefaultConfig()
	c.ErrorRate, c.Rate429, c.TimeoutRate = 0, 0, 0
	c.LatencyMin, c.LatencyMax = 0, 0
	c.RateLimit, c.Burst = 1e9, 1_000_000
	return c
}

type rig struct {
	h      *Handler
	srv    http.Handler
	fwd    *fakeForwarder
	clk    *clock
	reg    *prometheus.Registry
	sleeps []time.Duration
	sleepM sync.Mutex
	// onSleep runs at each Sleep, with the wait asked for.
	onSleep func(d time.Duration)
}

func newRig(t *testing.T, cfg Config) *rig {
	t.Helper()
	rg := &rig{fwd: &fakeForwarder{}, clk: newClock(), reg: prometheus.NewRegistry()}
	h, err := NewHandler(HandlerConfig{
		Config: cfg, Forwarder: rg.fwd, Registerer: rg.reg, Now: rg.clk.now,
		Sleep: func(ctx context.Context, d time.Duration) error {
			rg.sleepM.Lock()
			rg.sleeps = append(rg.sleeps, d)
			rg.sleepM.Unlock()
			if rg.onSleep != nil {
				rg.onSleep(d)
			}
			return ctx.Err()
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	h.Mount(r)
	rg.h, rg.srv = h, r
	return rg
}

func (rg *rig) do(t testing.TB, path, key string, body any) *httptest.ResponseRecorder {
	t.Helper()
	return rg.doCtx(t, context.Background(), path, key, body)
}

func (rg *rig) doCtx(t testing.TB, ctx context.Context, path, key string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	switch b := body.(type) {
	case string:
		raw = []byte(b)
	default:
		var err error
		if raw, err = json.Marshal(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	rg.srv.ServeHTTP(rec, req)
	return rec
}

func (rg *rig) metric(channel, outcome string) float64 {
	return testutil.ToFloat64(rg.h.requests.WithLabelValues(channel, outcome))
}

func goodSMS() SMSRequest { return SMSRequest{To: "+905551112233", Text: "Merhaba Ayşe"} }

func goodEmail() EmailRequest {
	return EmailRequest{To: "ayse@example.com", Subject: "Sepetin seni bekliyor", HTML: "<p>Merhaba Ayşe</p>", Text: "Merhaba Ayşe"}
}

func messageID(t testing.TB, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var a Accepted
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil || a.MessageID == "" {
		t.Fatalf("body %q has no message_id (%v)", rec.Body.String(), err)
	}
	return a.MessageID
}

func problemOf(t testing.TB, rec *httptest.ResponseRecorder) Problem {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != ProblemContentType {
		t.Fatalf("Content-Type = %q, want %q (body %q)", ct, ProblemContentType, rec.Body.String())
	}
	var p Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("problem body %q: %v", rec.Body.String(), err)
	}
	if p.Status != rec.Code {
		t.Errorf("problem status %d, response status %d", p.Status, rec.Code)
	}
	return p
}

var errInbox = errors.New("inbox down")
