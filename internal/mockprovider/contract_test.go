package mockprovider_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/furknozg/coldbru/internal/mockprovider"
	"github.com/furknozg/coldbru/internal/notification/provider"
)

// inbox is a Forwarder that records and can fail.
type inbox struct {
	mu   sync.Mutex
	n    int
	fail error
}

func (i *inbox) take() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.fail != nil {
		return i.fail
	}
	i.n++
	return nil
}
func (i *inbox) SendEmail(context.Context, mockprovider.Email) error { return i.take() }
func (i *inbox) SendSMS(context.Context, mockprovider.SMS) error     { return i.take() }
func (i *inbox) count() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.n
}

// contractRig runs the mock behind a real HTTP server, and Notification's real client against it.
func contractRig(t *testing.T, mutate func(*mockprovider.Config), clientTimeout time.Duration) (*provider.Client, *inbox) {
	t.Helper()
	cfg := mockprovider.DefaultConfig()
	cfg.ErrorRate, cfg.Rate429, cfg.TimeoutRate = 0, 0, 0
	cfg.LatencyMin, cfg.LatencyMax = 0, 0
	cfg.RateLimit, cfg.Burst = 1e9, 1_000_000
	if mutate != nil {
		mutate(&cfg)
	}
	in := &inbox{}
	h, err := mockprovider.NewHandler(mockprovider.HandlerConfig{Config: cfg, Forwarder: in})
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	h.Mount(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	c, err := provider.New(provider.Config{BaseURL: srv.URL, Timeout: clientTimeout})
	if err != nil {
		t.Fatal(err)
	}
	return c, in
}

var (
	sms   = provider.SMS{To: "+905551112233", Text: "Merhaba Ayşe"}
	email = provider.Email{To: "ayse@example.com", Subject: "Sepetin seni bekliyor", HTML: "<p>Merhaba</p>", Text: "Merhaba"}
)

func TestContractSuccessAndReplay(t *testing.T) {
	t.Parallel()
	c, in := contractRig(t, nil, 0)
	for name, send := range map[string]func(key string) (provider.Result, error){
		"sms":   func(k string) (provider.Result, error) { return c.SendSMS(t.Context(), k, sms) },
		"email": func(k string) (provider.Result, error) { return c.SendEmail(t.Context(), k, email) },
	} {
		before := in.count()
		first, err := send("delivery-" + name)
		if err != nil || first.Status != 202 || first.MessageID == "" {
			t.Fatalf("%s: first = %+v, %v", name, first, err)
		}
		again, err := send("delivery-" + name)
		if err != nil || again.MessageID != first.MessageID {
			t.Errorf("%s: replay = %+v, %v, want message id %q", name, again, err, first.MessageID)
		}
		if got := in.count() - before; got != 1 {
			t.Errorf("%s: forwarded %d times, want 1", name, got)
		}
	}
}

func TestContractPermanentError(t *testing.T) {
	t.Parallel()
	c, in := contractRig(t, nil, 0)
	_, err := c.SendSMS(t.Context(), "k", provider.SMS{To: "not-a-number", Text: "x"})
	var perm *provider.PermanentError
	if !errors.As(err, &perm) || provider.Classify(err) != provider.ClassPermanent || perm.Status != 422 {
		t.Errorf("invalid phone: err = %v (class %v), want a permanent 422", err, provider.Classify(err))
	}
	_, err = c.SendEmail(t.Context(), "k", provider.Email{To: "nope", Subject: "s", HTML: "h", Text: "t"})
	if provider.Classify(err) != provider.ClassPermanent {
		t.Errorf("invalid email: err = %v, want permanent", err)
	}
	// The same key with another body is permanent as well.
	if _, err := c.SendSMS(t.Context(), "same", sms); err != nil {
		t.Fatal(err)
	}
	_, err = c.SendSMS(t.Context(), "same", provider.SMS{To: sms.To, Text: "başka"})
	if provider.Classify(err) != provider.ClassPermanent || provider.StatusOf(err) != 422 {
		t.Errorf("key reuse: err = %v, want permanent 422", err)
	}
	if in.count() != 1 {
		t.Errorf("forwarded %d, want 1", in.count())
	}
}

func TestContractRateLimited(t *testing.T) {
	t.Parallel()
	t.Run("bucket", func(t *testing.T) {
		t.Parallel()
		c, _ := contractRig(t, func(cfg *mockprovider.Config) { cfg.RateLimit, cfg.Burst = 1, 1 }, 0)
		if _, err := c.SendSMS(t.Context(), "a", sms); err != nil {
			t.Fatal(err)
		}
		_, err := c.SendSMS(t.Context(), "b", sms)
		var rl *provider.RateLimitedError
		if !errors.As(err, &rl) || provider.Classify(err) != provider.ClassRateLimited || rl.RetryAfter != time.Second {
			t.Errorf("err = %v, want RateLimitedError with Retry-After 1s", err)
		}
	})
	t.Run("spontaneous", func(t *testing.T) {
		t.Parallel()
		c, in := contractRig(t, func(cfg *mockprovider.Config) { cfg.Rate429 = 1 }, 0)
		for range 20 {
			_, err := c.SendEmail(t.Context(), "k", email)
			var rl *provider.RateLimitedError
			if !errors.As(err, &rl) || rl.RetryAfter < time.Second || rl.RetryAfter > 3*time.Second {
				t.Fatalf("err = %v, want RateLimitedError with Retry-After 1-3s", err)
			}
		}
		if in.count() != 0 {
			t.Errorf("forwarded %d, want 0", in.count())
		}
	})
}

func TestContractTransientErrors(t *testing.T) {
	t.Parallel()
	t.Run("injected 5xx", func(t *testing.T) {
		t.Parallel()
		c, _ := contractRig(t, func(cfg *mockprovider.Config) { cfg.ErrorRate = 1 }, 0)
		seen := map[int]bool{}
		for range 30 {
			_, err := c.SendSMS(t.Context(), "k", sms)
			var tr *provider.TransientError
			if !errors.As(err, &tr) || provider.Classify(err) != provider.ClassTransient {
				t.Fatalf("err = %v, want transient", err)
			}
			seen[tr.Status] = true
		}
		if !seen[502] || !seen[503] || len(seen) != 2 {
			t.Errorf("statuses seen: %v, want 502 and 503", seen)
		}
	})
	t.Run("inbox down", func(t *testing.T) {
		t.Parallel()
		c, in := contractRig(t, nil, 0)
		in.fail = errors.New("down")
		_, err := c.SendEmail(t.Context(), "k", email)
		if provider.Classify(err) != provider.ClassTransient || provider.StatusOf(err) != 503 {
			t.Errorf("err = %v, want transient 503", err)
		}
		in.fail = nil
		if _, err := c.SendEmail(t.Context(), "k", email); err != nil {
			t.Errorf("retry after the inbox came back: %v", err)
		}
	})
}

// A lost answer: the mock accepts and stores, the client's timeout fires, and the retry with the
// same key gets the stored 202 - the message is in the inbox once.
func TestContractTimeoutThenRetry(t *testing.T) {
	t.Parallel()
	c, in := contractRig(t, func(cfg *mockprovider.Config) {
		cfg.TimeoutRate, cfg.TimeoutDelay = 1, 2*time.Second
	}, 200*time.Millisecond)

	_, err := c.SendSMS(t.Context(), "lost-answer", sms)
	var tr *provider.TransientError
	if !errors.As(err, &tr) || provider.Classify(err) != provider.ClassTransient || tr.Status != 0 {
		t.Fatalf("first attempt: err = %v, want a transient error without a status (timeout)", err)
	}
	res, err := c.SendSMS(t.Context(), "lost-answer", sms)
	if err != nil || res.Status != 202 || res.MessageID == "" {
		t.Fatalf("retry: %+v, %v, want the stored 202", res, err)
	}
	if in.count() != 1 {
		t.Errorf("forwarded %d times, want 1", in.count())
	}
}

// Every class the client understands is produced by the mock.
func TestContractProducesEveryClass(t *testing.T) {
	t.Parallel()
	classes := map[provider.Class]bool{}
	for _, mutate := range []func(*mockprovider.Config){
		nil,
		func(c *mockprovider.Config) { c.ErrorRate = 1 },
		func(c *mockprovider.Config) { c.Rate429 = 1 },
	} {
		c, _ := contractRig(t, mutate, 0)
		_, err := c.SendSMS(t.Context(), "k", sms)
		classes[provider.Classify(err)] = true
	}
	c, _ := contractRig(t, nil, 0)
	_, err := c.SendSMS(t.Context(), "k", provider.SMS{To: "x", Text: "y"})
	classes[provider.Classify(err)] = true
	for _, want := range []provider.Class{provider.ClassOK, provider.ClassTransient, provider.ClassRateLimited, provider.ClassPermanent} {
		if !classes[want] {
			t.Errorf("class %v not produced", want)
		}
	}
}
