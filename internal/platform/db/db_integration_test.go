//go:build integration

package db_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/furknozg/coldbru/internal/platform/db"
)

// dsn is the connection string of the container shared by the package's tests.
var dsn string

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	ctr, err := tcpostgres.Run(ctx, "postgres:18-alpine", tcpostgres.BasicWaitStrategies())
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres container: %v\n", err)
		return 1
	}
	defer func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			fmt.Fprintf(os.Stderr, "terminate postgres container: %v\n", err)
		}
	}()

	dsn, err = ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "container connection string: %v\n", err)
		return 1
	}
	return m.Run()
}

func TestOpenAndCheck(t *testing.T) {
	ctx := t.Context()

	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(pool.Close)

	t.Run("check passes on live pool", func(t *testing.T) {
		if err := db.Check(pool)(ctx); err != nil {
			t.Errorf("Check() error = %v", err)
		}
	})

	t.Run("check fails on closed pool", func(t *testing.T) {
		closed, err := db.Open(ctx, dsn)
		if err != nil {
			t.Fatalf("Open() error = %v", err)
		}
		closed.Close()
		if err := db.Check(closed)(ctx); err == nil {
			t.Error("Check() error = nil, want error after Close")
		}
	})
}

func TestOpen_Unreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	pool, err := db.Open(ctx, "postgres://u:p@127.0.0.1:1/d?sslmode=disable&connect_timeout=2")
	if err == nil {
		pool.Close()
		t.Fatal("Open() error = nil, want ping error")
	}
}
