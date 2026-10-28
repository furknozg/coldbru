package flow_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"

	platevents "github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/proposal/flow"
)

func keys(t *testing.T, raw []byte) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// The wire shape is a contract with Notification (D-074, docs/domain-model.md §6): field names are
// part of it, so a rename must fail a test.
func TestConfirmedDataWireShape(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	data := flow.ConfirmedData{
		CampaignID: "c1",
		Schedules:  []flow.ConfirmedSchedule{{Language: "tr", SendAt: &at}, {Language: "en"}},
		Messages:   []flow.ConfirmedMessage{{Language: "tr", Channel: "sms", Content: json.RawMessage(`{"text":"Merhaba {{first_name}}"}`)}},
		CTAURL:     "https://shop.example.com?utm_campaign=c1", RecipientBatches: 1,
		RecipientCounts: flow.Recipients{Email: 33, SMS: 13, Total: 46},
	}
	env, err := platevents.New(flow.EventTypeConfirmed, flow.EventVersion, flow.EventProducer, "c1", data)
	if err != nil {
		t.Fatal(err)
	}
	if subject, err := platevents.Subject(env); err != nil || subject != "evt.proposal.confirmed.v1" {
		t.Fatalf("subject = %q, %v", subject, err)
	}
	if env.Producer != "proposal" || env.CorrelationID != "c1" {
		t.Errorf("envelope = %+v", env)
	}
	if got, want := keys(t, env.Data), []string{"campaign_id", "cta_url", "messages", "recipient_batches", "recipient_counts", "schedules"}; !reflect.DeepEqual(got, want) {
		t.Errorf("data keys = %v, want %v", got, want)
	}
	var back struct {
		Schedules []map[string]any `json:"schedules"`
		Messages  []map[string]any `json:"messages"`
		Counts    map[string]int   `json:"recipient_counts"`
	}
	if err := json.Unmarshal(env.Data, &back); err != nil {
		t.Fatal(err)
	}
	if back.Schedules[0]["send_at"] != "2026-11-02T09:00:00Z" || back.Schedules[1]["send_at"] != nil {
		t.Errorf("schedules = %v: send_at is an RFC 3339 string, null for send immediately", back.Schedules)
	}
	if _, ok := back.Schedules[1]["send_at"]; !ok {
		t.Error("send_at must be present (null) for send immediately")
	}
	if back.Messages[0]["channel"] != "sms" || back.Messages[0]["language"] != "tr" || back.Messages[0]["content"].(map[string]any)["text"] != "Merhaba {{first_name}}" {
		t.Errorf("messages = %v", back.Messages)
	}
	if back.Counts["email"] != 33 || back.Counts["sms"] != 13 || back.Counts["total"] != 46 {
		t.Errorf("recipient_counts = %v", back.Counts)
	}
}

func TestCancelledDataWireShape(t *testing.T) {
	t.Parallel()
	env, err := platevents.New(flow.EventTypeCancelled, flow.EventVersion, flow.EventProducer, "c1", flow.CancelledData{CampaignID: "c1", Reason: "wrong text"})
	if err != nil {
		t.Fatal(err)
	}
	if subject, _ := platevents.Subject(env); subject != "evt.proposal.cancelled.v1" {
		t.Errorf("subject = %q", subject)
	}
	if got, want := keys(t, env.Data), []string{"campaign_id", "reason"}; !reflect.DeepEqual(got, want) {
		t.Errorf("data keys = %v, want %v", got, want)
	}
}

func TestRecipientBatchWireShape(t *testing.T) {
	t.Parallel()
	raw, err := json.Marshal(flow.RecipientBatch{
		CampaignID: "c1", BatchNo: 1, BatchCount: 2,
		Recipients: []flow.BatchRecipient{{CustomerID: "u1", FirstName: "Ada", Language: "tr", Channel: "email", Contact: "a@example.com"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := keys(t, raw), []string{"batch_count", "batch_no", "campaign_id", "recipients"}; !reflect.DeepEqual(got, want) {
		t.Errorf("batch keys = %v, want %v", got, want)
	}
	var b struct {
		Recipients []json.RawMessage `json:"recipients"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		t.Fatal(err)
	}
	if got, want := keys(t, b.Recipients[0]), []string{"channel", "contact", "customer_id", "first_name", "language"}; !reflect.DeepEqual(got, want) {
		t.Errorf("recipient keys = %v, want %v", got, want)
	}
	if flow.RecipientsSubject("c1") != "recipients.c1" {
		t.Errorf("RecipientsSubject = %q", flow.RecipientsSubject("c1"))
	}
}

func TestCampaignConfirmedHubEvent(t *testing.T) {
	t.Parallel()
	ev := flow.CampaignConfirmed{RecipientCounts: flow.Recipients{Email: 1, SMS: 2, Total: 3}, RecipientBatches: 1}
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	if ev.Type() != "campaign_confirmed" {
		t.Errorf("Type() = %q", ev.Type())
	}
	if got, want := keys(t, raw), []string{"at", "campaign_id", "recipient_batches", "recipient_counts"}; !reflect.DeepEqual(got, want) {
		t.Errorf("keys = %v, want %v", got, want)
	}
}
