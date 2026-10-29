package mock

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/prompts"
)

// DefaultChunkDelay is the pause between the chunks of a streamed text: long enough for the
// marketer to see the text arrive in the UI, short enough not to slow a demo down.
const DefaultChunkDelay = 60 * time.Millisecond

// mockTag marks a response as the mock's.
const mockTag = "[mock]"

// maxNoteChars caps the note quoted in a rationale.
const maxNoteChars = 200

// regenerationNote returns what to quote when a call is a regeneration (D-072): the reason of a
// rejection or the latest context note, "" when the input has none. The flow hands a rejection to
// the model as a note ("... Reason: <reason>"), so the reason is cut out of it.
func regenerationNote(notes []string) string {
	for i := len(notes) - 1; i >= 0; i-- {
		n := strings.Join(strings.Fields(notes[i]), " ")
		if n == "" {
			continue
		}
		if _, after, ok := strings.Cut(n, "Reason: "); ok && strings.Contains(n, "rejected the previous") {
			n = after
		}
		if r := []rune(n); len(r) > maxNoteChars {
			n = strings.TrimSpace(string(r[:maxNoteChars])) + "…"
		}
		return n
	}
	return ""
}

// regenerated prefixes a rationale with the regeneration marker when the call carried a note.
func regenerated(notes []string, rationale string) string {
	if n := regenerationNote(notes); n != "" {
		return mockTag + " yeniden üretim (regeneration): " + n + ". " + rationale
	}
	return rationale
}

// LLM implements llm.LLM without a model. It is safe for concurrent use.
type LLM struct {
	chunkDelay time.Duration
}

var _ llm.LLM = (*LLM)(nil)

// Option customizes an LLM.
type Option func(*LLM)

// WithChunkDelay sets the pause between the chunks StreamStructured delivers; tests pass 0.
func WithChunkDelay(d time.Duration) Option { return func(m *LLM) { m.chunkDelay = d } }

// New returns the mock.
func New(opts ...Option) *LLM {
	m := &LLM{chunkDelay: DefaultChunkDelay}
	for _, o := range opts {
		o(m)
	}
	return m
}

// record reports one finished mock call: model "mock", no tokens, the real prompt identity.
func record(ctx context.Context, purpose llm.Purpose, pkg *prompts.Package, started time.Time, outcome llm.Outcome, response json.RawMessage, errText string) {
	llm.Report(ctx, llm.CallRecord{
		Purpose: purpose, Model: Model, Attempt: 1, StopReason: "end_turn",
		LatencyMs: time.Since(started).Milliseconds(),
		Prompt:    llm.PromptRef{Name: pkg.Name, Version: pkg.Version, SHA256: pkg.SHA256},
		Outcome:   outcome, Response: response, Error: errText,
	})
}

// marshal encodes v without HTML escaping, as the real adapter does.
func marshal(v any) json.RawMessage {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil // unreachable: plain strings and slices
	}
	return bytes.TrimRight(buf.Bytes(), "\n")
}

// cancelled returns the error of a call whose context ended before it began.
func cancelled(ctx context.Context, purpose llm.Purpose) error {
	if err := ctx.Err(); err != nil {
		return &llm.APIError{Purpose: purpose, Err: err}
	}
	return nil
}

// Screen always allows.
func (m *LLM) Screen(ctx context.Context, _ string) (llm.ScreenResult, error) {
	if err := cancelled(ctx, llm.PurposeScreen); err != nil {
		return llm.ScreenResult{}, err
	}
	started := time.Now()
	res := llm.ScreenResult{Verdict: llm.VerdictAllow, Reason: "mock"}
	record(ctx, llm.PurposeScreen, prompts.Screen(), started, llm.OutcomeAccepted,
		marshal(struct {
			Verdict llm.Verdict `json:"verdict"`
			Reason  string      `json:"reason"`
		}{res.Verdict, res.Reason}), "")
	return res, nil
}

// ProposeSegment proposes the hand-written filter of the brief, or DefaultDSL, and runs it through
// tools.Validate like the real loop. A filter that does not validate (a changed catalog, an empty
// audience) ends the call with *llm.RepairExhaustedError after one attempt: the mock has nothing
// else to propose.
func (m *LLM) ProposeSegment(ctx context.Context, in llm.SegmentInput, tools llm.SegmentTools) (llm.SegmentResult, error) {
	if err := cancelled(ctx, llm.PurposeSegment); err != nil {
		return llm.SegmentResult{}, err
	}
	started := time.Now()
	pkg := prompts.Segment()
	c, _ := Match(in.Brief)
	calls := marshal([]struct {
		Name  string `json:"name"`
		Input struct {
			DSL string `json:"dsl"`
		} `json:"input"`
	}{{Name: "propose_segment", Input: struct {
		DSL string `json:"dsl"`
	}{c.DSL}}})

	f, p, errs, err := tools.Validate(ctx, c.DSL)
	switch {
	case err != nil:
		record(ctx, llm.PurposeSegment, pkg, started, llm.OutcomeAPIError, calls, "tools: "+err.Error())
		return llm.SegmentResult{}, &llm.ToolsError{Err: err}
	case len(errs) > 0:
		record(ctx, llm.PurposeSegment, pkg, started, llm.OutcomeValidationFailed, calls, "propose_segment was rejected")
		return llm.SegmentResult{}, &llm.RepairExhaustedError{Attempts: 1, Errors: errs, Reason: "the mock has one filter per brief and it was rejected"}
	}
	record(ctx, llm.PurposeSegment, pkg, started, llm.OutcomeAccepted, calls, "")
	return llm.SegmentResult{DSL: c.DSL, Filter: f, Preview: p, Turn: 1, Turns: 1, ProposeAttempts: 1}, nil
}

// ProposeChannelPlan proposes primary_fallback with the channel that reaches more people as the
// primary (email on a tie, D-010).
func (m *LLM) ProposeChannelPlan(ctx context.Context, in llm.ChannelPlanInput, validate func(llm.ChannelPlan) error) (llm.ChannelPlanResult, error) {
	if err := cancelled(ctx, llm.PurposeChannelPlan); err != nil {
		return llm.ChannelPlanResult{}, err
	}
	started := time.Now()
	pkg := prompts.ChannelPlan()
	primary := llm.ChannelEmail
	if cv := in.Coverage; cv != nil && cv.Coverage.Both+cv.Coverage.SmsOnly > cv.Coverage.Both+cv.Coverage.EmailOnly {
		primary = llm.ChannelSMS
	}
	plan := llm.ChannelPlan{
		Mode: llm.ModePrimaryFallback, Primary: primary,
		Rationale: regenerated(in.ContextNotes, fmt.Sprintf("%s Bu bir mock yanıt (mock response): %s birincil kanal, daha çok kişiye ulaşıyor; ona ulaşamadığı kişilere diğer kanaldan gönderilir.", mockTag, primary)),
	}
	out := marshal(struct {
		Mode      llm.PlanMode `json:"mode"`
		Primary   llm.Channel  `json:"primary"`
		Rationale string       `json:"rationale"`
	}{plan.Mode, plan.Primary, plan.Rationale})
	if validate != nil {
		if err := validate(plan); err != nil {
			record(ctx, llm.PurposeChannelPlan, pkg, started, llm.OutcomeValidationFailed, out, err.Error())
			return llm.ChannelPlanResult{}, &llm.RepairExhaustedError{Purpose: llm.PurposeChannelPlan, Attempts: 1, Problems: llm.Problems(err), Reason: "the mock has one plan and it was rejected"}
		}
	}
	record(ctx, llm.PurposeChannelPlan, pkg, started, llm.OutcomeAccepted, out, "")
	return llm.ChannelPlanResult{Plan: plan, Attempts: 1}, nil
}

// ProposeSchedule proposes "send immediately" for every (language, timezone) pair (D-076).
func (m *LLM) ProposeSchedule(ctx context.Context, in llm.ScheduleInput, validate func([]llm.ScheduleEntry) error) (llm.ScheduleResult, error) {
	if err := cancelled(ctx, llm.PurposeSchedule); err != nil {
		return llm.ScheduleResult{}, err
	}
	started := time.Now()
	pkg := prompts.Schedule()
	entries := make([]llm.ScheduleEntry, len(in.Locales))
	type item struct {
		Language  string  `json:"language"`
		Timezone  string  `json:"timezone"`
		SendAt    *string `json:"send_at"`
		Rationale string  `json:"rationale"`
	}
	items := make([]item, len(in.Locales))
	for i, loc := range in.Locales {
		why := regenerated(in.ContextNotes, mockTag+" Bu bir mock yanıt (mock response): hemen gönder.")
		entries[i] = llm.ScheduleEntry{Language: loc.Language, Timezone: loc.Timezone, Rationale: why}
		items[i] = item{Language: loc.Language, Timezone: loc.Timezone, Rationale: why}
	}
	out := marshal(struct {
		Entries []item `json:"entries"`
	}{items})
	if validate != nil {
		if err := validate(entries); err != nil {
			record(ctx, llm.PurposeSchedule, pkg, started, llm.OutcomeValidationFailed, out, err.Error())
			return llm.ScheduleResult{}, &llm.RepairExhaustedError{Purpose: llm.PurposeSchedule, Attempts: 1, Problems: llm.Problems(err), Reason: "the mock has one schedule and it was rejected"}
		}
	}
	record(ctx, llm.PurposeSchedule, pkg, started, llm.OutcomeAccepted, out, "")
	return llm.ScheduleResult{Entries: entries, Attempts: 1}, nil
}
