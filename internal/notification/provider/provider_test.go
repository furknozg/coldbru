package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func newClient(t *testing.T, h http.HandlerFunc, opts ...Option) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := New(Config{BaseURL: srv.URL}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSendSMSRequestAndSuccess(t *testing.T) {
	t.Parallel()
	var gotPath, gotKey, gotType string
	var gotBody SMS
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotKey, gotType = r.URL.Path, r.Header.Get("Idempotency-Key"), r.Header.Get("Content-Type")
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"message_id":"m-1"}`))
	})
	res, err := c.SendSMS(t.Context(), "key-1", SMS{To: "+905551112233", Text: "Merhaba"})
	if err != nil {
		t.Fatalf("SendSMS() error = %v", err)
	}
	if res.MessageID != "m-1" || res.Status != 202 {
		t.Errorf("result = %+v, want m-1 and 202", res)
	}
	if gotPath != "/v1/sms" || gotKey != "key-1" || gotType != "application/json" {
		t.Errorf("path = %q, key = %q, content type = %q", gotPath, gotKey, gotType)
	}
	if gotBody != (SMS{To: "+905551112233", Text: "Merhaba"}) {
		t.Errorf("body = %+v", gotBody)
	}
}

func TestSendEmailRequestAndSuccess(t *testing.T) {
	t.Parallel()
	var gotPath, gotKey string
	var gotBody map[string]string
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotKey = r.URL.Path, r.Header.Get("Idempotency-Key")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"message_id":"e-1"}`))
	})
	res, err := c.SendEmail(t.Context(), "key-2", Email{To: "a@example.com", Subject: "S", HTML: "<p>h</p>", Text: "t"})
	if err != nil {
		t.Fatalf("SendEmail() error = %v", err)
	}
	if res.MessageID != "e-1" {
		t.Errorf("message id = %q", res.MessageID)
	}
	want := map[string]string{"to": "a@example.com", "subject": "S", "html": "<p>h</p>", "text": "t"}
	if gotPath != "/v1/email" || gotKey != "key-2" || len(gotBody) != 4 {
		t.Errorf("path = %q, key = %q, body = %v", gotPath, gotKey, gotBody)
	}
	for k, v := range want {
		if gotBody[k] != v {
			t.Errorf("body[%s] = %q, want %q", k, gotBody[k], v)
		}
	}
}

func TestResponseClasses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		status     int
		body       string
		header     http.Header
		wantClass  Class
		wantStatus int
	}{
		{"202", 202, `{"message_id":"x"}`, nil, ClassOK, 202},
		{"429 with seconds", 429, `slow down`, http.Header{"Retry-After": {"2"}}, ClassRateLimited, 429},
		{"429 without header", 429, ``, nil, ClassRateLimited, 429},
		{"408 is transient", 408, ``, nil, ClassTransient, 408},
		{"500", 500, `boom`, nil, ClassTransient, 500},
		{"503", 503, ``, nil, ClassTransient, 503},
		{"504", 504, ``, nil, ClassTransient, 504},
		{"400 is permanent", 400, `{"error":"bad number"}`, nil, ClassPermanent, 400},
		{"401 is permanent", 401, ``, nil, ClassPermanent, 401},
		{"404 is permanent", 404, ``, nil, ClassPermanent, 404},
		{"422 is permanent", 422, ``, nil, ClassPermanent, 422},
		{"302 is permanent", 302, ``, http.Header{"Location": {"/elsewhere"}}, ClassPermanent, 302},
		{"200 without message_id is transient", 200, `{}`, nil, ClassTransient, 200},
		{"202 with garbage is transient", 202, `not json`, nil, ClassTransient, 202},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
				for k, v := range tt.header {
					w.Header()[k] = v
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})
			_, err := c.SendSMS(t.Context(), "k", SMS{To: "+90", Text: "t"})
			if got := Classify(err); got != tt.wantClass {
				t.Errorf("Classify() = %d, want %d (error %v)", got, tt.wantClass, err)
			}
			if got := StatusOf(err); err != nil && got != tt.wantStatus {
				t.Errorf("StatusOf() = %d, want %d", got, tt.wantStatus)
			}
		})
	}
}

func TestRateLimitedCarriesRetryAfter(t *testing.T) {
	t.Parallel()
	c := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	_, err := c.SendSMS(t.Context(), "k", SMS{})
	var rl *RateLimitedError
	if !errors.As(err, &rl) {
		t.Fatalf("error = %v, want *RateLimitedError", err)
	}
	if rl.RetryAfter != 3*time.Second {
		t.Errorf("RetryAfter = %s, want 3s", rl.RetryAfter)
	}
}

func TestNetworkErrorIsTransient(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // connection refused from now on
	c, err := New(Config{BaseURL: url})
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.SendSMS(t.Context(), "k", SMS{})
	if Classify(err) != ClassTransient {
		t.Fatalf("Classify() = %d for %v, want transient", Classify(err), err)
	}
	if StatusOf(err) != 0 || CodeClass(StatusOf(err)) != "error" {
		t.Errorf("status = %d, want 0 (no response)", StatusOf(err))
	}
}

func TestTimeoutIsTransient(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	t.Cleanup(func() { close(release); srv.Close() })
	c, err := New(Config{BaseURL: srv.URL, Timeout: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = c.SendSMS(t.Context(), "k", SMS{})
	if Classify(err) != ClassTransient {
		t.Fatalf("Classify() = %d for %v, want transient", Classify(err), err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("returned after %s, want about the 50ms timeout", elapsed)
	}
}

func TestCancelledContextIsTransient(t *testing.T) {
	t.Parallel()
	c := newClient(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusAccepted) })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := c.SendSMS(ctx, "k", SMS{})
	if Classify(err) != ClassTransient || !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want a transient error wrapping context.Canceled", err)
	}
}

func TestRedirectIsNotFollowed(t *testing.T) {
	t.Parallel()
	hits := 0
	c := newClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.URL.Path == "/v1/sms" {
			http.Redirect(w, r, "/elsewhere", http.StatusTemporaryRedirect)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"message_id":"x"}`))
	})
	_, err := c.SendSMS(t.Context(), "k", SMS{})
	if Classify(err) != ClassPermanent || hits != 1 {
		t.Errorf("class = %d, hits = %d, want permanent after one request", Classify(err), hits)
	}
}

func TestErrorBodyIsShortened(t *testing.T) {
	t.Parallel()
	c := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(strings.Repeat("x", 5000)))
	})
	_, err := c.SendSMS(t.Context(), "k", SMS{})
	if err == nil || len(err.Error()) > 400 {
		t.Errorf("error length = %d, want a short message", len(err.Error()))
	}
}

func TestParseRetryAfter(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		in   string
		want time.Duration
	}{
		{"empty", "", 0},
		{"seconds", "2", 2 * time.Second},
		{"zero", "0", 0},
		{"negative", "-5", 0},
		{"padded", "  7 ", 7 * time.Second},
		{"above the cap", "3600", MaxRetryAfter},
		{"http date in the future", now.Add(5 * time.Second).UTC().Format(http.TimeFormat), 5 * time.Second},
		{"http date in the past", now.Add(-time.Minute).UTC().Format(http.TimeFormat), 0},
		{"http date above the cap", now.Add(time.Hour).UTC().Format(http.TimeFormat), MaxRetryAfter},
		{"garbage", "soon", 0},
		{"fraction is not a number of seconds", "1.5", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ParseRetryAfter(tt.in, now); got != tt.want {
				t.Errorf("ParseRetryAfter(%q) = %s, want %s", tt.in, got, tt.want)
			}
		})
	}
}

func TestRetryAfterHTTPDateThroughClient(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", now.Add(4*time.Second).UTC().Format(http.TimeFormat))
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)
	c, err := New(Config{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	c.now = func() time.Time { return now }
	_, err = c.SendEmail(t.Context(), "k", Email{})
	var rl *RateLimitedError
	if !errors.As(err, &rl) || rl.RetryAfter != 4*time.Second {
		t.Errorf("error = %v, want 429 with RetryAfter 4s", err)
	}
}

func TestCodeClass(t *testing.T) {
	t.Parallel()
	for status, want := range map[int]string{202: "2xx", 400: "4xx", 408: "4xx", 429: "4xx", 500: "5xx", 503: "5xx", 302: "other", 0: "error"} {
		if got := CodeClass(status); got != want {
			t.Errorf("CodeClass(%d) = %q, want %q", status, got, want)
		}
	}
}

func TestClassifyUnknownErrorIsTransient(t *testing.T) {
	t.Parallel()
	if Classify(errors.New("who knows")) != ClassTransient || Classify(nil) != ClassOK {
		t.Error("an unknown error must be transient and nil must be OK")
	}
}

func TestNewRejectsBadBaseURL(t *testing.T) {
	t.Parallel()
	for _, u := range []string{"", "mockprovider:8080", "ftp://x", "http://"} {
		if _, err := New(Config{BaseURL: u}); err == nil {
			t.Errorf("New(%q) succeeded, want an error", u)
		}
	}
	if _, err := New(Config{BaseURL: "http://mockprovider:8080/"}); err != nil {
		t.Errorf("New with a trailing slash: %v", err)
	}
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()
	v := viper.New()
	if _, err := LoadConfig(v); err == nil || !strings.Contains(err.Error(), KeyBaseURL) {
		t.Errorf("LoadConfig() error = %v, want one naming %s", err, KeyBaseURL)
	}
	v.Set(KeyBaseURL, "http://mockprovider:8080")
	cfg, err := LoadConfig(v)
	if err != nil || cfg.BaseURL != "http://mockprovider:8080" {
		t.Errorf("LoadConfig() = %+v, %v", cfg, err)
	}
}
