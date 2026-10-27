package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/furknozg/coldbru/internal/platform/health"
	"github.com/furknozg/coldbru/internal/platform/metrics"
)

func newTestRouter(t *testing.T, logOut io.Writer, reg *health.Registry) http.Handler {
	t.Helper()
	logger := slog.New(slog.NewJSONHandler(logOut, nil))
	return NewRouter(logger, metrics.New(), reg)
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestRouterEndpoints(t *testing.T) {
	t.Parallel()
	reg := health.NewRegistry()
	reg.Add("db", func(context.Context) error { return errors.New("db down") })
	h := newTestRouter(t, io.Discard, reg)

	tests := []struct {
		name string
		path string
		want int
	}{
		{name: "liveness", path: "/healthz", want: http.StatusOK},
		{name: "readiness with failing check", path: "/readyz", want: http.StatusServiceUnavailable},
		{name: "metrics", path: "/metrics", want: http.StatusOK},
		{name: "unknown route", path: "/nope", want: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := get(h, tt.path).Code; got != tt.want {
				t.Errorf("GET %s = %d, want %d", tt.path, got, tt.want)
			}
		})
	}
}

func TestRouterMetricsCountRequests(t *testing.T) {
	t.Parallel()
	h := newTestRouter(t, io.Discard, health.NewRegistry())
	get(h, "/healthz")

	body := get(h, "/metrics").Body.String()
	if !strings.Contains(body, "http_requests_total") {
		t.Fatalf("metrics lacks http_requests_total:\n%s", body)
	}
	if !strings.Contains(body, `route="/healthz"`) {
		t.Errorf("metrics lacks route=/healthz label")
	}
}

func TestRequestLoggerEmitsJSONLine(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	h := newTestRouter(t, &buf, health.NewRegistry())
	get(h, "/healthz")

	var rec map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec); err != nil {
		t.Fatalf("log is not one JSON line: %v (%q)", err, buf.String())
	}
	if rec["msg"] != "http request" || rec["level"] != "INFO" {
		t.Errorf("msg/level = %v/%v", rec["msg"], rec["level"])
	}
	if rec["method"] != "GET" || rec["path"] != "/healthz" || rec["status"] != float64(200) {
		t.Errorf("unexpected attrs: %v", rec)
	}
	if id, _ := rec["request_id"].(string); id == "" {
		t.Errorf("request_id missing: %v", rec)
	}
	if _, ok := rec["duration_ms"].(float64); !ok {
		t.Errorf("duration_ms missing: %v", rec)
	}
}

func TestRecovererReturns500AndIsObserved(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	r := NewRouter(slog.New(slog.NewJSONHandler(&buf, nil)), metrics.New(), health.NewRegistry())
	r.Get("/boom", func(http.ResponseWriter, *http.Request) { panic("boom") })

	if got := get(r, "/boom").Code; got != http.StatusInternalServerError {
		t.Fatalf("code = %d, want 500", got)
	}

	// Recoverer prints the panic stack to stderr, not to the logger, so the
	// buffer holds only the request log line.
	var rec map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec); err != nil {
		t.Fatalf("log is not one JSON line: %v (%q)", err, buf.String())
	}
	if rec["status"] != float64(500) {
		t.Errorf("logged status = %v, want 500", rec["status"])
	}
	if id, _ := rec["request_id"].(string); id == "" {
		t.Errorf("request_id missing: %v", rec)
	}

	body := get(r, "/metrics").Body.String()
	if !strings.Contains(body, "http_requests_total{") || !strings.Contains(body, `status="500"`) {
		t.Errorf("metrics lacks a status=500 http_requests_total sample:\n%s", body)
	}
}

func TestNewServer(t *testing.T) {
	t.Parallel()
	srv := NewServer(":0", http.NotFoundHandler())
	if srv.ReadHeaderTimeout != 5*time.Second || srv.IdleTimeout != 60*time.Second {
		t.Errorf("timeouts = %v / %v", srv.ReadHeaderTimeout, srv.IdleTimeout)
	}
	if srv.WriteTimeout != 0 {
		t.Errorf("WriteTimeout = %v, want 0 for SSE", srv.WriteTimeout)
	}
}

func TestServeReturnsNilAfterContextCancel(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := NewServer(ln.Addr().String(), NewRouter(slog.New(slog.NewJSONHandler(io.Discard, nil)), metrics.New(), health.NewRegistry()))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, srv, ln, time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil))) }()

	resp, err := http.Get("http://" + ln.Addr().String() + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("serve = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after cancel")
	}
}

func TestRunListenError(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	srv := NewServer(ln.Addr().String(), http.NotFoundHandler()) // address already in use
	err = Run(context.Background(), srv, time.Second, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err == nil {
		t.Fatal("Run on busy address: want error")
	}
}
