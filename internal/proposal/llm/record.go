package llm

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/furknozg/coldbru/internal/proposal/store"
)

// Purpose says which call of the flow a model call belongs to; it is llm_call.purpose (D-068).
type Purpose string

// Purposes.
const (
	PurposeScreen  Purpose = "screen"
	PurposeSegment Purpose = "segment"
	PurposeContent Purpose = "content"
)

// Outcome is llm_call.outcome (D-068): what became of the call's output.
type Outcome string

// Outcomes.
const (
	// OutcomeAccepted: the output passed every check the adapter applies. For the segment loop that
	// means every tool call of the turn was valid; the turn that carries the accepted propose_segment
	// is SegmentResult.Turn.
	OutcomeAccepted Outcome = "accepted"
	// OutcomeValidationFailed: the output was rejected (invalid DSL, no tool call, cut off, not JSON).
	OutcomeValidationFailed Outcome = "validation_failed"
	// OutcomeRefusal: stop_reason refusal.
	OutcomeRefusal Outcome = "refusal"
	// OutcomeAPIError: the API call failed, or the segment tools failed for a non-DSL reason.
	OutcomeAPIError Outcome = "api_error"
)

// PromptRef identifies the prompt package a call was made with (D-059, D-060).
type PromptRef struct {
	Name    string
	Version string
	SHA256  string
}

// CallRecord describes one model call: one row of llm_call (D-060, D-068). Adapters emit one per
// API request, after the call and its validation are settled; it is final because llm_call is
// append-only.
type CallRecord struct {
	Purpose Purpose
	Model   string
	// Attempt is 1-based within one port method call: the model turn of the segment loop, always 1
	// for Screen and StreamStructured. A caller that repeats StreamStructured to repair its content
	// renumbers the records itself.
	Attempt    int
	Usage      Usage
	StopReason string
	LatencyMs  int64
	Prompt     PromptRef
	Outcome    Outcome
	// Response is the model's structured output as JSON, kept even when it was rejected: the
	// verdict for Screen, the tool calls ([{"name":..., "input":...}]) of a segment turn, the
	// document of StreamStructured. Nil when the call produced none.
	Response json.RawMessage
	// Error is the failure text; always set for OutcomeAPIError. For a refusal it names the
	// stop_details category.
	Error string
	// RefusalCategory is stop_details.category of a refusal, "" otherwise. llm_call has no column
	// for it: StoreParams writes it into Error.
	RefusalCategory string
}

// Recorder receives the records of model calls. Implementations must be safe for concurrent use
// and must not block for long: they run inside the call.
type Recorder interface {
	Record(ctx context.Context, rec CallRecord)
}

// RecorderFunc adapts a function to Recorder.
type RecorderFunc func(ctx context.Context, rec CallRecord)

// Record calls f.
func (f RecorderFunc) Record(ctx context.Context, rec CallRecord) { f(ctx, rec) }

type recorderKey struct{}

// WithRecorder returns a context whose model calls are reported to r. The flow of T-023 builds one
// recorder per campaign step and passes the context to the port.
func WithRecorder(ctx context.Context, r Recorder) context.Context {
	return context.WithValue(ctx, recorderKey{}, r)
}

// Report sends rec to the recorder in ctx, if any. Adapters call it once per model call.
func Report(ctx context.Context, rec CallRecord) {
	if r, ok := ctx.Value(recorderKey{}).(Recorder); ok && r != nil {
		r.Record(ctx, rec)
	}
}

// Collector is a Recorder that keeps the records in memory, for callers that persist them after
// the flow (and may adjust an outcome first) and for tests.
type Collector struct {
	mu   sync.Mutex
	recs []CallRecord
}

// Record appends rec.
func (c *Collector) Record(_ context.Context, rec CallRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recs = append(c.recs, rec)
}

// Records returns a copy of the records in the order they were reported.
func (c *Collector) Records() []CallRecord {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]CallRecord(nil), c.recs...)
}

// StoreParams converts the record into the arguments of store.InsertLLMCall. campaignID is
// required by the table; stepID and variantID may be nil.
func (r CallRecord) StoreParams(campaignID uuid.UUID, stepID, variantID *uuid.UUID) store.InsertLLMCallParams {
	p := store.InsertLLMCallParams{
		CampaignID:          campaignID,
		StepID:              stepID,
		VariantID:           variantID,
		Purpose:             string(r.Purpose),
		Model:               r.Model,
		Attempt:             int32(r.Attempt),
		InputTokens:         int32(r.Usage.InputTokens),
		OutputTokens:        int32(r.Usage.OutputTokens),
		CacheReadTokens:     int32(r.Usage.CacheReadTokens),
		CacheCreationTokens: int32(r.Usage.CacheCreationTokens),
		LatencyMs:           int32(r.LatencyMs),
		PromptName:          r.Prompt.Name,
		PromptVersion:       r.Prompt.Version,
		PromptSha256:        r.Prompt.SHA256,
		Outcome:             string(r.Outcome),
	}
	if r.StopReason != "" {
		p.StopReason = pgtype.Text{String: r.StopReason, Valid: true}
	}
	if len(r.Response) > 0 {
		p.Response = []byte(r.Response)
	}
	errText := r.Error
	if r.RefusalCategory != "" && errText == "" {
		errText = "refusal, stop_details.category: " + r.RefusalCategory
	}
	if errText != "" {
		p.Error = pgtype.Text{String: errText, Valid: true}
	}
	return p
}
