package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/furknozg/coldbru/internal/redirect/code"
)

const (
	testKey  = "unit-test-signing-key-123456"
	fallback = "https://shop.example.com/default"
)

// unreachablePool is a pool whose database does not exist: nothing connects until a query runs, and
// a query fails fast. A handler test that never reaches the database proves it by using this pool.
func unreachablePool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	p, err := pgxpool.New(t.Context(), "postgres://nobody@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

func newHandler(t *testing.T, reg prometheus.Registerer) (*Handler, *code.Signer) {
	t.Helper()
	signer, err := code.NewSigner(testKey)
	if err != nil {
		t.Fatal(err)
	}
	h, err := New(Config{
		Pool: unreachablePool(t), Signer: signer, FallbackURL: fallback, Registerer: reg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return h, signer
}

func serve(t *testing.T, h *Handler) *httptest.Server {
	t.Helper()
	r := chi.NewRouter()
	h.Mount(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

// noFollow is an HTTP client that returns redirects instead of following them.
var noFollow = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func TestBadCodesGoToTheFallbackWithoutTouchingTheDatabase(t *testing.T) {
	reg := prometheus.NewRegistry()
	h, signer := newHandler(t, reg)
	srv := serve(t, h)
	good, err := signer.New()
	if err != nil {
		t.Fatal(err)
	}
	foreign, _ := mustSigner(t, "another-signing-key-1234567").New()
	tampered := good[:code.IDLen] + strings.Repeat("0", code.TagLen)
	if tampered == good {
		tampered = good[:code.IDLen] + strings.Repeat("1", code.TagLen)
	}
	for name, c := range map[string]string{
		"wrong tag": tampered, "other key": foreign, "too short": good[:5], "too long": good + "x", "not base62": "abc-def_gh", "letters only": "AAAAAAAAAA",
	} {
		t.Run(name, func(t *testing.T) {
			resp, err := noFollow.Get(srv.URL + "/r/" + c)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != fallback {
				t.Errorf("GET /r/%s = %d, Location %q; want 302 to %s", c, resp.StatusCode, resp.Header.Get("Location"), fallback)
			}
			if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store: a cached redirect would hide clicks", cc)
			}
		})
	}
	// "AAAAAAAAAA" can be a valid code by chance (1 in 14 million); the others never are.
	if got := testutil.ToFloat64(h.clicks.WithLabelValues(resultInvalid)); got < 5 {
		t.Errorf("invalid_code count = %v, want at least 5", got)
	}
	if got := testutil.ToFloat64(h.clicks.WithLabelValues(resultFirst)) + testutil.ToFloat64(h.clicks.WithLabelValues(resultRepeat)); got != 0 {
		t.Errorf("%v clicks counted for bad codes", got)
	}
}

func mustSigner(t *testing.T, key string) *code.Signer {
	t.Helper()
	s, err := code.NewSigner(key)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDatabaseFailureStillRedirectsToTheFallback(t *testing.T) {
	h, signer := newHandler(t, nil)
	srv := serve(t, h)
	c, _ := signer.New()
	resp, err := noFollow.Get(srv.URL + "/r/" + c)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != fallback {
		t.Errorf("status %d, Location %q; want 302 to the fallback", resp.StatusCode, resp.Header.Get("Location"))
	}
	if got := testutil.ToFloat64(h.clicks.WithLabelValues(resultError)); got != 1 {
		t.Errorf("error count = %v, want 1", got)
	}
}

func TestOnlyGETFollowsAShortLink(t *testing.T) {
	h, signer := newHandler(t, nil)
	srv := serve(t, h)
	c, _ := signer.New()
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodHead} {
		req, _ := http.NewRequest(method, srv.URL+"/r/"+c, nil)
		resp, err := noFollow.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s /r/<code> = %d, want 405", method, resp.StatusCode)
		}
	}
}

func TestCreateLinksRejectsBadRequests(t *testing.T) {
	h, _ := newHandler(t, nil)
	srv := serve(t, h)
	id := uuid.NewString()
	one := func(mutate func(m map[string]string)) string {
		m := map[string]string{
			"delivery_id": id, "campaign_id": id, "channel": "sms", "language": "tr", "timezone": "Europe/Istanbul",
			"target_url": "https://shop.example.com/?utm_campaign=c",
		}
		mutate(m)
		var b strings.Builder
		b.WriteString(`{"links":[{`)
		first := true
		for k, v := range m {
			if v == "<omit>" {
				continue
			}
			if !first {
				b.WriteString(",")
			}
			first = false
			b.WriteString(`"` + k + `":"` + v + `"`)
		}
		b.WriteString(`}]}`)
		return b.String()
	}
	tests := map[string]struct {
		body   string
		status int
		code   string
	}{
		"not json":              {`{"links":`, 400, CodeInvalidJSON},
		"unknown field":         {`{"links":[],"extra":1}`, 400, CodeInvalidJSON},
		"empty list":            {`{"links":[]}`, 422, CodeInvalidRequest},
		"no list":               {`{}`, 422, CodeInvalidRequest},
		"too many":              {`{"links":[` + strings.TrimSuffix(strings.Repeat(`{},`, MaxLinksPerRequest+1), ",") + `]}`, 422, CodeInvalidRequest},
		"missing delivery":      {one(func(m map[string]string) { m["delivery_id"] = "<omit>" }), 422, CodeInvalidRequest},
		"delivery not a uuid":   {one(func(m map[string]string) { m["delivery_id"] = "d" }), 400, CodeInvalidJSON},
		"unknown channel":       {one(func(m map[string]string) { m["channel"] = "push" }), 422, CodeInvalidRequest},
		"unknown language":      {one(func(m map[string]string) { m["language"] = "de" }), 422, CodeInvalidRequest},
		"blank timezone":        {one(func(m map[string]string) { m["timezone"] = " " }), 422, CodeInvalidRequest},
		"relative target":       {one(func(m map[string]string) { m["target_url"] = "/shop" }), 422, CodeInvalidRequest},
		"javascript target":     {one(func(m map[string]string) { m["target_url"] = "javascript:alert(1)" }), 422, CodeInvalidRequest},
		"credentials in target": {one(func(m map[string]string) { m["target_url"] = "https://user:pw@shop.example.com" }), 422, CodeInvalidRequest},
		"target with a space":   {one(func(m map[string]string) { m["target_url"] = "https://shop.example.com/a b" }), 422, CodeInvalidRequest},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			resp, err := http.Post(srv.URL+"/links", "application/json", strings.NewReader(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != tt.status || resp.Header.Get("Content-Type") != ProblemContentType || !strings.Contains(string(body), `"code":"`+tt.code+`"`) {
				t.Errorf("status %d, content-type %q, body %s; want %d %s", resp.StatusCode, resp.Header.Get("Content-Type"), body, tt.status, tt.code)
			}
		})
	}
}

func TestCheckTargetURL(t *testing.T) {
	for _, ok := range []string{"https://shop.example.com", "http://localhost:8080/x?utm_campaign=a%20b", "https://a.co/p?q=1#frag"} {
		if err := checkTargetURL(ok); err != nil {
			t.Errorf("checkTargetURL(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "shop.example.com", "ftp://shop.example.com", "https://", "https://u@h.com", "https://h.com/\n", "https://h.com/" + strings.Repeat("a", maxTargetURLLen)} {
		if err := checkTargetURL(bad); err == nil {
			t.Errorf("checkTargetURL(%.40q) = nil, want an error", bad)
		}
	}
}

func TestNewRejectsAMissingDependency(t *testing.T) {
	signer, _ := code.NewSigner(testKey)
	if _, err := New(Config{Signer: signer, FallbackURL: fallback}); err == nil {
		t.Error("New without a pool: error = nil")
	}
	if _, err := New(Config{Pool: unreachablePool(t), FallbackURL: fallback}); err == nil {
		t.Error("New without a signer: error = nil")
	}
	if _, err := New(Config{Pool: unreachablePool(t), Signer: signer, FallbackURL: "/not-absolute"}); err == nil {
		t.Error("New with a relative fallback: error = nil")
	}
}
