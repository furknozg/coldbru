package intake

import "time"

// Wire contract (D-035, D-080, D-083): the shapes below mirror the events other services publish,
// field for field and with the same JSON names. Analytics does not import their packages;
// contract tests in this package's _test files import them and fail when the two drift apart.
// Unknown fields are ignored (D-060).

// Event types Analytics handles (envelope "type"), their version, and the subject filter of its
// consumer. Every other event on EVENTS is acknowledged and ignored.
const (
	EventTypeCompleted      = "notification.completed"
	EventTypeTokenCompleted = "proposal.token_completed"
	EventVersion            = 1
)

// Stream, durable consumer and subject filter (D-083).
const (
	Stream  = "EVENTS"
	Durable = "analytics-events"
	Filter  = "evt.>"
	// maxAckPending is the number of events in flight; every handler is idempotent and independent.
	maxAckPending = 100
)

// Count is the number of deliveries of one channel in one status.
type Count struct {
	Channel string `json:"channel"`
	Status  string `json:"status"`
	Count   int64  `json:"count"`
}

// CompletedData is the data of evt.notification.completed.v1.
type CompletedData struct {
	CampaignID       string    `json:"campaign_id"`
	CompletedAt      time.Time `json:"completed_at"`
	Counts           []Count   `json:"counts"`
	SMSSegments      int64     `json:"sms_segments"`
	EstimatedCostUSD string    `json:"estimated_cost_usd"`
}

// TokenCounts are the token usage of one model call.
type TokenCounts struct {
	Input         int64 `json:"input"`
	Output        int64 `json:"output"`
	CacheRead     int64 `json:"cache_read"`
	CacheCreation int64 `json:"cache_creation"`
}

// TokenCompletedData is the data of evt.proposal.token_completed.v1. StepID is not stored.
type TokenCompletedData struct {
	LLMCallID     string      `json:"llm_call_id"`
	CampaignID    string      `json:"campaign_id"`
	StepID        *string     `json:"step_id"`
	Purpose       string      `json:"purpose"`
	Model         string      `json:"model"`
	PromptName    string      `json:"prompt_name"`
	PromptVersion string      `json:"prompt_version"`
	Tokens        TokenCounts `json:"tokens"`
	Outcome       string      `json:"outcome"`
	LatencyMs     int64       `json:"latency_ms"`
	CreatedAt     time.Time   `json:"created_at"`
}
