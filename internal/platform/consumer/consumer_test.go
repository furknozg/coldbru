package consumer_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/furknozg/coldbru/internal/platform/consumer"
	"github.com/furknozg/coldbru/internal/platform/events"
)

func TestBackoff_Delay(t *testing.T) {
	b := consumer.DefaultBackoff
	tests := []struct {
		attempt int
		want    time.Duration // centre before jitter
	}{
		{1, time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{6, 32 * time.Second},
		{7, 64 * time.Second},
		{8, 128 * time.Second},
		{9, 256 * time.Second},
		{10, 512 * time.Second},
		{11, 15 * time.Minute}, // 1024s is capped
		{12, 15 * time.Minute},
		{19, 15 * time.Minute},
		{20, 15 * time.Minute},
		{1000, 15 * time.Minute}, // huge attempts must not overflow
		{0, time.Second},         // out of range clamps to the first attempt
		{-3, time.Second},        // out of range clamps to the first attempt
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("attempt %d", tt.attempt), func(t *testing.T) {
			lo := time.Duration(float64(tt.want) * 0.8)
			hi := time.Duration(float64(tt.want) * 1.2)
			for range 500 {
				got := b.Delay(tt.attempt)
				if got < lo || got > hi {
					t.Fatalf("Delay(%d) = %v, want within [%v, %v]", tt.attempt, got, lo, hi)
				}
				if got < 0 {
					t.Fatalf("Delay(%d) = %v, want non-negative", tt.attempt, got)
				}
				if got > time.Duration(float64(b.Max)*1.2) {
					t.Fatalf("Delay(%d) = %v, above Max+jitter", tt.attempt, got)
				}
			}
		})
	}

	t.Run("default policy values", func(t *testing.T) {
		if b.Base != time.Second || b.Factor != 2 || b.Max != 15*time.Minute || b.Jitter != 0.2 {
			t.Errorf("DefaultBackoff = %+v, want 1s x2 max 15m jitter 0.2", b)
		}
	})

	t.Run("no jitter is exact", func(t *testing.T) {
		nb := consumer.Backoff{Base: 10 * time.Millisecond, Factor: 3, Max: time.Second}
		for attempt, want := range map[int]time.Duration{1: 10 * time.Millisecond, 2: 30 * time.Millisecond, 3: 90 * time.Millisecond, 9: time.Second} {
			if got := nb.Delay(attempt); got != want {
				t.Errorf("Delay(%d) = %v, want %v", attempt, got, want)
			}
		}
	})

	t.Run("jitter actually varies", func(t *testing.T) {
		seen := map[time.Duration]bool{}
		for range 50 {
			seen[b.Delay(3)] = true
		}
		if len(seen) < 2 {
			t.Error("Delay(3) returned the same value 50 times, want jitter")
		}
	})
}

func TestConsumerConfig(t *testing.T) {
	cfg := consumer.ConsumerConfig("notification", "evt.proposal.>")
	if cfg.Durable != "notification" || cfg.FilterSubject != "evt.proposal.>" {
		t.Errorf("durable/filter = %q/%q", cfg.Durable, cfg.FilterSubject)
	}
	if cfg.AckPolicy != jetstream.AckExplicitPolicy {
		t.Errorf("AckPolicy = %v, want explicit", cfg.AckPolicy)
	}
	if cfg.MaxDeliver != consumer.MaxDeliver || consumer.MaxDeliver != 20 {
		t.Errorf("MaxDeliver = %d (const %d), want 20", cfg.MaxDeliver, consumer.MaxDeliver)
	}
	if cfg.AckWait != 30*time.Second {
		t.Errorf("AckWait = %v, want 30s", cfg.AckWait)
	}
}

// fakeMsg records how a message was settled; unused methods panic via the nil embed.
type fakeMsg struct {
	jetstream.Msg
	data  []byte
	acked int
	termd int
	naked int
}

func (m *fakeMsg) Data() []byte    { return m.data }
func (m *fakeMsg) Subject() string { return "evt.test.thing.v1" }
func (m *fakeMsg) Ack() error      { m.acked++; return nil }
func (m *fakeMsg) Term() error     { m.termd++; return nil }
func (m *fakeMsg) NakWithDelay(time.Duration) error {
	m.naked++
	return nil
}

func TestProcess_MalformedIsTerminatedBeforeTouchingDatabase(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "not json", data: "not json"},
		{name: "json array", data: `[1,2]`},
		{name: "empty id", data: `{"id":"","type":"a.b","version":1}`},
		{name: "missing id", data: `{"type":"a.b","version":1}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			p := &consumer.Processor{
				// Pool is nil on purpose: these messages must never reach the database.
				Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
				Handler: func(context.Context, pgx.Tx, events.Envelope) error {
					called = true
					return nil
				},
			}
			msg := &fakeMsg{data: []byte(tt.data)}
			if err := p.Process(t.Context(), msg); err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if msg.termd != 1 || msg.acked != 0 || msg.naked != 0 {
				t.Errorf("term/ack/nak = %d/%d/%d, want 1/0/0", msg.termd, msg.acked, msg.naked)
			}
			if called {
				t.Error("handler called, want not called")
			}
		})
	}
}
