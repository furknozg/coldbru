package progress

import "time"

// Wire contract with Notification (D-035, D-080): the shapes below mirror
// internal/notification/progress/wire.go field for field (same JSON names). Proposal does not
// import Notification's packages; wire_contract_test.go fails when the two drift apart.

// Event types of Notification that Proposal consumes (envelope "type") and their version.
const (
	EventTypeStarted   = "notification.started"
	EventTypeCompleted = "notification.completed"
	EventVersion       = 1
)

// Stream, durable consumer and subject filter (D-080).
const (
	Stream   = "EVENTS"
	Durable  = "proposal-notification-events"
	Filter   = "evt.notification.>"
	maxAcked = 100
)

// StartedData is the data of evt.notification.started.v1.
type StartedData struct {
	CampaignID string    `json:"campaign_id"`
	ScheduleID string    `json:"schedule_id"`
	Language   string    `json:"language"`
	Timezone   string    `json:"timezone"`
	StartedAt  time.Time `json:"started_at"`
}

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
