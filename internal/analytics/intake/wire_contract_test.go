package intake

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	nprogress "github.com/furknozg/coldbru/internal/notification/progress"
	"github.com/furknozg/coldbru/internal/proposal/flow"
)

// The producers' wire shapes (D-035, D-080, D-083) and Analytics' mirror of them must stay the
// same. These tests import the producing packages, which production code of this package never does.

func TestContractWithNotificationCompleted(t *testing.T) {
	if EventTypeCompleted != nprogress.EventTypeCompleted || EventVersion != nprogress.EventVersion {
		t.Errorf("event type/version = %s v%d, notification sends %s v%d", EventTypeCompleted, EventVersion, nprogress.EventTypeCompleted, nprogress.EventVersion)
	}
	at := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	sent := nprogress.CompletedData{
		CampaignID: "c", CompletedAt: at, SMSSegments: 6, EstimatedCostUSD: "0.2400",
		Counts: []nprogress.Count{{Channel: "sms", Status: "sent", Count: 3}, {Channel: "email", Status: "failed", Count: 1}},
	}
	raw, err := json.Marshal(sent)
	if err != nil {
		t.Fatal(err)
	}
	var got CompletedData
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	want := CompletedData{
		CampaignID: "c", CompletedAt: at, SMSSegments: 6, EstimatedCostUSD: "0.2400",
		Counts: []Count{{Channel: "sms", Status: "sent", Count: 3}, {Channel: "email", Status: "failed", Count: 1}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("decoded %+v, want %+v", got, want)
	}
	if jsonNames(t, sent) != jsonNames(t, want) {
		t.Errorf("JSON names differ: notification %s, analytics %s", jsonNames(t, sent), jsonNames(t, want))
	}
}

func TestContractWithProposalTokenCompleted(t *testing.T) {
	if EventTypeTokenCompleted != flow.EventTypeTokenCompleted || EventVersion != flow.EventVersion {
		t.Errorf("event type/version = %s v%d, proposal sends %s v%d", EventTypeTokenCompleted, EventVersion, flow.EventTypeTokenCompleted, flow.EventVersion)
	}
	step := "0198a2c4-0000-7000-8000-000000000003"
	at := time.Date(2026, 10, 29, 9, 0, 0, 0, time.UTC)
	sent := flow.TokenCompletedData{
		LLMCallID: "l", CampaignID: "c", StepID: &step, Purpose: "content", Model: "claude-sonnet-5-5",
		PromptName: "content", PromptVersion: "1.2.0", Outcome: "validation_failed", LatencyMs: 812, CreatedAt: at,
		Tokens: flow.TokenCounts{Input: 1, Output: 2, CacheRead: 3, CacheCreation: 4},
	}
	raw, err := json.Marshal(sent)
	if err != nil {
		t.Fatal(err)
	}
	var got TokenCompletedData
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	want := TokenCompletedData{
		LLMCallID: "l", CampaignID: "c", StepID: &step, Purpose: "content", Model: "claude-sonnet-5-5",
		PromptName: "content", PromptVersion: "1.2.0", Outcome: "validation_failed", LatencyMs: 812, CreatedAt: at,
		Tokens: TokenCounts{Input: 1, Output: 2, CacheRead: 3, CacheCreation: 4},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("decoded %+v, want %+v", got, want)
	}
	if jsonNames(t, sent) != jsonNames(t, want) {
		t.Errorf("JSON names differ: proposal %s, analytics %s", jsonNames(t, sent), jsonNames(t, want))
	}
}

// jsonNames lists the JSON object keys of v, recursively, sorted by encoding/json's map order.
func jsonNames(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(shape(generic))
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// shape keeps the keys of a decoded JSON value and replaces every leaf by its kind.
func shape(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			out[k] = shape(e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = shape(e)
		}
		return out
	case nil:
		return "string" // a null pointer field stands for its string form
	default:
		return reflect.TypeOf(v).Kind().String()
	}
}
