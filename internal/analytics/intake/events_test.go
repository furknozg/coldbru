package intake

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/analytics/pricing"
	"github.com/furknozg/coldbru/internal/platform/events"
)

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func handler() *Events { return &Events{Logger: quiet(), Prices: pricing.Defaults()} }

func envelope(typ string, version int, data string) events.Envelope {
	return events.Envelope{
		ID: uuid.NewString(), Type: typ, Version: version, OccurredAt: time.Now(), Producer: "test",
		CorrelationID: uuid.NewString(), Data: json.RawMessage(data),
	}
}

const (
	campaign = "0198a2c4-0000-7000-8000-000000000001"
	call     = "0198a2c4-0000-7000-8000-000000000002"
)

func completedJSON(mutate func(m map[string]any)) string {
	m := map[string]any{
		"campaign_id": campaign, "completed_at": "2026-10-29T09:00:00Z", "sms_segments": 6, "estimated_cost_usd": "0.2420",
		"counts": []any{
			map[string]any{"channel": "sms", "status": "sent", "count": 3},
			map[string]any{"channel": "email", "status": "failed", "count": 1},
		},
		"future_field": "ignored",
	}
	if mutate != nil {
		mutate(m)
	}
	raw, _ := json.Marshal(m)
	return string(raw)
}

func tokenJSON(mutate func(m map[string]any)) string {
	m := map[string]any{
		"llm_call_id": call, "campaign_id": campaign, "step_id": nil, "purpose": "screen", "model": "claude-haiku-5-5",
		"prompt_name": "screen", "prompt_version": "1.0.0",
		"tokens":  map[string]any{"input": 100, "output": 20, "cache_read": 0, "cache_creation": 0},
		"outcome": "accepted", "latency_ms": 640, "created_at": "2026-10-29T09:00:00Z",
	}
	if mutate != nil {
		mutate(m)
	}
	raw, _ := json.Marshal(m)
	return string(raw)
}

func TestHandleIgnoresWhatItDoesNotKnow(t *testing.T) {
	// A nil transaction proves that nothing is read or written.
	for name, env := range map[string]events.Envelope{
		"confirmed":      envelope("proposal.confirmed", 1, `{}`),
		"started":        envelope("notification.started", 1, `{}`),
		"future version": envelope(EventTypeCompleted, 2, `not even json`),
		"version zero":   envelope(EventTypeTokenCompleted, 0, `{}`),
	} {
		t.Run(name, func(t *testing.T) {
			if err := handler().Handle(context.Background(), nil, env); err != nil {
				t.Errorf("Handle() error = %v, want nil", err)
			}
		})
	}
}

func TestHandleRejectsMalformedCompleted(t *testing.T) {
	tests := map[string]string{
		"not json":             `{"campaign_id":`,
		"campaign not a uuid":  completedJSON(func(m map[string]any) { m["campaign_id"] = "c1" }),
		"no completed_at":      completedJSON(func(m map[string]any) { delete(m, "completed_at") }),
		"negative segments":    completedJSON(func(m map[string]any) { m["sms_segments"] = -1 }),
		"segments too large":   completedJSON(func(m map[string]any) { m["sms_segments"] = int64(1) << 40 }),
		"cost not money":       completedJSON(func(m map[string]any) { m["estimated_cost_usd"] = "cheap" }),
		"cost negative":        completedJSON(func(m map[string]any) { m["estimated_cost_usd"] = "-1.0000" }),
		"cost too many digits": completedJSON(func(m map[string]any) { m["estimated_cost_usd"] = "1.0000001" }),
		"cost too big":         completedJSON(func(m map[string]any) { m["estimated_cost_usd"] = "123456789.00" }),
		"old gbp field only":   completedJSON(func(m map[string]any) { delete(m, "estimated_cost_usd"); m["estimated_cost_gbp"] = "0.2400" }),
		"unknown channel": completedJSON(func(m map[string]any) {
			m["counts"] = []any{map[string]any{"channel": "push", "status": "sent", "count": 1}}
		}),
		"unknown status": completedJSON(func(m map[string]any) {
			m["counts"] = []any{map[string]any{"channel": "sms", "status": "lost", "count": 1}}
		}),
		"negative count": completedJSON(func(m map[string]any) {
			m["counts"] = []any{map[string]any{"channel": "sms", "status": "sent", "count": -1}}
		}),
		"duplicate pair": completedJSON(func(m map[string]any) {
			m["counts"] = []any{
				map[string]any{"channel": "sms", "status": "sent", "count": 1}, map[string]any{"channel": "sms", "status": "sent", "count": 2},
			}
		}),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			err := handler().Handle(context.Background(), nil, envelope(EventTypeCompleted, 1, data))
			if err == nil || !strings.HasPrefix(err.Error(), "malformed message:") {
				t.Errorf("Handle() error = %v, want a malformed-message error", err)
			}
		})
	}
}

func TestHandleRejectsMalformedTokenCompleted(t *testing.T) {
	tests := map[string]string{
		"not json":             `[`,
		"call not a uuid":      tokenJSON(func(m map[string]any) { m["llm_call_id"] = "x" }),
		"campaign not a uuid":  tokenJSON(func(m map[string]any) { m["campaign_id"] = "" }),
		"blank model":          tokenJSON(func(m map[string]any) { m["model"] = "  " }),
		"blank prompt name":    tokenJSON(func(m map[string]any) { m["prompt_name"] = "" }),
		"blank prompt version": tokenJSON(func(m map[string]any) { delete(m, "prompt_version") }),
		"unknown purpose":      tokenJSON(func(m map[string]any) { m["purpose"] = "translate" }),
		"unknown outcome":      tokenJSON(func(m map[string]any) { m["outcome"] = "timeout" }),
		"negative input":       tokenJSON(func(m map[string]any) { m["tokens"] = map[string]any{"input": -1} }),
		"huge output":          tokenJSON(func(m map[string]any) { m["tokens"] = map[string]any{"output": int64(1) << 40} }),
		"negative latency":     tokenJSON(func(m map[string]any) { m["latency_ms"] = -5 }),
		"no created_at":        tokenJSON(func(m map[string]any) { delete(m, "created_at") }),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			err := handler().Handle(context.Background(), nil, envelope(EventTypeTokenCompleted, 1, data))
			if err == nil || !strings.HasPrefix(err.Error(), "malformed message:") {
				t.Errorf("Handle() error = %v, want a malformed-message error", err)
			}
		})
	}
}

func TestValidAcceptsEveryPurposeAndOutcome(t *testing.T) {
	for _, p := range purposes {
		for _, o := range outcomes {
			var d TokenCompletedData
			if err := json.Unmarshal([]byte(tokenJSON(func(m map[string]any) { m["purpose"] = p; m["outcome"] = o })), &d); err != nil {
				t.Fatal(err)
			}
			if _, _, err := d.validate(); err != nil {
				t.Errorf("%s/%s: %v", p, o, err)
			}
		}
	}
}

func TestDecodeKeepsTheStepIDNullable(t *testing.T) {
	var d TokenCompletedData
	if err := json.Unmarshal([]byte(tokenJSON(nil)), &d); err != nil || d.StepID != nil {
		t.Fatalf("null step_id: %+v, %v", d.StepID, err)
	}
	if err := json.Unmarshal([]byte(tokenJSON(func(m map[string]any) { m["step_id"] = call })), &d); err != nil || d.StepID == nil || *d.StepID != call {
		t.Fatalf("step_id: %v, %v", d.StepID, err)
	}
	if d.Tokens != (TokenCounts{Input: 100, Output: 20}) || d.LatencyMs != 640 {
		t.Errorf("decoded %+v", d)
	}
}
