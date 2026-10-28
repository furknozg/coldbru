package llm

import (
	"errors"
	"strings"
	"time"

	"github.com/furknozg/coldbru/internal/proposal/customer"
)

// Channel is a delivery channel (D-010).
type Channel string

// Channels.
const (
	ChannelSMS   Channel = "sms"
	ChannelEmail Channel = "email"
)

// Valid reports whether c is a known channel.
func (c Channel) Valid() bool { return c == ChannelSMS || c == ChannelEmail }

// Other returns the other channel.
func (c Channel) Other() Channel {
	if c == ChannelEmail {
		return ChannelSMS
	}
	return ChannelEmail
}

// PlanMode is how the audience is spread over the channels (D-010).
type PlanMode string

// Plan modes.
const (
	// ModeSingle: one channel; only people reachable on it receive a message.
	ModeSingle PlanMode = "single"
	// ModePrimaryFallback: one message per person, on the primary channel, or on the other one
	// when the person is not reachable on the primary.
	ModePrimaryFallback PlanMode = "primary_fallback"
	// ModeBoth: a message on each channel the person is reachable on.
	ModeBoth PlanMode = "both"
)

// Valid reports whether m is a known mode.
func (m PlanMode) Valid() bool {
	return m == ModeSingle || m == ModePrimaryFallback || m == ModeBoth
}

// ChannelPlan is the model's proposal for the channel plan step (D-070). Primary is the channel of
// ModeSingle and the primary of ModePrimaryFallback; for ModeBoth it is only a tiebreak the plan
// does not use.
type ChannelPlan struct {
	Mode      PlanMode
	Primary   Channel
	Rationale string
}

// ChannelPlanInput is the call-specific data of the plan call. It goes to the model as one JSON
// object in the user message (D-060).
type ChannelPlanInput struct {
	// Brief is the marketer's text. It is untrusted data.
	Brief string
	// ContextNotes are remarks of the marketer about this campaign, and the reason of a rejected
	// earlier proposal when there is one.
	ContextNotes []string
	// Coverage is the coverage table SQL computed for the approved audience: the model never counts.
	Coverage *customer.Preview
}

// ChannelPlanResult is a plan that passed validate.
type ChannelPlanResult struct {
	Plan ChannelPlan
	// Attempts is the number of model calls used, the accepted one included.
	Attempts int
	Usage    Usage
}

// ScheduleEntry is the send time proposed for one language (D-009). SendAt is nil for "send
// immediately". It carries the offset the model wrote.
type ScheduleEntry struct {
	Language  string
	SendAt    *time.Time
	Rationale string
}

// ScheduleInput is the call-specific data of the schedule call (D-060, D-070).
type ScheduleInput struct {
	Brief        string
	ContextNotes []string
	// Languages are the languages that need a send time; the model returns one entry for each.
	Languages []string
	// Now is the current time; the adapter sends it as RFC 3339 in UTC.
	Now time.Time
}

// ScheduleResult is a schedule that passed validate.
type ScheduleResult struct {
	Entries  []ScheduleEntry
	Attempts int
	Usage    Usage
}

// Timezones maps a customer language to the IANA zone its send time is read in (D-070).
var Timezones = map[string]string{"tr": "Europe/Istanbul", "en": "Europe/London"}

// TimezoneOf returns the zone of language, UTC for a language without one.
func TimezoneOf(language string) string {
	if z, ok := Timezones[language]; ok {
		return z
	}
	return "UTC"
}

// ProblemsError is a validation error with a list of problems. A validate callback returns it so
// that the adapter can send every problem back to the model on its own line and keep them in the
// RepairExhaustedError; any other error is one problem.
type ProblemsError struct{ Problems []string }

func (e *ProblemsError) Error() string { return strings.Join(e.Problems, "; ") }

// NewProblemsError returns nil for an empty list, otherwise a *ProblemsError.
func NewProblemsError(problems []string) error {
	if len(problems) == 0 {
		return nil
	}
	return &ProblemsError{Problems: problems}
}

// Problems lists the problems in err: those of a *ProblemsError found anywhere in the chain, else
// the text of err as the single problem. It is empty for a nil error.
func Problems(err error) []string {
	if err == nil {
		return nil
	}
	var pe *ProblemsError
	if errors.As(err, &pe) {
		return pe.Problems
	}
	return []string{err.Error()}
}
