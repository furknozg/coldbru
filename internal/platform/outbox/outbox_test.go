package outbox_test

import (
	"strings"
	"testing"

	"github.com/furknozg/coldbru/internal/platform/outbox"
)

func TestValidateRawSubject(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		subject string
		wantErr string // substring; empty means valid
	}{
		{"recipients batch subject", "recipients.0199f3a1-0000-7000-8000-000000000001", ""},
		{"single token", "recipients", ""},
		{"underscore and dash", "send.my_queue-1", ""},
		{"upper case", "Send.SMS", ""},
		{"empty", "", "empty"},
		{"star wildcard", "recipients.*", "wildcards"},
		{"full wildcard", "recipients.>", "wildcards"},
		{"space", "recipients. a", "wildcards or spaces"},
		{"leading dot", ".recipients", "empty token"},
		{"trailing dot", "recipients.", "empty token"},
		{"double dot", "recipients..x", "empty token"},
		{"envelope namespace", "evt.proposal.confirmed.v1", "Write"},
		{"envelope namespace alone", "evt.x", "Write"},
		{"newline", "a\nb", "wildcards or spaces"},
		{"too long", strings.Repeat("a", outbox.MaxSubjectLength+1), "longer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := outbox.ValidateRawSubject(tt.subject)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateRawSubject(%q) error = %v, want nil", tt.subject, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateRawSubject(%q) error = %v, want one containing %q", tt.subject, err, tt.wantErr)
			}
		})
	}
}

func TestValidateRawSubject_MaxLengthIsValid(t *testing.T) {
	t.Parallel()
	if err := outbox.ValidateRawSubject(strings.Repeat("a", outbox.MaxSubjectLength)); err != nil {
		t.Fatalf("ValidateRawSubject() error = %v, want nil at the limit", err)
	}
}

// WriteMessage checks its arguments before it touches the transaction, so a nil tx is enough here.
func TestWriteMessage_RejectsBadArguments(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		subject string
		payload []byte
		wantErr string
	}{
		{"bad subject", "recipients.*", []byte(`{}`), "outbox subject"},
		{"envelope subject", "evt.proposal.confirmed.v1", []byte(`{}`), "outbox subject"},
		{"nil payload", "recipients.x", nil, "outbox payload"},
		{"not json", "recipients.x", []byte(`{broken`), "outbox payload"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := outbox.WriteMessage(t.Context(), nil, tt.subject, tt.payload)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("WriteMessage() error = %v, want one containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateHeaders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		headers map[string]string
		wantErr string // substring; empty means valid
	}{
		{"nil", nil, ""},
		{"empty", map[string]string{}, ""},
		{"schedule pair", map[string]string{"Nats-Schedule": "@at 2026-10-29T12:00:00Z", "Nats-Schedule-Target": "fire.x"}, ""},
		{"value may hold spaces and colons", map[string]string{"X-Note": "a b: c"}, ""},
		{"empty value", map[string]string{"X-Note": ""}, ""},
		{"empty name", map[string]string{"": "v"}, "empty"},
		{"colon in name", map[string]string{"X:Y": "v"}, "not allowed"},
		{"space in name", map[string]string{"X Y": "v"}, "not allowed"},
		{"tab in name", map[string]string{"X\tY": "v"}, "not allowed"},
		{"newline in name", map[string]string{"X\nY": "v"}, "not allowed"},
		{"control character in name", map[string]string{"X\x00Y": "v"}, "not allowed"},
		{"newline in value", map[string]string{"X-Note": "a\nNats-Msg-Id: 1"}, "line break"},
		{"carriage return in value", map[string]string{"X-Note": "a\rb"}, "line break"},
		{"Nats-Msg-Id", map[string]string{"Nats-Msg-Id": "x"}, "row id"},
		{"nats-msg-id lower case", map[string]string{"nats-msg-id": "x"}, "row id"},
		{"NATS-MSG-ID upper case", map[string]string{"NATS-MSG-ID": "x"}, "row id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := outbox.ValidateHeaders(tt.headers)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateHeaders() error = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateHeaders() error = %v, want one containing %q", err, tt.wantErr)
			}
		})
	}
}

// WriteMessage checks its headers before it touches the transaction, so a nil tx is enough here.
func TestWriteMessage_RejectsBadHeaders(t *testing.T) {
	t.Parallel()
	_, err := outbox.WriteMessage(t.Context(), nil, "recipients.x", []byte(`{}`),
		outbox.WithHeaders(map[string]string{"Nats-Msg-Id": "x"}))
	if err == nil || !strings.Contains(err.Error(), "outbox headers") {
		t.Fatalf("WriteMessage() error = %v, want one containing %q", err, "outbox headers")
	}
	// A second WithHeaders adds to the first, so the bad name is still caught.
	_, err = outbox.WriteMessage(t.Context(), nil, "recipients.x", []byte(`{}`),
		outbox.WithHeaders(map[string]string{"X-A": "1"}), outbox.WithHeaders(map[string]string{"bad name": "2"}))
	if err == nil || !strings.Contains(err.Error(), "outbox headers") {
		t.Fatalf("WriteMessage() error = %v, want one containing %q", err, "outbox headers")
	}
}
