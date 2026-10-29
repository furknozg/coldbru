package api

import (
	"time"

	"github.com/google/uuid"
)

// Campaign statuses at which a campaign has nothing left to send (the stream ends).
const (
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
)

// ChannelCount is the number of deliveries of one channel in one status.
type ChannelCount struct {
	Channel string `json:"channel" validate:"required" enums:"sms,email"`
	Status  string `json:"status" validate:"required" enums:"pending,sending,sent,failed,skipped_no_consent,cancelled"`
	Count   int64  `json:"count" validate:"required" example:"1200"`
}

// DeliverySummary is the live state of a campaign's deliveries: the response of GET
// /campaigns/{id}/deliveries and the data of every `snapshot` event.
type DeliverySummary struct {
	CampaignID uuid.UUID `json:"campaign_id" validate:"required" format:"uuid"`
	// Status is the campaign status in Notification (not Proposal's): scheduled until the first
	// schedule fires, sending, then completed, or cancelled.
	Status string `json:"status" validate:"required" enums:"scheduled,sending,completed,cancelled"`
	// Counts has one entry per (channel, status) that has at least one delivery, ordered by channel
	// and status.
	Counts []ChannelCount `json:"counts" validate:"required"`
	// Total is the sum of Counts.
	Total int64 `json:"total" validate:"required" example:"2400"`
	// UpdatedAt is the time the counts were read.
	UpdatedAt time.Time `json:"updated_at" validate:"required" format:"date-time"`
}

// DoneEvent is the data of the `done` event: the last event of a stream, sent when the campaign
// is completed or cancelled.
type DoneEvent struct {
	Status string `json:"status" validate:"required" enums:"completed,cancelled"`
}

// PingEvent is the data of a `ping` event.
type PingEvent struct {
	// At is the time the ping was sent.
	At time.Time `json:"at" validate:"required" format:"date-time"`
}

// EventCatalog documents the stream of GET /campaigns/{id}/deliveries/events: one property per
// event, named like the SSE event, holding the schema of that event's data line. It is never sent
// as a whole.
type EventCatalog struct {
	Snapshot DeliverySummary `json:"snapshot" validate:"required"`
	Done     DoneEvent       `json:"done" validate:"required"`
	Ping     PingEvent       `json:"ping" validate:"required"`
}

// Names of the events of the stream.
const (
	EventSnapshot = "snapshot"
	EventDone     = "done"
	EventPing     = "ping"
)

// sameState reports whether a and b show the same campaign state: status, total and counts. The
// read time is ignored, so a poll that finds nothing new sends nothing.
func sameState(a, b DeliverySummary) bool {
	if a.Status != b.Status || a.Total != b.Total || len(a.Counts) != len(b.Counts) {
		return false
	}
	for i := range a.Counts {
		if a.Counts[i] != b.Counts[i] {
			return false
		}
	}
	return true
}

// terminal reports whether the campaign has nothing left to send.
func terminal(status string) bool { return status == StatusCompleted || status == StatusCancelled }
