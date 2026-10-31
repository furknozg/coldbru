//go:build integration

package sender_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/furknozg/coldbru/internal/mockprovider"
	"github.com/furknozg/coldbru/internal/notification/provider"
	"github.com/furknozg/coldbru/internal/notification/sender"
)

// The fault rates of the real mock provider in this test (D-103), against the defaults of 0.05,
// 0.02 and 0.01. Faults count against a delivery's five tries (sender.MaxDeliver): it ends `failed`
// only if five attempts in a row hit one, which is 0.16^5 = 1e-4 per delivery here. At 0.2 and 0.2
// it would be 1% per delivery, and with 50 deliveries the test would fail about four runs in ten
// for a reason that is the design, not a bug.
const (
	fault5xx     = 0.10
	fault429     = 0.04
	faultTimeout = 0.02
	// The held answer of a timeout fault outlasts the provider client's limit, so the client gives
	// up on a message the provider has accepted and stored; its retry must get the stored answer.
	faultHold     = 1500 * time.Millisecond
	clientTimeout = 750 * time.Millisecond

	// mockSeed pins the random choices of the mock. It was picked from a scan of seeds for one whose
	// first 50 deliveries meet 502/503, 429 and timeout faults, with no delivery hit twice in a row.
	mockSeed = 50
)

// inboxRecorder is the mock provider's inbox: what was forwarded, by Idempotency-Key.
type inboxRecorder struct {
	mu       sync.Mutex
	forwards map[string]int
}

func (b *inboxRecorder) SendEmail(_ context.Context, e mockprovider.Email) error {
	b.add(e.IdempotencyKey)
	return nil
}

func (b *inboxRecorder) SendSMS(_ context.Context, s mockprovider.SMS) error {
	b.add(s.IdempotencyKey)
	return nil
}

func (b *inboxRecorder) add(key string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.forwards[key]++
}

// statusWriter keeps the status the handler wrote.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// TestSend_AgainstRealMockProviderSendsEachDeliveryOnce runs the send workers against the real
// mockprovider handler with a pinned seed and fault rates well above its defaults. It asserts the
// invariant and not the number of attempts: every delivery ends sent, the provider accepted (forwarded)
// at most one message per Idempotency-Key, and the retry path was used (D-103).
func TestSend_AgainstRealMockProviderSendsEachDeliveryOnce(t *testing.T) {
	const perChannel = 25
	e := newEnv(t, withDeferredSender())

	in := &inboxRecorder{forwards: map[string]int{}}
	cfg := mockprovider.DefaultConfig()
	cfg.ErrorRate, cfg.Rate429, cfg.TimeoutRate, cfg.TimeoutDelay = fault5xx, fault429, faultTimeout, faultHold
	cfg.LatencyMin, cfg.LatencyMax = 0, 20*time.Millisecond
	cfg.RateLimit = 1000 // the rate limit is tested elsewhere; here it would only add 429s
	cfg.Seed, cfg.HasSeed = mockSeed, true
	h, err := mockprovider.NewHandler(mockprovider.HandlerConfig{Config: cfg, Forwarder: in, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	h.Mount(r)

	// Count what the provider answered, per Idempotency-Key.
	var (
		mu       sync.Mutex
		requests = map[string]int{}
		statuses = map[int]int{}
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		r.ServeHTTP(sw, req)
		mu.Lock()
		requests[req.Header.Get("Idempotency-Key")]++
		statuses[sw.status]++
		mu.Unlock()
	}))
	t.Cleanup(srv.Close)

	pc, err := provider.New(provider.Config{BaseURL: srv.URL, Timeout: clientTimeout})
	if err != nil {
		t.Fatal(err)
	}
	stop, err := sender.Start(t.Context(), sender.Deps{
		JS: e.js, Pool: e.pool, Logger: e.logger, Provider: pc, Consent: e.consent, Registerer: e.reg, Backoff: e.opts.backoff,
	})
	if err != nil {
		t.Fatalf("sender.Start() error = %v", err)
	}
	e.stopSend = stop

	id := newCampaignID()
	e.campaignOf(id, smsCustomers[:perChannel], emailCustomers[:perChannel])
	const total = 2 * perChannel
	e.waitStatuses(id, 2*time.Minute, map[string]int{"sent": total})

	// One delivery per (customer, channel), each sent with the provider's message id.
	if n := e.count(`SELECT count(*) FROM (SELECT 1 FROM delivery WHERE campaign_id = $1::uuid GROUP BY customer_id, channel) x`, id); n != total {
		t.Errorf("%d (customer, channel) pairs, want %d", n, total)
	}
	if n := e.count(`SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid AND status = 'sent' AND provider_message_id IS NOT NULL AND sent_at IS NOT NULL`, id); n != total {
		t.Errorf("%d deliveries sent with a message id and a time, want %d", n, total)
	}

	// The provider accepted exactly one message per delivery, and none for any other key.
	rows, err := e.pool.Query(t.Context(), `SELECT id::text FROM delivery WHERE campaign_id = $1::uuid`, id)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			t.Fatal(err)
		}
		ids[d] = true
	}
	rows.Close()
	in.mu.Lock()
	defer in.mu.Unlock()
	for d := range ids {
		if in.forwards[d] != 1 {
			t.Errorf("the provider accepted %d messages for delivery %s, want exactly 1", in.forwards[d], d)
		}
	}
	for key, n := range in.forwards {
		if !ids[key] || n > 1 {
			t.Errorf("the provider accepted %d messages for key %q (a delivery: %v), want at most 1", n, key, ids[key])
		}
	}

	// The retry path ran: the provider answered some requests with a fault, some deliveries needed
	// more than one attempt, and some keys were sent more than once.
	mu.Lock()
	defer mu.Unlock()
	faults, retriedKeys := 0, 0
	for status, n := range statuses {
		if status != http.StatusAccepted {
			faults += n
		}
	}
	for _, n := range requests {
		if n > 1 {
			retriedKeys++
		}
	}
	retried := e.count(`SELECT count(*) FROM delivery WHERE campaign_id = $1::uuid AND attempts > 1`, id)
	t.Logf("provider answers by status %v; %d of %d keys were requested more than once; %d deliveries with attempts > 1", statuses, retriedKeys, total, retried)
	if faults == 0 || retriedKeys == 0 || retried == 0 || statuses[http.StatusTooManyRequests] == 0 {
		t.Errorf("the retry path was not exercised (statuses %v): %d fault answers, %d retried keys, %d deliveries with attempts > 1; pick another mockSeed if the mock's random sequence changed",
			statuses, faults, retriedKeys, retried)
	}
}
