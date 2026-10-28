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
