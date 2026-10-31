package intake

import "time"

// Wire contract (D-035, D-080, D-083, D-108): the shapes below mirror the events other services publish,
// field for field and with the same JSON names. Analytics does not import their packages;
// contract tests in this package's _test files import them and fail when the two drift apart.
// Unknown fields are ignored (D-060).

// Event types Analytics handles (envelope "type"), their version, and the subject filter of its
// consumer. Every other event on EVENTS is acknowledged and ignored.
const (
	EventTypeCompleted      = "notification.completed"
	EventTypeTokenCompleted = "proposal.token_completed"
	EventTypeClicked        = "redirect.clicked"
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

// LocaleCount is the number of deliveries sent in one channel to one (language, timezone) slot.
type LocaleCount struct {
	Channel  string `json:"channel"`
	Language string `json:"language"`
	Timezone string `json:"timezone"`
	Sent     int64  `json:"sent"`
}

// CompletedData is the data of evt.notification.completed.v1. SentByLocale is absent in events
// published before D-108 added it.
type CompletedData struct {
	CampaignID       string        `json:"campaign_id"`
	CompletedAt      time.Time     `json:"completed_at"`
	Counts           []Count       `json:"counts"`
	SMSSegments      int64         `json:"sms_segments"`
	EstimatedCostUSD string        `json:"estimated_cost_usd"`
	SentByLocale     []LocaleCount `json:"sent_by_locale"`
}

// ClickedData is the data of evt.redirect.clicked.v1 (D-107, D-108).
type ClickedData struct {
	ClickID    string    `json:"click_id"`
	DeliveryID string    `json:"delivery_id"`
	CampaignID string    `json:"campaign_id"`
	Channel    string    `json:"channel"`
	Language   string    `json:"language"`
	Timezone   string    `json:"timezone"`
	ClickedAt  time.Time `json:"clicked_at"`
	First      bool      `json:"first"`
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
