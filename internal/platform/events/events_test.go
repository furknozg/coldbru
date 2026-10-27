package events_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/furknozg/coldbru/internal/platform/events"
)

func TestSubject(t *testing.T) {
	tests := []struct {
		name    string
		typ     string
		version int
		want    string
		wantErr string
	}{
		{name: "simple", typ: "proposal.confirmed", version: 1, want: "evt.proposal.confirmed.v1"},
		{name: "higher version", typ: "notification.completed", version: 12, want: "evt.notification.completed.v12"},
		{name: "underscore and digits", typ: "proposal.token_completed2", version: 1, want: "evt.proposal.token_completed2.v1"},
		{name: "no dot", typ: "proposal", version: 1, wantErr: "want <service>.<event>"},
		{name: "empty", typ: "", version: 1, wantErr: "want <service>.<event>"},
		{name: "two dots", typ: "a.b.c", version: 1, wantErr: "exactly one dot"},
		{name: "empty service", typ: ".confirmed", version: 1, wantErr: "service"},
		{name: "empty event", typ: "proposal.", version: 1, wantErr: "event"},
		{name: "uppercase service", typ: "Proposal.confirmed", version: 1, wantErr: "service"},
		{name: "uppercase event", typ: "proposal.Confirmed", version: 1, wantErr: "event"},
		{name: "leading digit", typ: "proposal.1confirmed", version: 1, wantErr: "event"},
		{name: "hyphen", typ: "proposal.token-completed", version: 1, wantErr: "event"},
		{name: "wildcard", typ: "proposal.*", version: 1, wantErr: "event"},
		{name: "version zero", typ: "proposal.confirmed", version: 0, wantErr: "version"},
		{name: "version negative", typ: "proposal.confirmed", version: -1, wantErr: "version"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := events.Subject(events.Envelope{Type: tt.typ, Version: tt.version})
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Subject() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Subject() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("Subject() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	before := time.Now().UTC()
	e, err := events.New("proposal.confirmed", 1, "proposal", "camp-1", map[string]int{"n": 3})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if e.ID != "" {
		t.Errorf("ID = %q, want empty", e.ID)
	}
	if e.Type != "proposal.confirmed" || e.Version != 1 || e.Producer != "proposal" || e.CorrelationID != "camp-1" {
		t.Errorf("envelope = %+v", e)
	}
	if string(e.Data) != `{"n":3}` {
		t.Errorf("Data = %s, want {\"n\":3}", e.Data)
	}
	if e.OccurredAt.Before(before) || e.OccurredAt.Location() != time.UTC {
		t.Errorf("OccurredAt = %v, want UTC time >= %v", e.OccurredAt, before)
	}

	t.Run("unmarshalable data", func(t *testing.T) {
		if _, err := events.New("a.b", 1, "p", "c", make(chan int)); err == nil {
			t.Error("New() error = nil, want marshal error")
		}
	})
}

func TestEnvelope_JSONRoundTrip(t *testing.T) {
	in := events.Envelope{
		ID:            "0199f3a2-7c4e-7b1a-9d2e-5f6a7b8c9d0e",
		Type:          "proposal.confirmed",
		Version:       1,
		OccurredAt:    time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC),
		Producer:      "proposal",
		CorrelationID: "0199f3a1-0000-7000-8000-000000000001",
		Data:          json.RawMessage(`{"campaign_id":"c1"}`),
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	t.Run("field names", func(t *testing.T) {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		for _, k := range []string{"id", "type", "version", "occurred_at", "producer", "correlation_id", "data"} {
			if _, ok := m[k]; !ok {
				t.Errorf("missing field %q in %s", k, raw)
			}
		}
		if len(m) != 7 {
			t.Errorf("got %d fields, want 7: %s", len(m), raw)
		}
		if got, want := string(m["occurred_at"]), `"2026-10-27T09:00:00Z"`; got != want {
			t.Errorf("occurred_at = %s, want %s", got, want)
		}
	})

	t.Run("round trip", func(t *testing.T) {
		var out events.Envelope
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if !out.OccurredAt.Equal(in.OccurredAt) {
			t.Errorf("OccurredAt = %v, want %v", out.OccurredAt, in.OccurredAt)
		}
		out.OccurredAt = in.OccurredAt
		if out.ID != in.ID || out.Type != in.Type || out.Version != in.Version ||
			out.Producer != in.Producer || out.CorrelationID != in.CorrelationID ||
			string(out.Data) != string(in.Data) {
			t.Errorf("round trip = %+v, want %+v", out, in)
		}
	})
}
