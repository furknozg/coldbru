//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/testcontainers/testcontainers-go"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/furknozg/coldbru/internal/platform/app"
	"github.com/furknozg/coldbru/internal/platform/db"
)

func TestRun_DatabasePlatformMigrationsNATS(t *testing.T) {
	ctx := t.Context()

	pg, err := tcpostgres.Run(ctx, "postgres:18-alpine", tcpostgres.BasicWaitStrategies())
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(pg) })
	ns, err := tcnats.Run(ctx, "nats:2.15-alpine")
	if err != nil {
		t.Fatalf("start nats container: %v", err)
	}
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(ns) })

	baseDSN, err := pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("postgres connection string: %v", err)
	}
	natsURL, err := ns.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("nats connection string: %v", err)
	}

	// The service schema exists before the service starts (initdb, D-048).
	admin, err := db.Open(ctx, baseDSN)
	if err != nil {
		t.Fatalf("open admin pool: %v", err)
	}
	t.Cleanup(admin.Close)
	if _, err := admin.Exec(ctx, "CREATE SCHEMA svc"); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	u, err := url.Parse(baseDSN)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	q := u.Query()
	q.Set("search_path", "svc,public")
	u.RawQuery = q.Encode()

	addr := freeAddr(t)
	v := viper.New()
	v.Set("HTTP_ADDR", addr)
	v.Set("SHUTDOWN_TIMEOUT", "5s")
	v.Set("DATABASE_URL", u.String())
	v.Set("NATS_URL", natsURL)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- app.Run(runCtx, app.Options{Name: "app-test", Database: true, PlatformMigrations: true, NATS: true}, v, logger)
	}()

	body := waitReady(t, addr, done)
	var rep struct {
		Status string `json:"status"`
		Checks map[string]struct {
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(body, &rep); err != nil {
		t.Fatalf("decode readyz %q: %v", body, err)
	}
	if rep.Status != "ok" {
		t.Errorf("readyz status = %q, want ok", rep.Status)
	}
	for _, name := range []string{"postgres", "nats"} {
		if got := rep.Checks[name].Status; got != "ok" {
			t.Errorf("check %q = %q, want ok", name, got)
		}
	}

	var n int
	err = admin.QueryRow(ctx,
		"SELECT count(*) FROM information_schema.tables WHERE table_schema = 'svc' AND table_name IN ('outbox', 'processed_event', 'goose_platform_version')").Scan(&n)
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	if n != 3 {
		t.Errorf("platform tables in schema svc = %d, want 3", n)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Run() after cancel error = %v, want nil", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Run() did not return after context cancel")
	}
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
	return addr
}

// waitReady polls /readyz until it answers 200 and returns the body. It fails
// early if Run exits first.
func waitReady(t *testing.T, addr string, done <-chan error) []byte {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-done:
			t.Fatalf("Run() exited early: %v", err)
		default:
		}
		resp, err := http.Get("http://" + addr + "/readyz")
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return body
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("readyz never returned 200")
	return nil
}
