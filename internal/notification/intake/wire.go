package intake

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Wire contract with Proposal (D-035, D-074, D-076, D-078): the shapes below mirror
// internal/proposal/flow/wire.go field for field (same JSON names). Fields Notification does not
// use are left out (confirmed.recipient_counts); wire_contract_test.go fails when Proposal's types
// gain or lose a field this mirror does not know about.

// Event types of Proposal that Notification consumes (envelope "type") and their version.
const (
	EventTypeConfirmed = "proposal.confirmed"
	EventTypeCancelled = "proposal.cancelled"
	EventVersion       = 1
)

// Subjects. A schedule is published on sched.<schedule_id> with Nats-Schedule headers and the server
// republishes it on fire.<schedule_id> (the target) when due; with no send time it goes to the fire
// subject directly. Both are in the SCHEDULES stream. A delivery is queued on send.<channel> (SEND).
const (
	ScheduleSubjectPrefix = "sched."
	FireSubjectPrefix     = "fire."
	SendSubjectPrefix     = "send."
	RecipientsFilter      = "recipients.>"
	FireFilter            = "fire.>"
	EventsFilter          = "evt.proposal.>"
)

// ScheduleSubject is where the schedule of a (language, timezone) slot is published when it has a send time.
func ScheduleSubject(scheduleID string) string { return ScheduleSubjectPrefix + scheduleID }

// FireSubject is where a schedule firing is delivered to the SCHEDULES consumer.
func FireSubject(scheduleID string) string { return FireSubjectPrefix + scheduleID }

// SendSubject is the SEND subject of a channel.
func SendSubject(channel string) string { return SendSubjectPrefix + channel }

// Languages and channels the notification_db CHECK constraints accept (D-077).
var (
	languages = map[string]bool{"tr": true, "en": true}
	channels  = map[string]bool{"sms": true, "email": true}
)

// ConfirmedData is the data of evt.proposal.confirmed.v1.
type ConfirmedData struct {
	CampaignID       string              `json:"campaign_id"`
	Schedules        []ConfirmedSchedule `json:"schedules"`
	Messages         []ConfirmedMessage  `json:"messages"`
	CTAURL           string              `json:"cta_url"`
	RecipientBatches int                 `json:"recipient_batches"`
}

// ConfirmedSchedule is the send time of one (language, timezone) slot; SendAt nil means "send as
// soon as the event is processed".
type ConfirmedSchedule struct {
	Language string     `json:"language"`
	Timezone string     `json:"timezone"`
	SendAt   *time.Time `json:"send_at"`
}

// ConfirmedMessage is the approved content of one (language, channel) slot.
type ConfirmedMessage struct {
	Language string          `json:"language"`
	Channel  string          `json:"channel"`
	Content  json.RawMessage `json:"content"`
}

// CancelledData is the data of evt.proposal.cancelled.v1.
type CancelledData struct {
	CampaignID string `json:"campaign_id"`
	Reason     string `json:"reason"`
}

// Batch is one message of the RECIPIENTS stream.
type Batch struct {
	CampaignID string      `json:"campaign_id"`
	BatchNo    int         `json:"batch_no"`
	BatchCount int         `json:"batch_count"`
	Recipients []Recipient `json:"recipients"`
}

// Recipient is one person on one channel. Contact is the address for "email", the number for "sms".
type Recipient struct {
	CustomerID string `json:"customer_id"`
	FirstName  string `json:"first_name"`
	Language   string `json:"language"`
	Timezone   string `json:"timezone"`
	Channel    string `json:"channel"`
	Contact    string `json:"contact"`
}

// FirePayload is the payload of the message on sched.<schedule_id> and fire.<schedule_id>, written
// by the confirmed-event handler and read by the schedule consumer.
type FirePayload struct {
	ScheduleID string `json:"schedule_id"`
	CampaignID string `json:"campaign_id"`
}

// SendPayload is the payload of a message on send.<channel>: the delivery to send (D-078).
type SendPayload struct {
	DeliveryID string `json:"delivery_id"`
}

// validate checks the confirmed event data and returns the campaign id. Anything it rejects could
// never be stored (the notification_db constraints would refuse it), so the caller terminates the
// event instead of retrying.
func (d *ConfirmedData) validate() (uuid.UUID, error) {
	id, err := uuid.Parse(d.CampaignID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("campaign_id %q is not a uuid", d.CampaignID)
	}
	if strings.TrimSpace(d.CTAURL) == "" {
		return uuid.Nil, errors.New("cta_url is empty")
	}
	if d.RecipientBatches < 1 || d.RecipientBatches > math.MaxInt32 {
		return uuid.Nil, fmt.Errorf("recipient_batches = %d, want 1 to %d", d.RecipientBatches, math.MaxInt32)
	}
	if len(d.Schedules) == 0 {
		return uuid.Nil, errors.New("no schedules")
	}
	if len(d.Messages) == 0 {
		return uuid.Nil, errors.New("no messages")
	}
	seenSchedule := map[[2]string]bool{}
	for i, s := range d.Schedules {
		if !languages[s.Language] {
			return uuid.Nil, fmt.Errorf("schedules[%d]: language %q is not supported", i, s.Language)
		}
		if strings.TrimSpace(s.Timezone) == "" {
			return uuid.Nil, fmt.Errorf("schedules[%d]: timezone is empty", i)
		}
		key := [2]string{s.Language, s.Timezone}
		if seenSchedule[key] {
			return uuid.Nil, fmt.Errorf("schedules[%d]: (%s, %s) appears twice", i, s.Language, s.Timezone)
		}
		seenSchedule[key] = true
	}
	seenMessage := map[[2]string]bool{}
	for i, m := range d.Messages {
		if !languages[m.Language] {
			return uuid.Nil, fmt.Errorf("messages[%d]: language %q is not supported", i, m.Language)
		}
		if !channels[m.Channel] {
			return uuid.Nil, fmt.Errorf("messages[%d]: channel %q is not supported", i, m.Channel)
		}
		if !json.Valid(m.Content) || strings.TrimSpace(string(m.Content)) == "null" {
			return uuid.Nil, fmt.Errorf("messages[%d]: content is not a JSON value", i)
		}
		key := [2]string{m.Language, m.Channel}
		if seenMessage[key] {
			return uuid.Nil, fmt.Errorf("messages[%d]: (%s, %s) appears twice", i, m.Language, m.Channel)
		}
		seenMessage[key] = true
	}
	return id, nil
}

// scheduleHeader is the Nats-Schedule value that fires at t. NATS parses whole seconds; a time with
// a fraction is rounded up so the message is never sent early.
func scheduleHeader(t time.Time) string {
	t = t.UTC()
	if t.Nanosecond() != 0 {
		t = t.Truncate(time.Second).Add(time.Second)
	}
	return "@at " + t.Format(time.RFC3339)
}
