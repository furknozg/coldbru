package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

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

func TestRun_ServiceMigrationsRequireDatabase(t *testing.T) {
	t.Parallel()
	err := Run(t.Context(), Options{Name: "x", Migrations: fstest.MapFS{}}, nil, nil)
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

func TestRun_ShutdownHooksRunAfterTheServerStopsInReverseOrder(t *testing.T) {
	t.Parallel()
	v := viper.New()
	v.Set(config.KeyHTTPAddr, "127.0.0.1:0")
	v.Set(config.KeyShutdownTimeout, "5s")
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var (
		order    []string
		deadline bool
	)
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{Name: "svc", Setup: func(ctx context.Context, d *Deps) error {
			d.OnShutdown(func(context.Context) error { order = append(order, "first"); return nil })
			d.OnShutdown(func(ctx context.Context) error {
				_, deadline = ctx.Deadline()
				order = append(order, "second")
				return errors.New("boom")
			})
			go func() { time.Sleep(50 * time.Millisecond); cancel() }()
			return nil
		}}, v, logger)
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return")
	}
	if got := strings.Join(order, ","); got != "second,first" {
		t.Errorf("hooks ran %q, want second,first", got)
	}
	if !deadline {
		t.Error("a hook got no deadline")
	}
	if !strings.Contains(logs.String(), "shutdown hook failed") {
		t.Errorf("the failure of a hook is not logged: %s", logs.String())
	}
}

func TestRun_ShutdownHooksRunWhenSetupFails(t *testing.T) {
	t.Parallel()
	v := viper.New()
	v.Set(config.KeyShutdownTimeout, "5s")
	ran := false
	err := Run(t.Context(), Options{Name: "svc", Setup: func(_ context.Context, d *Deps) error {
		d.OnShutdown(func(context.Context) error { ran = true; return nil })
		return errors.New("bad config")
	}}, v, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err == nil || !strings.Contains(err.Error(), "bad config") {
		t.Fatalf("Run() error = %v", err)
	}
	if !ran {
		t.Error("the hook of a half-built service did not run")
	}
}
