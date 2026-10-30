//go:build integration

package intake_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// TestNATSScheduling pins down how nats:2.15 handles the scheduled messages D-078 relies on, against
// the real SCHEDULES stream definition: Nats-Schedule "@at <RFC3339>" with Nats-Schedule-Target on a
// message published to sched.<id> delivers the payload on the target fire.<id> at that time, once.
// The service's own SCHEDULES consumer also sees the probe's fire.* messages and terminates them
// as malformed; that does not matter here, the test reads the stream.
func TestNATSScheduling(t *testing.T) {
	e := newEnv(t)
	st := e.streams["SCHEDULES"]
	ctx := t.Context()

	id := "probe-" + uuid.NewString()
	sendAt := time.Now().Add(3 * time.Second).UTC().Truncate(time.Second).Add(time.Second)
	payload := []byte(`{"schedule_id":"` + id + `"}`)
	publish := func(subject, at, target string) error {
		h := nats.Header{"Nats-Schedule": {"@at " + at}}
		if target != "" {
			h["Nats-Schedule-Target"] = []string{target}
		}
		_, err := e.js.PublishMsg(ctx, &nats.Msg{Subject: subject, Data: payload, Header: h})
		return err
	}

	t.Run("fires on the target at the time, once, and the schedule message goes", func(t *testing.T) {
		if err := publish("sched."+id, sendAt.Format(time.RFC3339), "fire."+id); err != nil {
			t.Fatalf("publish scheduled message: %v", err)
		}
		// Nothing on the target before the time.
		time.Sleep(1 * time.Second)
		if _, err := st.GetLastMsgForSubject(ctx, "fire."+id); !errors.Is(err, jetstream.ErrMsgNotFound) {
			t.Fatalf("fire.%s before its time: err = %v, want message not found", id, err)
		}
		if _, err := st.GetLastMsgForSubject(ctx, "sched."+id); err != nil {
			t.Fatalf("sched.%s while waiting: err = %v, want the schedule message", id, err)
		}

		eventually(t, 10*time.Second, "message on the target", func() bool {
			_, err := st.GetLastMsgForSubject(ctx, "fire."+id)
			return err == nil
		})
		msg, err := st.GetLastMsgForSubject(ctx, "fire."+id)
		if err != nil {
			t.Fatal(err)
		}
		// The server stamps the target message on the clock its scheduler fires by; the host clock
		// of the Docker VM can drift from it by a few milliseconds (D-097).
		fired := msg.Time
		if fired.Before(sendAt) {
			t.Errorf("delivered at %s, before the scheduled %s", fired.Format(time.RFC3339Nano), sendAt.Format(time.RFC3339Nano))
		}
		if late := fired.Sub(sendAt); late > 2*time.Second {
			t.Errorf("delivered %v after the scheduled time, want within 2s", late)
		}
		if string(msg.Data) != string(payload) {
			t.Errorf("target payload = %s, want the schedule message's %s", msg.Data, payload)
		}
		if got := msg.Header.Get("Nats-Scheduler"); got != "sched."+id {
			t.Errorf("Nats-Scheduler = %q, want sched.%s", got, id)
		}
		// A one-shot @at schedule is removed once it fired.
		time.Sleep(500 * time.Millisecond)
		if _, err := st.GetLastMsgForSubject(ctx, "sched."+id); !errors.Is(err, jetstream.ErrMsgNotFound) {
			t.Errorf("sched.%s after firing: err = %v, want message not found", id, err)
		}
		if got := e.subjectCounts(t, "SCHEDULES")["fire."+id]; got != 1 {
			t.Errorf("messages on fire.%s = %d, want exactly 1", id, got)
		}
	})

	t.Run("a time in the past fires at once", func(t *testing.T) {
		pid := "probe-" + uuid.NewString()
		if err := publish("sched."+pid, time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), "fire."+pid); err != nil {
			t.Fatalf("publish: %v", err)
		}
		eventually(t, 5*time.Second, "message on the target", func() bool {
			_, err := st.GetLastMsgForSubject(ctx, "fire."+pid)
			return err == nil
		})
	})

	t.Run("target must be set, inside the stream and not the schedule subject", func(t *testing.T) {
		tests := []struct{ name, subject, target string }{
			{"no target", "sched.t1", ""},
			{"target outside the stream", "sched.t2", "other.t2"},
			{"target is the schedule subject", "sched.t3", "sched.t3"},
		}
		for _, tt := range tests {
			err := publish(tt.subject, sendAt.Format(time.RFC3339), tt.target)
			if err == nil || !strings.Contains(err.Error(), "message schedules target is invalid") {
				t.Errorf("%s: error = %v, want \"message schedules target is invalid\"", tt.name, err)
			}
		}
	})
}
