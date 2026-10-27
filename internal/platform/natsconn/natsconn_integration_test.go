//go:build integration

package natsconn_test

import (
	"io"
	"log/slog"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"

	"github.com/furknozg/coldbru/internal/platform/natsconn"
)

func TestConnectAndCheck(t *testing.T) {
	ctx := t.Context()
	ctr, err := tcnats.Run(ctx, "nats:2.15-alpine")
	if err != nil {
		t.Fatalf("start nats container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(ctr); err != nil {
			t.Errorf("terminate nats container: %v", err)
		}
	})
	url, err := ctr.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	nc, js, err := natsconn.Connect(url, "natsconn-test", logger)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(nc.Close)
	if js == nil {
		t.Fatal("Connect() returned nil JetStream")
	}

	t.Run("check passes while connected", func(t *testing.T) {
		if err := natsconn.Check(nc)(ctx); err != nil {
			t.Errorf("Check() error = %v", err)
		}
	})

	t.Run("check fails after close", func(t *testing.T) {
		closed, _, err := natsconn.Connect(url, "natsconn-test-closed", logger)
		if err != nil {
			t.Fatalf("Connect() error = %v", err)
		}
		closed.Close()
		if err := natsconn.Check(closed)(ctx); err == nil {
			t.Error("Check() error = nil, want error after Close")
		}
	})
}

func TestConnect_Unreachable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if nc, _, err := natsconn.Connect("nats://127.0.0.1:1", "natsconn-test", logger); err == nil {
		nc.Close()
		t.Fatal("Connect() error = nil, want error for unreachable server")
	}
}
