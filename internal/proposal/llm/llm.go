// Package llm is the port between Proposal's agent flow and the language model (D-025, D-027,
// D-069). The flow (T-023) depends on the LLM interface only; the real adapter is in package
// anthropic and the test doubles are in package llmtest, so every test runs without an API key.
//
// The package also owns what any adapter must report: typed errors, the per-call Recorder that
// feeds the llm_call table (D-060, D-068), and the SegmentTools through which the segment loop
// validates and counts a filter against the real backend.
package llm

import (
	"context"
	"encoding/json"

	"github.com/furknozg/coldbru/internal/proposal/compile"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/prompts"
)

// Limits of the segment loop (D-069).
const (
	// MaxProposeAttempts is the number of propose_segment attempts in total: one plus two repairs.
	// A turn that ends without calling the tool counts as an attempt.
	MaxProposeAttempts = 3
	// MaxSegmentTurns is the number of model turns in total, count_audience probes included.
	MaxSegmentTurns = 8
)

// LLM is what the agent flow needs from a model. Methods are safe for concurrent use. Every model
// call is reported to the Recorder in ctx (see WithRecorder).
type LLM interface {
	// Screen decides whether a brief may enter the pipeline (D-069). It is fail-closed: a block
	// verdict returns the result together with *ScreenBlockedError, and every API failure, refusal
	// or unreadable answer returns an error with a zero verdict. Only a nil error means allow.
	Screen(ctx context.Context, brief string) (ScreenResult, error)

	// ProposeSegment runs the tool loop that turns a brief into a validated DSL filter. tools
	// validates and counts the filters the model writes. Errors: *RefusalError, *RepairExhaustedError,
	// *TurnLimitError, *APIError, or a *ToolsError when tools itself fails.
	ProposeSegment(ctx context.Context, in SegmentInput, tools SegmentTools) (SegmentResult, error)

	// StreamStructured runs one streamed structured-output call with prompt package pkg (which
	// carries the system text and the output schema) and userJSON as the user message. onDelta gets
	// every text fragment as it arrives, before anything is validated: the caller must treat deltas
	// as a preview and discard them when the call fails (onDelta may be nil).
	//
	// After a clean stream with valid JSON, validate (may be nil) checks the document against the
	// schema and the channel limits. The call's record is emitted exactly once, after the outcome is
	// known: accepted when validate is nil or returns nil, validation_failed with the error text
	// when it fails. A validate failure returns raw and usage together with *ContentInvalidError
	// wrapping it; there is no retry inside, repairing content is the caller's call.
	// Other errors return no raw: *RefusalError, *InvalidOutputError (cut off or not JSON),
	// *APIError.
	StreamStructured(ctx context.Context, pkg *prompts.Package, userJSON []byte, onDelta func(text string), validate func(raw json.RawMessage) error) (raw json.RawMessage, usage Usage, err error)

	// ProposeChannelPlan asks for the delivery plan of an audience (D-010, D-070). validate (may be
	// nil) is run in code on every answer; its error comes back to the model in the same
	// conversation as a user message, see Problems. At most MaxProposeAttempts attempts (one plus
	// two repairs). Errors: *RefusalError (no repair), *RepairExhaustedError carrying the last
	// problems, *APIError.
	ProposeChannelPlan(ctx context.Context, in ChannelPlanInput, validate func(ChannelPlan) error) (ChannelPlanResult, error)

	// ProposeSchedule asks for the send time of every language of the audience (D-009, D-070). The
	// repair loop and the errors are those of ProposeChannelPlan.
	ProposeSchedule(ctx context.Context, in ScheduleInput, validate func([]ScheduleEntry) error) (ScheduleResult, error)
}

// Verdict is the outcome of the injection pre-screen.
type Verdict string

// Verdicts of Screen.
const (
	VerdictAllow Verdict = "allow"
	VerdictBlock Verdict = "block"
)

// ScreenResult is the pre-screen's answer. Reason is one sentence in the language of the brief.
// Title is the campaign name the screen proposed (D-088, screen@2.0.0): at most MaxTitleChars
// characters, in the language of the brief; the flow stores it on the campaign.
type ScreenResult struct {
	Verdict Verdict
	Reason  string
	Title   string
}

// SegmentInput is the call-specific data of the segment call. It goes to the model as one JSON
// object in the user message, never into the system text (D-060).
type SegmentInput struct {
	// Brief is the marketer's text. It is untrusted data.
	Brief string
	// ContextNotes are earlier remarks of the marketer about this campaign (context_note rows).
	ContextNotes []string
	// Clarifications are the questions the agent asked in this step and the marketer's answers
	// (D-088). Answers override the brief where they conflict.
	Clarifications []Clarification
	// QuestionsLeft is how many clarifying questions the agent may still ask in this step (0 when the
	// limit of MaxQuestionsPerStep is reached): a question beyond it is a validation failure.
	QuestionsLeft int
}

// SegmentResult is a filter that passed Validate, or, when Question is set, the agent's clarifying
// question (D-088): the loop ended with ask_user, nothing was proposed and the other fields are
// empty except Turns and Usage.
type SegmentResult struct {
	// Question is set when the agent asked instead of proposing.
	Question *Question
	// Summary is the markdown the agent sent with send_summary after the accepted proposal; empty
	// when it sent none (the step succeeds without one).
	Summary string
	// DSL is the filter as the model wrote it in its accepted propose_segment call.
	DSL string
	// Filter is the compiled filter that Validate returned for DSL.
	Filter *compile.Filter
	// Preview is the audience size and channel reach Validate returned for DSL.
	Preview *customer.Preview
	// Turn is the model turn (Attempt of its CallRecord) that produced DSL.
	Turn int
	// Turns is the number of model turns used; ProposeAttempts the number of propose attempts,
	// the accepted one included.
	Turns           int
	ProposeAttempts int
	// Usage sums the tokens of all turns.
	Usage Usage
}

// Limits of the agent's questions and summaries (D-088).
const (
	// MaxQuestionsPerStep is how many clarifying questions the agent may ask in one step; after
	// them it must decide.
	MaxQuestionsPerStep = 2
	// MaxTitleChars is the longest campaign title (campaign.title CHECK).
	MaxTitleChars = 60
	// MaxQuestionChars, MaxOptionChars and MaxSummaryChars bound what the agent may write.
	MaxQuestionChars = 500
	MaxOptionChars   = 100
	MaxSummaryChars  = 2000
	// MinOptions and MaxOptions bound the options of a question.
	MinOptions = 2
	MaxOptions = 4
)

// Question is a clarifying question of the agent (D-088): the step waits for the marketer's
// answer. Options are 2 to 4 suggested answers, or none; the marketer may answer freely.
type Question struct {
	Text    string
	Options []string
}

// Clarification is one answered question of a step, given back to the model when the step is
// generated again (D-088).
type Clarification struct {
	Question string
	Answer   string
}

// Usage is the token accounting of a model call, from the API's usage object.
type Usage struct {
	InputTokens         int
	OutputTokens        int
	CacheReadTokens     int
	CacheCreationTokens int
}

// Add returns the sum of u and o.
func (u Usage) Add(o Usage) Usage {
	return Usage{
		InputTokens:         u.InputTokens + o.InputTokens,
		OutputTokens:        u.OutputTokens + o.OutputTokens,
		CacheReadTokens:     u.CacheReadTokens + o.CacheReadTokens,
		CacheCreationTokens: u.CacheCreationTokens + o.CacheCreationTokens,
	}
}
