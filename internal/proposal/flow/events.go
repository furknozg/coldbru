package flow

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// Event is something that happened to a campaign. The flow publishes events to the Hub after the
// state they describe is committed; T-025 maps them to SSE (D-013, D-070). Events are typed Go
// structs; Type is the stable name a wire format uses.
type Event interface {
	// Type is the event's name, for example "variant_created".
	Type() string
	// Campaign is the campaign the event belongs to.
	Campaign() uuid.UUID
	// When is the time the event was created.
	When() time.Time
}

// Meta is the part every event shares.
type Meta struct {
	CampaignID uuid.UUID `json:"campaign_id" validate:"required" format:"uuid"`
	At         time.Time `json:"at" validate:"required" format:"date-time"`
}

// Campaign returns the campaign id.
func (m Meta) Campaign() uuid.UUID { return m.CampaignID }

// When returns the creation time.
func (m Meta) When() time.Time { return m.At }

// The events of the flow.
type (
	// ScreenResult: the injection pre-screen answered. Verdict is "allow", "block", or "error" when
	// the screen call itself failed; the campaign is cancelled for the last two (D-069).
	ScreenResult struct {
		Meta
		Verdict string `json:"verdict" validate:"required" enums:"allow,block,error"`
		Reason  string `json:"reason" validate:"required"`
	}
	// CampaignCancelled: the campaign was cancelled; Reason says why.
	CampaignCancelled struct {
		Meta
		Reason string `json:"reason" validate:"required"`
	}
	// CampaignConfirmed: the schedule was confirmed, the campaign is `scheduled`, and the confirmed
	// event and the recipient batches were written to the outbox (D-074). RecipientCounts are
	// messages per channel; a person on two channels (plan mode both) counts twice.
	CampaignConfirmed struct {
		Meta
		RecipientCounts  Recipients `json:"recipient_counts" validate:"required"`
		RecipientBatches int        `json:"recipient_batches" validate:"required"`
	}
	// CampaignStatus: the campaign status changed because Notification reported progress
	// (D-080): `sending` when the first schedule fired, `completed` when every delivery is final.
	CampaignStatus struct {
		Meta
		Status string `json:"status" validate:"required" enums:"sending,completed"`
	}
	// StepOpened: a step opened, version 1 or a new version after a reopen.
	StepOpened struct {
		Meta
		StepID  uuid.UUID `json:"step_id" validate:"required" format:"uuid"`
		Kind    string    `json:"kind" validate:"required" enums:"audience,channel_plan,content,schedule"`
		Version int       `json:"version" validate:"required"`
	}
	// StepApproved: every slot of the step is approved.
	StepApproved struct {
		Meta
		StepID  uuid.UUID `json:"step_id" validate:"required" format:"uuid"`
		Kind    string    `json:"kind" validate:"required" enums:"audience,channel_plan,content,schedule"`
		Version int       `json:"version" validate:"required"`
	}
	// StepSuperseded: the step was replaced by a reopen.
	StepSuperseded struct {
		Meta
		StepID  uuid.UUID `json:"step_id" validate:"required" format:"uuid"`
		Kind    string    `json:"kind" validate:"required" enums:"audience,channel_plan,content,schedule"`
		Version int       `json:"version" validate:"required"`
	}
	// GenerationStarted: the model (or the content generator) started on the Slots of a step.
	GenerationStarted struct {
		Meta
		StepID uuid.UUID `json:"step_id" validate:"required" format:"uuid"`
		Kind   string    `json:"kind" validate:"required" enums:"audience,channel_plan,content,schedule"`
		Slots  []Slot    `json:"slots" validate:"required"`
	}
	// GenerationFailed: generating a slot failed, for example a refusal or an exhausted repair
	// loop. Slot is empty for steps with one slot. Deltas received earlier are to be dropped.
	GenerationFailed struct {
		Meta
		StepID uuid.UUID `json:"step_id" validate:"required" format:"uuid"`
		Kind   string    `json:"kind" validate:"required" enums:"audience,channel_plan,content,schedule"`
		Slot   Slot      `json:"slot" validate:"required"`
		Reason string    `json:"reason" validate:"required"`
	}
	// VariantCreated: a new draft variant exists (generated or edited).
	VariantCreated struct {
		Meta
		StepID    uuid.UUID       `json:"step_id" validate:"required" format:"uuid"`
		VariantID uuid.UUID       `json:"variant_id" validate:"required" format:"uuid"`
		Kind      string          `json:"kind" validate:"required" enums:"audience,channel_plan,content,schedule"`
		Slot      Slot            `json:"slot" validate:"required"`
		Content   json.RawMessage `json:"content" swaggertype:"object" validate:"required"`
	}
	// VariantApproved: a variant was approved.
	VariantApproved struct {
		Meta
		StepID    uuid.UUID `json:"step_id" validate:"required" format:"uuid"`
		VariantID uuid.UUID `json:"variant_id" validate:"required" format:"uuid"`
	}
	// VariantRejected: a variant was rejected with a reason; its slot is regenerated.
	VariantRejected struct {
		Meta
		StepID    uuid.UUID `json:"step_id" validate:"required" format:"uuid"`
		VariantID uuid.UUID `json:"variant_id" validate:"required" format:"uuid"`
		Reason    string    `json:"reason" validate:"required"`
	}
	// VariantSuperseded: a variant was replaced (by a new draft, a regeneration or a reopen).
	VariantSuperseded struct {
		Meta
		StepID    uuid.UUID `json:"step_id" validate:"required" format:"uuid"`
		VariantID uuid.UUID `json:"variant_id" validate:"required" format:"uuid"`
	}
	// ContentDelta: a text fragment of a content variant being generated, forwarded from the
	// ContentGenerator. Attempt is the 1-based attempt the fragment belongs to: the text of an
	// attempt after the first replaces the earlier text (see ContentReset). It is a preview; the
	// variant is final only with VariantCreated.
	ContentDelta struct {
		Meta
		StepID  uuid.UUID `json:"step_id" validate:"required" format:"uuid"`
		Slot    Slot      `json:"slot" validate:"required"`
		Text    string    `json:"text" validate:"required"`
		Attempt int       `json:"attempt" validate:"required"`
	}
	// ContentReset: attempt (2 or 3) of a content slot starts because the previous answer broke a
	// rule; the preview of the slot is to be cleared, and the deltas that follow carry the new
	// attempt number (D-071, D-072).
	ContentReset struct {
		Meta
		StepID  uuid.UUID `json:"step_id" validate:"required" format:"uuid"`
		Slot    Slot      `json:"slot" validate:"required"`
		Attempt int       `json:"attempt" validate:"required"`
	}
	// ContextAdded: the marketer added a context note to the open step.
	ContextAdded struct {
		Meta
		StepID uuid.UUID `json:"step_id" validate:"required" format:"uuid"`
		NoteID uuid.UUID `json:"note_id" validate:"required" format:"uuid"`
		Body   string    `json:"body" validate:"required"`
	}
	// CampaignTitled: the pre-screen named the campaign (D-088).
	CampaignTitled struct {
		Meta
		Title string `json:"title" validate:"required"`
	}
	// QuestionAsked: the agent asked the marketer a clarifying question instead of proposing; the
	// step waits for POST /messages/{message_id}/answer (D-088). Options are the suggested
	// answers, when the agent gave any.
	QuestionAsked struct {
		Meta
		StepID    uuid.UUID `json:"step_id" validate:"required" format:"uuid"`
		MessageID uuid.UUID `json:"message_id" validate:"required" format:"uuid"`
		Question  string    `json:"question" validate:"required"`
		Options   []string  `json:"options,omitempty"`
	}
	// AnswerGiven: the marketer answered a question; the step is generated again with the answer
	// (D-088). MessageID is the answer, QuestionID the question it belongs to.
	AnswerGiven struct {
		Meta
		StepID     uuid.UUID `json:"step_id" validate:"required" format:"uuid"`
		MessageID  uuid.UUID `json:"message_id" validate:"required" format:"uuid"`
		QuestionID uuid.UUID `json:"question_id" validate:"required" format:"uuid"`
		Answer     string    `json:"answer" validate:"required"`
	}
	// SummaryPosted: the agent summarised what it proposed for a step, in markdown (D-088). Slot
	// names the content slot the summary belongs to; it is empty for the other steps.
	SummaryPosted struct {
		Meta
		StepID    uuid.UUID `json:"step_id" validate:"required" format:"uuid"`
		MessageID uuid.UUID `json:"message_id" validate:"required" format:"uuid"`
		Slot      Slot      `json:"slot" validate:"required"`
		Markdown  string    `json:"markdown" validate:"required"`
	}
)

// Type names.
const (
	TypeScreenResult      = "screen_result"
	TypeCampaignCancelled = "campaign_cancelled"
	TypeCampaignConfirmed = "campaign_confirmed"
	TypeCampaignStatus    = "campaign_status"
	TypeStepOpened        = "step_opened"
	TypeStepApproved      = "step_approved"
	TypeStepSuperseded    = "step_superseded"
	TypeGenerationStarted = "generation_started"
	TypeGenerationFailed  = "generation_failed"
	TypeVariantCreated    = "variant_created"
	TypeVariantApproved   = "variant_approved"
	TypeVariantRejected   = "variant_rejected"
	TypeVariantSuperseded = "variant_superseded"
	TypeContentDelta      = "content_delta"
	TypeContentReset      = "content_reset"
	TypeContextAdded      = "context_added"
	TypeCampaignTitled    = "campaign_titled"
	TypeQuestionAsked     = "question_asked"
	TypeAnswerGiven       = "answer_given"
	TypeSummaryPosted     = "summary_posted"
)

// Type implementations.
func (ScreenResult) Type() string      { return TypeScreenResult }
func (CampaignCancelled) Type() string { return TypeCampaignCancelled }
func (CampaignConfirmed) Type() string { return TypeCampaignConfirmed }
func (CampaignStatus) Type() string    { return TypeCampaignStatus }
func (StepOpened) Type() string        { return TypeStepOpened }
func (StepApproved) Type() string      { return TypeStepApproved }
func (StepSuperseded) Type() string    { return TypeStepSuperseded }
func (GenerationStarted) Type() string { return TypeGenerationStarted }
func (GenerationFailed) Type() string  { return TypeGenerationFailed }
func (VariantCreated) Type() string    { return TypeVariantCreated }
func (VariantApproved) Type() string   { return TypeVariantApproved }
func (VariantRejected) Type() string   { return TypeVariantRejected }
func (VariantSuperseded) Type() string { return TypeVariantSuperseded }
func (ContentDelta) Type() string      { return TypeContentDelta }
func (ContentReset) Type() string      { return TypeContentReset }
func (ContextAdded) Type() string      { return TypeContextAdded }
func (CampaignTitled) Type() string    { return TypeCampaignTitled }
func (QuestionAsked) Type() string     { return TypeQuestionAsked }
func (AnswerGiven) Type() string       { return TypeAnswerGiven }
func (SummaryPosted) Type() string     { return TypeSummaryPosted }

// DefaultHubBuffer is the buffer of a subscription made with NewHub(0).
const DefaultHubBuffer = 256

// Hub fans events out to the subscribers of a campaign. It lives in memory: it does not survive a
// restart and is not shared between replicas (D-070). Publish never blocks: a subscriber whose
// buffer is full misses the event, and the hub counts the drop. A consumer that notices a gap
// (Subscription.Dropped grew) re-reads the state with Service.Get.
type Hub struct {
	buffer  int
	mu      sync.RWMutex
	subs    map[uuid.UUID]map[*Subscription]struct{}
	dropped atomic.Uint64
}

// NewHub returns a hub whose subscriptions buffer buffer events each; DefaultHubBuffer when
// buffer <= 0.
func NewHub(buffer int) *Hub {
	if buffer <= 0 {
		buffer = DefaultHubBuffer
	}
	return &Hub{buffer: buffer, subs: map[uuid.UUID]map[*Subscription]struct{}{}}
}

// Subscription receives the events of one campaign on C until Close.
type Subscription struct {
	// C delivers events in publishing order. It is closed by Close.
	C <-chan Event

	hub      *Hub
	campaign uuid.UUID
	ch       chan Event
	dropped  atomic.Uint64
}

// Subscribe returns a subscription for the events of campaignID published from now on.
func (h *Hub) Subscribe(campaignID uuid.UUID) *Subscription {
	ch := make(chan Event, h.buffer)
	s := &Subscription{C: ch, hub: h, campaign: campaignID, ch: ch}
	h.mu.Lock()
	defer h.mu.Unlock()
	m := h.subs[campaignID]
	if m == nil {
		m = map[*Subscription]struct{}{}
		h.subs[campaignID] = m
	}
	m[s] = struct{}{}
	return s
}

// Publish delivers ev to every subscriber of its campaign without blocking.
func (h *Hub) Publish(ev Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subs[ev.Campaign()] {
		select {
		case s.ch <- ev:
		default:
			s.dropped.Add(1)
			h.dropped.Add(1)
		}
	}
}

// Dropped returns the number of deliveries dropped because a subscriber was too slow.
func (h *Hub) Dropped() uint64 { return h.dropped.Load() }

// Subscribers returns the number of open subscriptions of campaignID.
func (h *Hub) Subscribers(campaignID uuid.UUID) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs[campaignID])
}

// Dropped returns the number of events this subscription missed.
func (s *Subscription) Dropped() uint64 { return s.dropped.Load() }

// Close removes the subscription and closes C. It is safe to call more than once.
func (s *Subscription) Close() {
	h := s.hub
	h.mu.Lock()
	defer h.mu.Unlock()
	m := h.subs[s.campaign]
	if _, ok := m[s]; !ok {
		return
	}
	delete(m, s)
	if len(m) == 0 {
		delete(h.subs, s.campaign)
	}
	close(s.ch)
}
