package metrics_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/furknozg/coldbru/internal/platform/metrics"
)

func newRouter(m *metrics.Metrics) chi.Router {
	r := chi.NewRouter()
	r.Use(m.Middleware())
	r.Get("/items/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	return r
}

func scrape(t *testing.T, m *metrics.Metrics) string {
	t.Helper()
	rec := httptest.NewRecorder()
	m.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("scrape code = %d", rec.Code)
	}
	return rec.Body.String()
}

func TestMiddlewareUsesRoutePattern(t *testing.T) {
	t.Parallel()
	m := metrics.New()
	r := newRouter(m)

	for _, id := range []string{"1", "2", "3"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/items/"+id, nil))
	}

	want := `
# HELP http_requests_total Total HTTP requests by method, route pattern and status.
# TYPE http_requests_total counter
http_requests_total{method="GET",route="/items/{id}",status="418"} 3
`
	if err := testutil.GatherAndCompare(m.Registry(), strings.NewReader(want), "http_requests_total"); err != nil {
		t.Error(err)
	}
}

func TestMiddlewareUnmatchedRoute(t *testing.T) {
	t.Parallel()
	m := metrics.New()
	newRouter(m).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/nope", nil))

	if got := testutil.CollectAndCount(m.Registry(), "http_requests_total"); got != 1 {
		t.Fatalf("series = %d, want 1", got)
	}
	if body := scrape(t, m); !strings.Contains(body, `route="unmatched"`) {
		t.Errorf("scrape lacks unmatched route label:\n%s", body)
	}
}

func TestHandlerExposesRuntimeAndDurationMetrics(t *testing.T) {
	t.Parallel()
	m := metrics.New()
	newRouter(m).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/items/9", nil))

	body := scrape(t, m)
	for _, name := range []string{"http_request_duration_seconds_bucket", "go_goroutines", "process_start_time_seconds"} {
		if !strings.Contains(body, name) {
			t.Errorf("scrape lacks %s", name)
		}
	}
}
