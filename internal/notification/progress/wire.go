package progress

import (
	"time"
)

// Events Notification publishes about the progress of a campaign (D-035, D-080, docs/domain-model.md
// §6). Proposal mirrors these shapes in internal/proposal/progress; a contract test there fails when
// they drift apart.

// Event types (envelope "type", subject evt.<type>.v1), the producer name and the version.
const (
	EventProducer      = "notification"
	EventTypeStarted   = "notification.started"
	EventTypeCompleted = "notification.completed"
	EventVersion       = 1
)

// StartedData is the data of evt.notification.started.v1: one (language, timezone) schedule of the
// campaign fired. It is written once per firing, in the transaction that fires the schedule (D-080).
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

// LocaleCount is the number of deliveries that were sent in one channel to the recipients of one
// (language, timezone) send slot (D-108). Together they are the denominators of the click-through
// rate; the entries add up to the sent count of the same channel in Counts.
type LocaleCount struct {
	Channel  string `json:"channel"`
	Language string `json:"language"`
	Timezone string `json:"timezone"`
	Sent     int64  `json:"sent"`
}

// CompletedData is the data of evt.notification.completed.v1: the campaign finished, every
// delivery is final (D-080). SMSSegments is the sum of the stored segment counts of the sent SMS
// (an estimate, D-071), EstimatedCostUSD is those segments at the SMS segment price plus the sent
// emails at the email price, a decimal string with four decimals like content.Money. SentByLocale
// was added to version 1 later (D-108): an additive field, absent in events published before it,
// one entry per (channel, language, timezone) with at least one sent delivery, ordered by channel,
// language and timezone; empty (never null) when nothing was sent.
type CompletedData struct {
	CampaignID       string        `json:"campaign_id"`
	CompletedAt      time.Time     `json:"completed_at"`
	Counts           []Count       `json:"counts"`
	SMSSegments      int64         `json:"sms_segments"`
	EstimatedCostUSD string        `json:"estimated_cost_usd"`
	SentByLocale     []LocaleCount `json:"sent_by_locale"`
}
