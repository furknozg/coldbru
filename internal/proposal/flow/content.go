package flow

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
)

// ContentRequest is what the content step gives a ContentGenerator for one (language, channel)
// slot (D-009, D-010, D-070). The brief and the notes are untrusted data for the model.
type ContentRequest struct {
	CampaignID uuid.UUID
	StepID     uuid.UUID
	Language   string
	Channel    string
	Brief      string
	// ContextNotes are all notes of the campaign, oldest first.
	ContextNotes []string
	// Audience and Plan are the approved audience and channel plan.
	Audience AudienceContent
	Plan     PlanContent
	// Recipients is how many people the plan sends to in this language on this channel.
	Recipients int
	// RejectionReason is the marketer's reason when this slot replaces a rejected variant.
	RejectionReason string
}

// ContentGenerator writes and checks the texts of the content step. contentgen implements it with
// the content prompt and the validators of package content (D-071); the flow treats the content as
// opaque JSON, except that it reads estimated_cost_gbp for the cost summary (D-070).
// Implementations must be safe for concurrent use.
type ContentGenerator interface {
	// Generate returns the validated content of one slot. onDelta (may be nil) gets the text
	// fragments of the stream as they arrive; they are a preview that the caller drops when
	// Generate fails. The model calls it makes are reported through llm.Report on ctx: the flow
	// puts a recorder there and stores the records (D-060). The returned content must be a JSON
	// object.
	Generate(ctx context.Context, req ContentRequest, onDelta func(text string)) (json.RawMessage, error)
	// Validate checks hand-edited content of the slot req describes (its language, channel,
	// audience, plan and recipients; the other fields may be empty) with the validators Generate
	// applies (D-070, D-071), and returns the content to store: the edit with the measurements and
	// the cost recomputed. The error's problems (llm.Problems) are returned to the marketer.
	Validate(req ContentRequest, content json.RawMessage) (json.RawMessage, error)
}

// Confirmer is called once when the last variant of the schedule step is approved, after the
// approval is committed (D-070). T-026 implements it: it fetches the recipients, assigns the
// channels and publishes the confirmed event through the outbox. A failure is returned by
// Approve wrapped in ErrConfirm; the approval is not rolled back.
type Confirmer interface {
	Confirm(ctx context.Context, campaignID uuid.UUID) error
}

// NopConfirmer is the default Confirmer: it does nothing.
type NopConfirmer struct{}

// Confirm does nothing.
func (NopConfirmer) Confirm(context.Context, uuid.UUID) error { return nil }
