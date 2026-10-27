package natsconn_test

import (
	"strings"
	"testing"

	"github.com/nats-io/nats.go"

	"github.com/furknozg/coldbru/internal/platform/natsconn"
)

func TestCheck_NeverConnected(t *testing.T) {
	t.Parallel()
	// A zero Conn has never connected: its status is DISCONNECTED.
	err := natsconn.Check(&nats.Conn{})(t.Context())
	if err == nil {
		t.Fatal("Check() error = nil, want error for a conn that is not connected")
	}
	if !strings.Contains(err.Error(), "DISCONNECTED") {
		t.Errorf("Check() error = %q, want it to name the status", err)
	}
}
