package progress

import (
	"encoding/json"
	"testing"

	"github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/proposal/flow"
)

func TestNext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		current string
		typ     string
		want    string
		moves   bool
	}{
		{"started moves scheduled to sending", flow.CampaignStatusScheduled, EventTypeStarted, flow.CampaignStatusSending, true},
		{"a second started is a no-op", flow.CampaignStatusSending, EventTypeStarted, "", false},
		{"started after completed is a no-op", flow.CampaignStatusCompleted, EventTypeStarted, "", false},
		{"started for a cancelled campaign is ignored", flow.CampaignStatusCancelled, EventTypeStarted, "", false},
		{"started for a draft is ignored", flow.CampaignStatusDrafting, EventTypeStarted, "", false},
		{"completed moves sending to completed", flow.CampaignStatusSending, EventTypeCompleted, flow.CampaignStatusCompleted, true},
		{"completed moves scheduled to completed", flow.CampaignStatusScheduled, EventTypeCompleted, flow.CampaignStatusCompleted, true},
		{"a second completed is a no-op", flow.CampaignStatusCompleted, EventTypeCompleted, "", false},
		{"completed for a cancelled campaign is ignored", flow.CampaignStatusCancelled, EventTypeCompleted, "", false},
		{"completed for a draft is ignored", flow.CampaignStatusDrafting, EventTypeCompleted, "", false},
		{"an unknown type changes nothing", flow.CampaignStatusScheduled, "notification.other", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := next(tt.current, tt.typ)
			if got != tt.want || ok != tt.moves {
				t.Errorf("next(%q, %q) = %q, %v; want %q, %v", tt.current, tt.typ, got, ok, tt.want, tt.moves)
			}
		})
	}
}

func TestCampaignOf(t *testing.T) {
	t.Parallel()
	const id = "0199f3a1-0000-7000-8000-000000000001"
	tests := []struct {
		name string
		data string
		ok   bool
	}{
		{"valid", `{"campaign_id":"` + id + `","extra":1}`, true},
		{"not a uuid", `{"campaign_id":"nope"}`, false},
		{"missing", `{}`, false},
		{"not json", `[`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := campaignOf(events.Envelope{Data: json.RawMessage(tt.data)})
			if (err == nil) != tt.ok || (tt.ok && got.String() != id) {
				t.Errorf("campaignOf(%s) = %v, %v", tt.data, got, err)
			}
		})
	}
}

func TestConsumerConfig(t *testing.T) {
	t.Parallel()
	cfg := ConsumerConfig()
	if cfg.Durable != "proposal-notification-events" || cfg.FilterSubject != "evt.notification.>" {
		t.Errorf("consumer = %s on %s", cfg.Durable, cfg.FilterSubject)
	}
}

func TestStashIsTakenOnce(t *testing.T) {
	t.Parallel()
	c, err := New(Deps{Pool: nonNilPool(t)})
	if err != nil {
		t.Fatal(err)
	}
	c.stash("e1", flow.CampaignStatus{Status: "sending"})
	if got := c.take("e1"); len(got) != 1 {
		t.Fatalf("take = %v", got)
	}
	if got := c.take("e1"); len(got) != 0 {
		t.Errorf("second take = %v, want none", got)
	}
}
