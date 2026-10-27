package health_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/furknozg/coldbru/internal/platform/health"
)

type response struct {
	Status string `json:"status"`
	Checks map[string]struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	} `json:"checks"`
}

func probe(t *testing.T, h http.Handler) (*httptest.ResponseRecorder, response) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	var resp response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return rec, resp
}

func TestLivenessHandler(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	health.LivenessHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("code = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("content-type = %q", got)
	}
	if got := rec.Body.String(); got != "{\"status\":\"ok\"}\n" {
		t.Errorf("body = %q", got)
	}
}

func TestReadinessHandler(t *testing.T) {
	t.Parallel()
	okCheck := func(context.Context) error { return nil }
	failCheck := func(context.Context) error { return errors.New("db down") }
	blockCheck := func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }
	hang := make(chan struct{})
	t.Cleanup(func() { close(hang) })
	ignoreCtx := func(context.Context) error { <-hang; return nil }

	tests := []struct {
		name       string
		checks     map[string]health.Check
		wantCode   int
		wantStatus string
		wantFailed map[string]string // name -> expected error substring
	}{
		{name: "no checks", wantCode: 200, wantStatus: "ok"},
		{name: "all ok", checks: map[string]health.Check{"db": okCheck, "nats": okCheck}, wantCode: 200, wantStatus: "ok"},
		{
			name:       "one failing",
			checks:     map[string]health.Check{"db": failCheck, "nats": okCheck},
			wantCode:   503,
			wantStatus: "fail",
			wantFailed: map[string]string{"db": "db down"},
		},
		{
			name:       "context-aware check past timeout",
			checks:     map[string]health.Check{"slow": blockCheck},
			wantCode:   503,
			wantStatus: "fail",
			wantFailed: map[string]string{"slow": "deadline exceeded"},
		},
		{
			name:       "check ignoring context past timeout",
			checks:     map[string]health.Check{"stuck": ignoreCtx},
			wantCode:   503,
			wantStatus: "fail",
			wantFailed: map[string]string{"stuck": "deadline exceeded"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := health.NewRegistry()
			r.Timeout = 50 * time.Millisecond
			for n, c := range tt.checks {
				r.Add(n, c)
			}
			start := time.Now()
			rec, resp := probe(t, r.ReadinessHandler())
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Errorf("handler took %v, want prompt return", elapsed)
			}
			if rec.Code != tt.wantCode {
				t.Errorf("code = %d, want %d", rec.Code, tt.wantCode)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("content-type = %q", got)
			}
			if resp.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", resp.Status, tt.wantStatus)
			}
			for name := range tt.checks {
				got, ok := resp.Checks[name]
				if !ok {
					t.Errorf("check %q missing from response", name)
					continue
				}
				if want, failed := tt.wantFailed[name]; failed {
					if got.Status != "fail" || !strings.Contains(got.Error, want) {
						t.Errorf("check %q = %+v, want fail containing %q", name, got, want)
					}
				} else if got.Status != "ok" {
					t.Errorf("check %q = %+v, want ok", name, got)
				}
			}
		})
	}
}
