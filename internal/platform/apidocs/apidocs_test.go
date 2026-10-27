package apidocs_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/furknozg/coldbru/internal/platform/apidocs"
)

const testSpec = `{"openapi":"3.1.0","info":{"title":"Test","version":"0.0.1"},"paths":{}}`

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestMount(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	if err := apidocs.Mount(r, []byte(testSpec), "Test API"); err != nil {
		t.Fatalf("Mount() error = %v", err)
	}

	t.Run("openapi.json", func(t *testing.T) {
		t.Parallel()
		rec := get(r, "/openapi.json")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		if !bytes.Equal(rec.Body.Bytes(), []byte(testSpec)) {
			t.Errorf("body = %q, want the exact spec bytes", rec.Body.String())
		}
	})

	t.Run("docs", func(t *testing.T) {
		t.Parallel()
		rec := get(r, "/docs")
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("Content-Type = %q, want text/html", ct)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `"url":"openapi.json"`) {
			t.Errorf("page does not point at the relative spec url: %s", body)
		}
		if strings.Contains(body, "placeholder.invalid") {
			t.Error("page still contains the placeholder spec url")
		}
		if !strings.Contains(body, apidocs.ScalarCDN) {
			t.Errorf("page does not load pinned Scalar bundle %s", apidocs.ScalarCDN)
		}
		if !strings.Contains(body, "Test API") {
			t.Error("page does not carry the title")
		}
	})
}

func TestMountEmptySpecRegistersNothing(t *testing.T) {
	t.Parallel()
	for name, spec := range map[string][]byte{"nil": nil, "empty": {}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := chi.NewRouter()
			if err := apidocs.Mount(r, spec, "Test API"); err != nil {
				t.Fatalf("Mount() error = %v", err)
			}
			for _, p := range []string{"/docs", "/openapi.json"} {
				if rec := get(r, p); rec.Code != http.StatusNotFound {
					t.Errorf("GET %s = %d, want 404", p, rec.Code)
				}
			}
		})
	}
}
