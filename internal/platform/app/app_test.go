package app

import (
	"bytes"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/spf13/viper"

	"github.com/furknozg/coldbru/internal/platform/config"
)

func TestPortOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		addr    string
		want    string
		wantErr bool
	}{
		{addr: ":8080", want: "8080"},
		{addr: "0.0.0.0:9000", want: "9000"},
		{addr: "127.0.0.1:1", want: "1"},
		{addr: "[::]:8080", want: "8080"},
		{addr: "8080", wantErr: true},
		{addr: "localhost:", wantErr: true},
		{addr: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			t.Parallel()
			got, err := portOf(tt.addr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("portOf(%q) error = %v, wantErr %v", tt.addr, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("portOf(%q) = %q, want %q", tt.addr, got, tt.want)
			}
		})
	}
}

func TestHealthcheck(t *testing.T) {
	t.Parallel()
	serve := func(t *testing.T, code int) string {
		t.Helper()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/readyz" {
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(code)
		}))
		t.Cleanup(srv.Close)
		return srv.Listener.Addr().String()
	}

	t.Run("200 is ready", func(t *testing.T) {
		t.Parallel()
		if err := Healthcheck(serve(t, http.StatusOK)); err != nil {
			t.Errorf("Healthcheck() error = %v, want nil", err)
		}
	})
	t.Run("503 is not ready", func(t *testing.T) {
		t.Parallel()
		if err := Healthcheck(serve(t, http.StatusServiceUnavailable)); err == nil {
			t.Error("Healthcheck() error = nil, want error for 503")
		}
	})
	t.Run("unreachable", func(t *testing.T) {
		t.Parallel()
		// Reserve a port, then free it so nothing listens there.
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		addr := ln.Addr().String()
		_ = ln.Close()
		if err := Healthcheck(addr); err == nil {
			t.Error("Healthcheck() error = nil, want error for unreachable server")
		}
	})
	t.Run("bad addr", func(t *testing.T) {
		t.Parallel()
		if err := Healthcheck("nonsense"); err == nil {
			t.Error("Healthcheck() error = nil, want error for addr without port")
		}
	})
}

func TestRun_PlatformMigrationsRequireDatabase(t *testing.T) {
	t.Parallel()
	err := Run(t.Context(), Options{Name: "x", PlatformMigrations: true}, nil, nil)
	if err == nil {
		t.Fatal("Run() error = nil, want error")
	}
}

func TestMountAPIDocs(t *testing.T) {
	t.Parallel()
	const spec = `{"openapi":"3.1.0","info":{"title":"T","version":"1"},"paths":{}}`
	tests := []struct {
		name    string
		enabled bool
		spec    string
		want    int
		wantLog string
	}{
		{name: "enabled with spec", enabled: true, spec: spec, want: http.StatusOK, wantLog: "enabled=true"},
		{name: "disabled with spec", enabled: false, spec: spec, want: http.StatusNotFound, wantLog: "enabled=false"},
		{name: "enabled without spec", enabled: true, spec: "", want: http.StatusNotFound, wantLog: "enabled=false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			v := viper.New()
			v.Set(config.KeyAPIDocsEnabled, tt.enabled)
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			r := chi.NewRouter()

			err := mountAPIDocs(r, v, Options{Name: "svc", OpenAPISpec: []byte(tt.spec)}, logger)
			if err != nil {
				t.Fatalf("mountAPIDocs() error = %v", err)
			}
			for _, p := range []string{"/docs", "/openapi.json"} {
				rec := httptest.NewRecorder()
				r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
				if rec.Code != tt.want {
					t.Errorf("GET %s = %d, want %d", p, rec.Code, tt.want)
				}
				_, _ = io.Copy(io.Discard, rec.Body)
			}
			if !strings.Contains(logs.String(), tt.wantLog) {
				t.Errorf("log %q does not contain %q", logs.String(), tt.wantLog)
			}
		})
	}
}
