package consumer_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/furknozg/coldbru/internal/platform/consumer"
)

// fakeTx records how the transaction ended; other methods panic via the nil embed.
type fakeTx struct {
	pgx.Tx
	commits, rollbacks int
	commitErr          error
}

func (t *fakeTx) Commit(context.Context) error {
	t.commits++
	return t.commitErr
}

func (t *fakeTx) Rollback(context.Context) error {
	t.rollbacks++
	return nil
}

type fakePool struct {
	tx       *fakeTx
	beginErr error
}

func (p *fakePool) Begin(context.Context) (pgx.Tx, error) {
	if p.beginErr != nil {
		return nil, p.beginErr
	}
	return p.tx, nil
}

// rawMsg records how a message was settled and reports a delivery number.
type rawMsg struct {
	jetstream.Msg
	delivered uint64
	metaErr   error
	acked     int
	termd     int
	naked     []time.Duration
}

func (m *rawMsg) Data() []byte    { return []byte(`{}`) }
func (m *rawMsg) Subject() string { return "recipients.c1" }
func (m *rawMsg) Ack() error      { m.acked++; return nil }
func (m *rawMsg) Term() error     { m.termd++; return nil }
func (m *rawMsg) NakWithDelay(d time.Duration) error {
	m.naked = append(m.naked, d)
	return nil
}
func (m *rawMsg) Metadata() (*jetstream.MsgMetadata, error) {
	if m.metaErr != nil {
		return nil, m.metaErr
	}
	return &jetstream.MsgMetadata{NumDelivered: m.delivered}, nil
}

// logSink collects log lines.
type logSink struct {
	mu sync.Mutex
	sb strings.Builder
}

func (s *logSink) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sb.Write(p)
}

func (s *logSink) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sb.String()
}

var rawBackoff = consumer.Backoff{Base: time.Second, Factor: 2, Max: time.Minute}

func newRaw(h consumer.RawHandler) (*consumer.RawProcessor, *fakePool, *logSink) {
	pool := &fakePool{tx: &fakeTx{}}
	sink := &logSink{}
	return &consumer.RawProcessor{
		Pool:    pool,
		Logger:  slog.New(slog.NewTextHandler(sink, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Backoff: rawBackoff,
		Handler: h,
	}, pool, sink
}

func TestRawProcess_SuccessCommitsThenAcks(t *testing.T) {
	var gotSubject string
	var gotTx pgx.Tx
	p, pool, _ := newRaw(func(_ context.Context, tx pgx.Tx, msg jetstream.Msg) error {
		gotSubject = msg.Subject()
		gotTx = tx
		return nil
	})
	msg := &rawMsg{delivered: 1}
	if err := p.Process(t.Context(), msg); err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if gotSubject != "recipients.c1" {
		t.Errorf("handler subject = %q", gotSubject)
	}
	if gotTx != pgx.Tx(pool.tx) {
		t.Error("handler did not get the transaction of the pool")
	}
	if pool.tx.commits != 1 || msg.acked != 1 || msg.termd != 0 || len(msg.naked) != 0 {
		t.Errorf("commits/ack/term/nak = %d/%d/%d/%d, want 1/1/0/0", pool.tx.commits, msg.acked, msg.termd, len(msg.naked))
	}
}

func TestRawProcess_RetryLater(t *testing.T) {
	t.Run("handler delay is used and nothing is logged above debug", func(t *testing.T) {
		p, pool, sink := newRaw(func(context.Context, pgx.Tx, jetstream.Msg) error {
			return consumer.RetryLater(7*time.Second, "campaign not here yet")
		})
		msg := &rawMsg{delivered: 3}
		if err := p.Process(t.Context(), msg); err != nil {
			t.Fatalf("Process() error = %v, want nil (not a failure)", err)
		}
		if len(msg.naked) != 1 || msg.naked[0] != 7*time.Second {
			t.Errorf("naks = %v, want one of 7s", msg.naked)
		}
		if msg.acked != 0 || msg.termd != 0 || pool.tx.commits != 0 || pool.tx.rollbacks == 0 {
			t.Errorf("ack/term/commits/rollbacks = %d/%d/%d/%d, want 0/0/0/>0", msg.acked, msg.termd, pool.tx.commits, pool.tx.rollbacks)
		}
		if strings.Contains(sink.String(), "level=ERROR") || strings.Contains(sink.String(), "level=WARN") {
			t.Errorf("retry later logged above debug: %s", sink.String())
		}
	})
	t.Run("zero delay falls back to the backoff of the attempt", func(t *testing.T) {
		p, _, _ := newRaw(func(context.Context, pgx.Tx, jetstream.Msg) error {
			return fmt.Errorf("wrapped: %w", consumer.RetryLater(0, "waiting"))
		})
		msg := &rawMsg{delivered: 4}
		if err := p.Process(t.Context(), msg); err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if len(msg.naked) != 1 || msg.naked[0] != 8*time.Second { // 1s * 2^(4-1)
			t.Errorf("naks = %v, want one of 8s", msg.naked)
		}
	})
	t.Run("terminated with a log at the delivery limit", func(t *testing.T) {
		p, _, sink := newRaw(func(context.Context, pgx.Tx, jetstream.Msg) error {
			return consumer.RetryLater(time.Second, "still waiting")
		})
		msg := &rawMsg{delivered: consumer.MaxDeliver}
		if err := p.Process(t.Context(), msg); err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if msg.termd != 1 || len(msg.naked) != 0 {
			t.Errorf("term/nak = %d/%d, want 1/0", msg.termd, len(msg.naked))
		}
		if !strings.Contains(sink.String(), "level=ERROR") || !strings.Contains(sink.String(), "still waiting") {
			t.Errorf("log = %s, want an error naming the reason", sink.String())
		}
	})
}

func TestRawProcess_MalformedIsTerminatedAndLogged(t *testing.T) {
	cause := errors.New("recipient has no schedule")
	p, pool, sink := newRaw(func(context.Context, pgx.Tx, jetstream.Msg) error {
		return fmt.Errorf("decode: %w", consumer.Malformed(cause))
	})
	msg := &rawMsg{delivered: 1}
	if err := p.Process(t.Context(), msg); err != nil {
		t.Fatalf("Process() error = %v, want nil", err)
	}
	if msg.termd != 1 || msg.acked != 0 || len(msg.naked) != 0 || pool.tx.commits != 0 {
		t.Errorf("term/ack/nak/commits = %d/%d/%d/%d, want 1/0/0/0", msg.termd, msg.acked, len(msg.naked), pool.tx.commits)
	}
	if log := sink.String(); !strings.Contains(log, "level=ERROR") || !strings.Contains(log, "recipient has no schedule") {
		t.Errorf("log = %s, want an error with the cause", log)
	}
	if !errors.Is(consumer.Malformed(cause), cause) {
		t.Error("Malformed(err) does not unwrap to err")
	}
}

func TestRawProcess_FailureBacksOffThenTerminates(t *testing.T) {
	boom := errors.New("boom")
	h := func(context.Context, pgx.Tx, jetstream.Msg) error { return boom }

	t.Run("nak with backoff and the error returned", func(t *testing.T) {
		p, pool, sink := newRaw(h)
		msg := &rawMsg{delivered: 3}
		err := p.Process(t.Context(), msg)
		if !errors.Is(err, boom) {
			t.Fatalf("Process() error = %v, want it to wrap boom", err)
		}
		if len(msg.naked) != 1 || msg.naked[0] != 4*time.Second || msg.termd != 0 || pool.tx.commits != 0 {
			t.Errorf("naks/term/commits = %v/%d/%d, want one 4s nak", msg.naked, msg.termd, pool.tx.commits)
		}
		if sink.String() != "" {
			t.Errorf("Process logged %q, want nothing (single handling rule)", sink.String())
		}
	})
	t.Run("terminated and logged at the delivery limit", func(t *testing.T) {
		p, _, sink := newRaw(h)
		msg := &rawMsg{delivered: consumer.MaxDeliver}
		if err := p.Process(t.Context(), msg); err != nil {
			t.Fatalf("Process() error = %v, want nil", err)
		}
		if msg.termd != 1 || len(msg.naked) != 0 {
			t.Errorf("term/nak = %d/%d, want 1/0", msg.termd, len(msg.naked))
		}
		if !strings.Contains(sink.String(), "level=ERROR") || !strings.Contains(sink.String(), "boom") {
			t.Errorf("log = %s, want an error with the cause", sink.String())
		}
	})
	t.Run("unreadable metadata still naks", func(t *testing.T) {
		p, _, _ := newRaw(h)
		msg := &rawMsg{metaErr: errors.New("no metadata")}
		err := p.Process(t.Context(), msg)
		if !errors.Is(err, boom) || len(msg.naked) != 1 {
			t.Errorf("error = %v, naks = %v, want boom and one nak", err, msg.naked)
		}
	})
}

func TestRawProcess_DatabaseErrors(t *testing.T) {
	t.Run("begin fails", func(t *testing.T) {
		called := false
		p, pool, _ := newRaw(func(context.Context, pgx.Tx, jetstream.Msg) error { called = true; return nil })
		pool.beginErr = errors.New("pool closed")
		msg := &rawMsg{delivered: 1}
		if err := p.Process(t.Context(), msg); err == nil || !strings.Contains(err.Error(), "pool closed") {
			t.Fatalf("Process() error = %v, want pool closed", err)
		}
		if called || len(msg.naked) != 1 {
			t.Errorf("handler called = %v, naks = %v, want not called and one nak", called, msg.naked)
		}
	})
	t.Run("commit fails: no ack, nak", func(t *testing.T) {
		p, pool, _ := newRaw(func(context.Context, pgx.Tx, jetstream.Msg) error { return nil })
		pool.tx.commitErr = errors.New("serialization failure")
		msg := &rawMsg{delivered: 1}
		if err := p.Process(t.Context(), msg); err == nil || !strings.Contains(err.Error(), "serialization failure") {
			t.Fatalf("Process() error = %v", err)
		}
		if msg.acked != 0 || len(msg.naked) != 1 {
			t.Errorf("ack/nak = %d/%d, want 0/1", msg.acked, len(msg.naked))
		}
	})
}

func TestRawProcess_AfterAckRunsOnlyAfterACommittedAck(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name    string
		handler error
		commit  error
		want    int
	}{
		{"success", nil, nil, 1},
		{"retry later", consumer.RetryLater(time.Second, "wait"), nil, 0},
		{"malformed", consumer.Malformed(boom), nil, 0},
		{"failure", boom, nil, 0},
		{"commit fails", nil, errors.New("commit"), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, pool, _ := newRaw(func(context.Context, pgx.Tx, jetstream.Msg) error { return tt.handler })
			pool.tx.commitErr = tt.commit
			msg := &rawMsg{delivered: 1}
			calls, ackedAtCall := 0, -1
			p.AfterAck = func(_ context.Context, m jetstream.Msg) {
				calls++
				ackedAtCall = msg.acked
				if m != jetstream.Msg(msg) {
					t.Error("AfterAck got a different message")
				}
			}
			_ = p.Process(t.Context(), msg)
			if calls != tt.want {
				t.Fatalf("AfterAck calls = %d, want %d", calls, tt.want)
			}
			if tt.want == 1 && ackedAtCall != 1 {
				t.Errorf("acks when AfterAck ran = %d, want 1 (after the ack)", ackedAtCall)
			}
		})
	}
}
