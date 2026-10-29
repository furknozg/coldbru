//go:build integration

package flow_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"

	platevents "github.com/furknozg/coldbru/internal/platform/events"
	"github.com/furknozg/coldbru/internal/proposal/flow"
)

// requireTokenEvents asserts D-083: every llm_call row of the campaign has exactly one
// evt.proposal.token_completed.v1 row in the outbox, written with it, whose data matches the row
// field by field. The envelope carries the campaign id as its correlation id.
func requireTokenEvents(t *testing.T, h *harness, campaign uuid.UUID) {
	t.Helper()
	rows, err := h.q.ListLLMCalls(t.Context(), campaign)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("the campaign has no llm_call rows")
	}
	sub, err := platevents.Subject(platevents.Envelope{Type: flow.EventTypeTokenCompleted, Version: flow.EventVersion})
	if err != nil {
		t.Fatal(err)
	}
	res, err := h.pool.Query(t.Context(), `SELECT payload FROM outbox WHERE subject = $1 AND payload->>'correlation_id' = $2`, sub, campaign.String())
	if err != nil {
		t.Fatal(err)
	}
	defer res.Close()
	got := map[string]flow.TokenCompletedData{}
	for res.Next() {
		var raw []byte
		if err := res.Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var env platevents.Envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			t.Fatal(err)
		}
		if env.Producer != flow.EventProducer || env.Version != flow.EventVersion || env.Type != flow.EventTypeTokenCompleted {
			t.Errorf("envelope = %+v", env)
		}
		var d flow.TokenCompletedData
		if err := json.Unmarshal(env.Data, &d); err != nil {
			t.Fatalf("decode %s: %v", env.Data, err)
		}
		if _, dup := got[d.LLMCallID]; dup {
			t.Errorf("two events for llm_call %s", d.LLMCallID)
		}
		got[d.LLMCallID] = d
	}
	if err := res.Err(); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(rows) {
		t.Fatalf("%d token_completed events for %d llm_call rows", len(got), len(rows))
	}
	for _, r := range rows {
		d, ok := got[r.ID.String()]
		if !ok {
			t.Errorf("llm_call %s has no event", r.ID)
			continue
		}
		wantStep := ""
		if r.StepID != nil {
			wantStep = r.StepID.String()
		}
		gotStep := ""
		if d.StepID != nil {
			gotStep = *d.StepID
		}
		if d.CampaignID != r.CampaignID.String() || gotStep != wantStep || d.Purpose != r.Purpose || d.Model != r.Model ||
			d.PromptName != r.PromptName || d.PromptVersion != r.PromptVersion || d.Outcome != r.Outcome ||
			d.LatencyMs != int64(r.LatencyMs) || !d.CreatedAt.Equal(r.CreatedAt.Time) ||
			d.Tokens != (flow.TokenCounts{
				Input: int64(r.InputTokens), Output: int64(r.OutputTokens),
				CacheRead: int64(r.CacheReadTokens), CacheCreation: int64(r.CacheCreationTokens),
			}) {
			t.Errorf("event %+v does not match row %+v", d, r)
		}
		if (r.StepID == nil) != (d.StepID == nil) {
			t.Errorf("step_id of event %v, of row %v", d.StepID, r.StepID)
		}
	}
}
