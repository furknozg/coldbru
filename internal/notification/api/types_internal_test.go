package api

import (
	"testing"
	"time"
)

func TestSameState(t *testing.T) {
	t.Parallel()
	base := DeliverySummary{Status: "sending", Total: 3, Counts: []ChannelCount{{"sms", "pending", 1}, {"sms", "sent", 2}}}
	later := base
	later.UpdatedAt = base.UpdatedAt.Add(time.Minute)
	status := base
	status.Status = "completed"
	count := DeliverySummary{Status: "sending", Total: 3, Counts: []ChannelCount{{"sms", "pending", 2}, {"sms", "sent", 1}}}
	shorter := DeliverySummary{Status: "sending", Total: 3, Counts: []ChannelCount{{"sms", "sent", 3}}}
	other := DeliverySummary{Status: "sending", Total: 3, Counts: []ChannelCount{{"email", "pending", 1}, {"sms", "sent", 2}}}
	tests := []struct {
		name string
		a, b DeliverySummary
		want bool
	}{
		{"identical", base, base, true},
		{"only the read time differs", base, later, true},
		{"status differs", base, status, false},
		{"a count differs", base, count, false},
		{"a channel differs", base, other, false},
		{"the number of groups differs", base, shorter, false},
		{"empty equals empty", DeliverySummary{Status: "scheduled"}, DeliverySummary{Status: "scheduled", Counts: []ChannelCount{}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := sameState(tt.a, tt.b); got != tt.want {
				t.Errorf("sameState = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTerminal(t *testing.T) {
	t.Parallel()
	for status, want := range map[string]bool{"scheduled": false, "sending": false, "completed": true, "cancelled": true, "": false} {
		if got := terminal(status); got != want {
			t.Errorf("terminal(%q) = %v, want %v", status, got, want)
		}
	}
}
