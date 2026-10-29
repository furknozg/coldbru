package intake_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/furknozg/coldbru/internal/notification/intake"
	"github.com/furknozg/coldbru/internal/proposal/flow"
)

// jsonFields returns the JSON field names of struct type t.
func jsonFields(t reflect.Type) []string {
	var out []string
	for i := range t.NumField() {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// TestWireMirrorsProposal fails when Proposal's wire types gain or lose a field that this package's
// mirror does not know about, so the two cannot drift apart unnoticed (D-035, D-074).
func TestWireMirrorsProposal(t *testing.T) {
	tests := []struct {
		name     string
		proposal reflect.Type
		mirror   reflect.Type
		ignored  []string // Proposal fields Notification does not read
	}{
		{"confirmed data", reflect.TypeFor[flow.ConfirmedData](), reflect.TypeFor[intake.ConfirmedData](), []string{"recipient_counts"}},
		{"confirmed schedule", reflect.TypeFor[flow.ConfirmedSchedule](), reflect.TypeFor[intake.ConfirmedSchedule](), nil},
		{"confirmed message", reflect.TypeFor[flow.ConfirmedMessage](), reflect.TypeFor[intake.ConfirmedMessage](), nil},
		{"cancelled data", reflect.TypeFor[flow.CancelledData](), reflect.TypeFor[intake.CancelledData](), nil},
		{"recipient batch", reflect.TypeFor[flow.RecipientBatch](), reflect.TypeFor[intake.Batch](), nil},
		{"batch recipient", reflect.TypeFor[flow.BatchRecipient](), reflect.TypeFor[intake.Recipient](), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := jsonFields(tt.proposal)
			want = slices.DeleteFunc(want, func(f string) bool { return slices.Contains(tt.ignored, f) })
			if got := jsonFields(tt.mirror); !slices.Equal(got, want) {
				t.Errorf("mirror fields = %v, Proposal fields (minus ignored %v) = %v", got, tt.ignored, want)
			}
		})
	}
}

func TestWireDecodesWhatProposalPublishes(t *testing.T) {
	send := time.Date(2026, 10, 30, 6, 0, 0, 0, time.UTC)
	sent := flow.ConfirmedData{
		CampaignID: "0199f3a1-0000-7000-8000-000000000001",
		Schedules: []flow.ConfirmedSchedule{
			{Language: "tr", Timezone: "Europe/Istanbul", SendAt: &send},
			{Language: "en", Timezone: "Europe/London"},
		},
		Messages:         []flow.ConfirmedMessage{{Language: "tr", Channel: "sms", Content: json.RawMessage(`{"text":"Merhaba"}`)}},
		CTAURL:           "https://shop.example.com",
		RecipientBatches: 2,
		RecipientCounts:  flow.Recipients{Email: 1, SMS: 2, Total: 3},
	}
	var got intake.ConfirmedData
	if err := json.Unmarshal(mustMarshal(t, sent), &got); err != nil {
		t.Fatalf("decode confirmed data: %v", err)
	}
	if got.CampaignID != sent.CampaignID || got.CTAURL != sent.CTAURL || got.RecipientBatches != 2 ||
		len(got.Schedules) != 2 || got.Schedules[0].SendAt == nil || !got.Schedules[0].SendAt.Equal(send) || got.Schedules[1].SendAt != nil ||
		got.Schedules[0].Language != "tr" || got.Schedules[0].Timezone != "Europe/Istanbul" ||
		len(got.Messages) != 1 || got.Messages[0].Channel != "sms" || string(got.Messages[0].Content) != `{"text":"Merhaba"}` {
		t.Errorf("decoded confirmed data = %+v", got)
	}

	var cancelled intake.CancelledData
	if err := json.Unmarshal(mustMarshal(t, flow.CancelledData{CampaignID: "c", Reason: "r"}), &cancelled); err != nil || cancelled.CampaignID != "c" || cancelled.Reason != "r" {
		t.Errorf("decoded cancelled data = %+v, %v", cancelled, err)
	}

	b := flow.RecipientBatch{CampaignID: "c", BatchNo: 2, BatchCount: 3, Recipients: []flow.BatchRecipient{
		{CustomerID: "CUS-1", FirstName: "Ada", Language: "tr", Timezone: "Europe/Istanbul", Channel: "sms", Contact: "+905551112233"},
	}}
	var batch intake.Batch
	if err := json.Unmarshal(mustMarshal(t, b), &batch); err != nil {
		t.Fatalf("decode batch: %v", err)
	}
	if batch.CampaignID != "c" || batch.BatchNo != 2 || batch.BatchCount != 3 || len(batch.Recipients) != 1 ||
		batch.Recipients[0] != (intake.Recipient{CustomerID: "CUS-1", FirstName: "Ada", Language: "tr", Timezone: "Europe/Istanbul", Channel: "sms", Contact: "+905551112233"}) {
		t.Errorf("decoded batch = %+v", batch)
	}
}

func TestWireConstantsMatchProposal(t *testing.T) {
	if intake.EventTypeConfirmed != flow.EventTypeConfirmed || intake.EventTypeCancelled != flow.EventTypeCancelled || intake.EventVersion != flow.EventVersion {
		t.Errorf("event types/version differ: %s %s %d vs %s %s %d",
			intake.EventTypeConfirmed, intake.EventTypeCancelled, intake.EventVersion, flow.EventTypeConfirmed, flow.EventTypeCancelled, flow.EventVersion)
	}
	if !strings.HasPrefix(flow.RecipientsSubject("x"), "recipients.") || intake.RecipientsFilter != "recipients.>" {
		t.Errorf("recipients subject %q vs filter %q", flow.RecipientsSubject("x"), intake.RecipientsFilter)
	}
	if intake.EventsFilter != "evt.proposal.>" {
		t.Errorf("events filter = %q", intake.EventsFilter)
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}
