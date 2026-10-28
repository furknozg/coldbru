package flow

import (
	"encoding/json"
	"time"
)

// Wire contract of the NATS messages Proposal publishes through the outbox (D-035, D-074,
// docs/domain-model.md §6). Notification decodes the same shapes.

// Event types (envelope "type", subject evt.<type>.v1) and the producer name.
const (
	EventProducer           = "proposal"
	EventTypeConfirmed      = "proposal.confirmed"
	EventTypeCancelled      = "proposal.cancelled"
	EventVersion            = 1
	RecipientsSubjectPrefix = "recipients." // + campaign id
)

// RecipientsSubject is the subject of the recipient batches of a campaign (RECIPIENTS stream).
func RecipientsSubject(campaignID string) string { return RecipientsSubjectPrefix + campaignID }

// ConfirmedSchedule is the send time of one language. SendAt is nil for "send immediately": the
// receiver sends when it processes the event.
type ConfirmedSchedule struct {
	Language string     `json:"language"`
	SendAt   *time.Time `json:"send_at"`
}

// ConfirmedMessage is the approved text of one (language, channel) slot. Content is the variant
// content as stored: {"text", ...} for an SMS, the email blocks for an email (D-071).
type ConfirmedMessage struct {
	Language string          `json:"language"`
	Channel  string          `json:"channel"`
	Content  json.RawMessage `json:"content"`
}

// ConfirmedData is the data of evt.proposal.confirmed.v1. CTAURL is the email button link with
// utm_campaign merged in; RecipientBatches is how many messages on recipients.<campaign_id> carry
// the recipients.
type ConfirmedData struct {
	CampaignID       string              `json:"campaign_id"`
	Schedules        []ConfirmedSchedule `json:"schedules"`
	Messages         []ConfirmedMessage  `json:"messages"`
	CTAURL           string              `json:"cta_url"`
	RecipientBatches int                 `json:"recipient_batches"`
	RecipientCounts  Recipients          `json:"recipient_counts"`
}

// CancelledData is the data of evt.proposal.cancelled.v1.
type CancelledData struct {
	CampaignID string `json:"campaign_id"`
	Reason     string `json:"reason"`
}

// BatchRecipient is one person on one channel. Contact is the email address for channel "email"
// and the phone number for channel "sms".
type BatchRecipient struct {
	CustomerID string `json:"customer_id"`
	FirstName  string `json:"first_name"`
	Language   string `json:"language"`
	Channel    string `json:"channel"`
	Contact    string `json:"contact"`
}

// RecipientBatch is one message of the RECIPIENTS stream; BatchNo counts from 1.
type RecipientBatch struct {
	CampaignID string           `json:"campaign_id"`
	BatchNo    int              `json:"batch_no"`
	BatchCount int              `json:"batch_count"`
	Recipients []BatchRecipient `json:"recipients"`
}

// MaxBatchRecipients is the most recipients one RecipientBatch carries (D-074).
const MaxBatchRecipients = 500
