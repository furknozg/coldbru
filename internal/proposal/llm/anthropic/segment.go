package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/anthropics/anthropic-sdk-go/v2"

	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/dsl"
	"github.com/furknozg/coldbru/internal/proposal/llm"
)

// Tool names of the segment prompt package (segment/tools.json).
const (
	toolPropose = "propose_segment"
	toolCount   = "count_audience"
	toolAsk     = "ask_user"
	toolSummary = "send_summary"
)

// Nudges are user messages that follow a failed turn; the call-specific part of the conversation
// lives in the messages, never in the cached system text.
const (
	nudgeNoTool = "You did not call propose_segment. Do not answer in text: call propose_segment now with the final DSL filter."
	nudgeCutOff = "Your answer was cut off before it was complete. Think less, keep the filter short, and call propose_segment with the final DSL filter."
)

// summaryResult is the tool result of an accepted send_summary.
const summaryResult = `{"ok":true,"next":"The summary was sent to the marketer. Stop."}`

// segmentUser is the user message of the segment call: one JSON object (D-060, D-088).
type segmentUser struct {
	Brief          string          `json:"brief"`
	ContextNotes   []string        `json:"context_notes,omitempty"`
	Clarifications []clarification `json:"clarifications,omitempty"`
	QuestionsLeft  int             `json:"questions_left"`
}

// clarification is one answered question of the step, as the user message carries it (D-088).
type clarification struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

func clarificationsOf(in []llm.Clarification) []clarification {
	if len(in) == 0 {
		return nil
	}
	out := make([]clarification, len(in))
	for i, c := range in {
		out[i] = clarification{Question: c.Question, Answer: c.Answer}
	}
	return out
}

// toolCall is one tool_use of a turn as stored in llm_call.response.
type toolCall struct {
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ProposeSegment runs the segment tool loop (D-027, D-069, D-088). The model sees the brief, may
// probe audience sizes with count_audience and ends by calling propose_segment. Each propose_segment
// is validated by tools; a rejected one returns to the model as an is_error tool_result with every
// problem (code, position, near text, suggestions) so it repairs in the same conversation, with
// its thinking blocks passed back unchanged. At most llm.MaxProposeAttempts propose attempts and
// llm.MaxSegmentTurns model turns; a refusal stops at once.
//
// Two more tools (D-088): ask_user ends the loop with a question for the marketer, unless the step
// has no question left (in.QuestionsLeft is 0), in which case the call is rejected and counts as
// a propose attempt; send_summary, called in the same turn as the accepted propose_segment or in
// the turn after its result, is returned as Summary. A loop that ends without a summary still
// succeeds. A turn that holds both an accepted propose_segment and ask_user keeps the proposal.
func (c *Client) ProposeSegment(ctx context.Context, in llm.SegmentInput, tools llm.SegmentTools) (llm.SegmentResult, error) {
	pkg := c.segment
	oc, err := outputConfig(pkg)
	if err != nil {
		return llm.SegmentResult{}, err
	}
	user, err := marshal(segmentUser{
		Brief: in.Brief, ContextNotes: in.ContextNotes, Clarifications: clarificationsOf(in.Clarifications), QuestionsLeft: in.QuestionsLeft,
	})
	if err != nil {
		return llm.SegmentResult{}, errorf("segment: encode input: %w", err)
	}
	messages := []sdk.MessageParam{userMessage(string(user))}
	var (
		usage    llm.Usage
		proposes int
		lastErrs []*dsl.Error
		lastWhy  string
		// accepted is the proposal of an earlier turn that waits for its summary.
		accepted *llm.SegmentResult
	)
	exhausted := func() error {
		return &llm.RepairExhaustedError{Attempts: proposes, Errors: lastErrs, Reason: lastWhy}
	}
	finish := func(res *llm.SegmentResult, turn int) llm.SegmentResult {
		res.Turns, res.ProposeAttempts, res.Usage = turn, proposes, usage
		return *res
	}

	for turn := 1; turn <= llm.MaxSegmentTurns; turn++ {
		cl := newCall(ctx, llm.PurposeSegment, pkg, turn)
		msg, err := c.api.Messages.New(ctx, sdk.MessageNewParams{
			Model:        sdk.Model(pkg.Model),
			MaxTokens:    SegmentMaxTokens,
			System:       systemBlocks(pkg),
			Tools:        toolParams(pkg),
			ToolChoice:   sdk.ToolChoiceUnionParam{OfAuto: &sdk.ToolChoiceAutoParam{}},
			Thinking:     sdk.ThinkingConfigParamUnion{OfAdaptive: &sdk.ThinkingConfigAdaptiveParam{}},
			OutputConfig: oc,
			Messages:     messages,
		})
		if err != nil {
			if accepted != nil {
				// The proposal stands; only the summary turn failed.
				cl.report(nil, llm.OutcomeAPIError, nil, asAPIError(llm.PurposeSegment, err).Error())
				return finish(accepted, turn-1), nil
			}
			return llm.SegmentResult{}, cl.failed(nil, err)
		}
		usage = usage.Add(usageOf(msg.Usage))

		// Branch on the stop reason before reading content.
		if msg.StopReason == sdk.StopReasonRefusal {
			if accepted != nil {
				cl.report(msg, llm.OutcomeRefusal, nil, "refusal in the summary turn")
				return finish(accepted, turn), nil
			}
			return llm.SegmentResult{}, cl.refused(msg)
		}
		uses := toolUses(msg)

		if accepted != nil {
			// The summary turn: only send_summary matters; anything else ends the loop without one.
			var calls []toolCall
			for _, tu := range uses {
				calls = append(calls, toolCall{Name: tu.Name, Input: validJSON(tu.Input)})
				if tu.Name != toolSummary || accepted.Summary != "" {
					continue
				}
				if md, perr := parseSummaryInput(tu.Input); perr == nil {
					accepted.Summary = md
				}
			}
			cl.report(msg, llm.OutcomeAccepted, callsJSON(calls), "")
			return finish(accepted, turn), nil
		}

		if msg.StopReason == sdk.StopReasonMaxTokens {
			// A cut-off turn is a failed attempt; there is no retry with a higher limit. A tool_use
			// in it may be incomplete, so it is not sent back and not run.
			proposes++
			lastErrs, lastWhy = nil, "the answer was cut off at max_tokens"
			cl.report(msg, llm.OutcomeValidationFailed, nil, lastWhy)
			if proposes >= llm.MaxProposeAttempts {
				return llm.SegmentResult{}, exhausted()
			}
			if kept := withoutToolUse(msg); len(kept) > 0 {
				messages = append(messages, sdk.NewAssistantMessage(kept...))
			}
			messages = append(messages, sdk.NewUserMessage(sdk.NewTextBlock(nudgeCutOff)))
			continue
		}

		if len(uses) == 0 {
			// tool_choice is auto, so the model may answer in text. That counts as an attempt (D-027).
			proposes++
			lastErrs, lastWhy = nil, "the model ended its turn without calling propose_segment"
			var out json.RawMessage
			if text := textOf(msg); text != "" {
				out, _ = marshal(struct {
					Text string `json:"text"`
				}{truncateText(text)})
			}
			cl.report(msg, llm.OutcomeValidationFailed, out, lastWhy)
			if proposes >= llm.MaxProposeAttempts {
				return llm.SegmentResult{}, exhausted()
			}
			messages = append(messages, msg.ToParam(), sdk.NewUserMessage(sdk.NewTextBlock(nudgeNoTool)))
			continue
		}

		var (
			results  []sdk.ContentBlockParamUnion
			calls    []toolCall
			question *llm.Question
			summary  string
			failure  string
		)
		for _, tu := range uses {
			calls = append(calls, toolCall{Name: tu.Name, Input: validJSON(tu.Input)})
			switch tu.Name {
			case toolPropose:
				proposes++
				dslText, perr := parseDSLInput(tu.Input)
				if perr != nil {
					lastErrs, lastWhy, failure = nil, perr.Error(), perr.Error()
					results = append(results, sdk.NewToolResultBlock(tu.ID, perr.Error(), true))
					continue
				}
				f, p, errs, terr := tools.Validate(ctx, dslText)
				if terr != nil {
					cl.report(msg, llm.OutcomeAPIError, callsJSON(calls), "tools: "+terr.Error())
					return llm.SegmentResult{}, &llm.ToolsError{Err: terr}
				}
				if len(errs) > 0 {
					lastErrs, lastWhy, failure = errs, "", "propose_segment was rejected"
					results = append(results, sdk.NewToolResultBlock(tu.ID, llm.FormatErrors(errs), true))
					continue
				}
				if accepted == nil {
					accepted = &llm.SegmentResult{DSL: dslText, Filter: f, Preview: p, Turn: turn}
				}
				results = append(results, sdk.NewToolResultBlock(tu.ID, acceptedResult(p), false))
			case toolCount:
				dslText, perr := parseDSLInput(tu.Input)
				if perr != nil {
					failure = perr.Error()
					results = append(results, sdk.NewToolResultBlock(tu.ID, perr.Error(), true))
					continue
				}
				p, terr := tools.CountAudience(ctx, dslText)
				if terr != nil {
					errs := llm.RepairableErrors(terr)
					if len(errs) == 0 {
						cl.report(msg, llm.OutcomeAPIError, callsJSON(calls), "tools: "+terr.Error())
						return llm.SegmentResult{}, &llm.ToolsError{Err: terr}
					}
					failure = "count_audience was rejected"
					results = append(results, sdk.NewToolResultBlock(tu.ID, llm.FormatErrors(errs), true))
					continue
				}
				results = append(results, sdk.NewToolResultBlock(tu.ID, llm.FormatPreview(p), false))
			case toolAsk:
				q, perr := parseQuestionInput(tu.Input)
				if perr == nil && in.QuestionsLeft <= 0 {
					perr = errors.New(llm.QuestionLimitProblem)
				}
				if perr != nil {
					// A rejected question costs a propose attempt (D-069, D-088).
					proposes++
					lastErrs, lastWhy, failure = nil, perr.Error(), perr.Error()
					results = append(results, sdk.NewToolResultBlock(tu.ID, perr.Error(), true))
					continue
				}
				if question == nil {
					question = q
				}
				results = append(results, sdk.NewToolResultBlock(tu.ID, `{"ok":true,"next":"The question was sent to the marketer. Stop."}`, false))
			case toolSummary:
				md, perr := parseSummaryInput(tu.Input)
				if perr != nil {
					failure = perr.Error()
					results = append(results, sdk.NewToolResultBlock(tu.ID, perr.Error(), true))
					continue
				}
				if summary == "" {
					summary = md
				}
				results = append(results, sdk.NewToolResultBlock(tu.ID, summaryResult, false))
			default:
				failure = "unknown tool " + tu.Name
				results = append(results, sdk.NewToolResultBlock(tu.ID, "unknown tool "+tu.Name+"; use propose_segment, count_audience, ask_user or send_summary", true))
			}
		}

		if accepted != nil {
			cl.report(msg, llm.OutcomeAccepted, callsJSON(calls), "")
			accepted.Summary = summary
			if summary != "" || turn == llm.MaxSegmentTurns {
				return finish(accepted, turn), nil
			}
			// One more turn for send_summary (D-088); whatever it brings, the loop ends there.
			messages = append(messages, msg.ToParam(), sdk.NewUserMessage(results...))
			continue
		}
		if question != nil {
			// ask_user ends the loop: the step waits for the marketer (D-088).
			cl.report(msg, llm.OutcomeAccepted, callsJSON(calls), "")
			return finish(&llm.SegmentResult{Question: question}, turn), nil
		}
		outcome := llm.OutcomeAccepted
		if failure != "" {
			outcome = llm.OutcomeValidationFailed
		}
		cl.report(msg, outcome, callsJSON(calls), failure)
		if proposes >= llm.MaxProposeAttempts {
			return llm.SegmentResult{}, exhausted()
		}
		// The assistant turn goes back unchanged, thinking blocks included, then the results.
		messages = append(messages, msg.ToParam(), sdk.NewUserMessage(results...))
	}
	if accepted != nil {
		return finish(accepted, llm.MaxSegmentTurns), nil
	}
	return llm.SegmentResult{}, &llm.TurnLimitError{Turns: llm.MaxSegmentTurns}
}

// acceptedResult is the tool result of an accepted propose_segment: the audience it selects (the
// view of llm.FormatPreview) and what to do next.
func acceptedResult(p *customer.Preview) string {
	preview := "{}"
	if p != nil {
		preview = llm.FormatPreview(p)
	}
	return `{"accepted":true,"audience":` + preview + `,"next":"The filter is accepted. Call send_summary once with a short summary for the marketer, then stop."}`
}

func toolUses(msg *sdk.Message) []sdk.ToolUseBlock {
	var out []sdk.ToolUseBlock
	for _, b := range msg.Content {
		if tu, ok := b.AsAny().(sdk.ToolUseBlock); ok {
			out = append(out, tu)
		}
	}
	return out
}

// withoutToolUse returns the blocks of a cut-off turn that are safe to send back.
func withoutToolUse(msg *sdk.Message) []sdk.ContentBlockParamUnion {
	var out []sdk.ContentBlockParamUnion
	for _, b := range msg.Content {
		if b.Type == "tool_use" {
			continue
		}
		out = append(out, b.ToParam())
	}
	return out
}

// parseDSLInput reads the tool input {"dsl": "..."}. strict tools make the shape certain, but the
// check is ours to make (D-069); a problem goes back to the model as an error result.
func parseDSLInput(raw json.RawMessage) (string, error) {
	var in struct {
		DSL *string `json:"dsl"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return "", fmt.Errorf(`the tool input must be a JSON object {"dsl": "<expression>"}: %v`, err)
	}
	if in.DSL == nil || strings.TrimSpace(*in.DSL) == "" {
		return "", errors.New(`the tool input needs a non-empty "dsl" string`)
	}
	return *in.DSL, nil
}

// parseQuestionInput reads the ask_user input {"question": "...", "options": [...] | null} and
// checks it with llm.CheckQuestion (D-088).
func parseQuestionInput(raw json.RawMessage) (*llm.Question, error) {
	var in struct {
		Question *string   `json:"question"`
		Options  *[]string `json:"options"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return nil, fmt.Errorf(`the tool input must be a JSON object {"question": "<text>", "options": [...] or null}: %v`, err)
	}
	if in.Question == nil {
		return nil, errors.New(`the tool input needs a "question" string`)
	}
	q := &llm.Question{Text: *in.Question}
	if in.Options != nil {
		q.Options = *in.Options
	}
	if problems := llm.CheckQuestion(q); len(problems) > 0 {
		return nil, errors.New("the question was rejected: " + strings.Join(problems, "; "))
	}
	return q, nil
}

// parseSummaryInput reads the send_summary input {"markdown": "..."} and checks it with
// llm.CheckSummary (D-088).
func parseSummaryInput(raw json.RawMessage) (string, error) {
	var in struct {
		Markdown *string `json:"markdown"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return "", fmt.Errorf(`the tool input must be a JSON object {"markdown": "<text>"}: %v`, err)
	}
	if in.Markdown == nil {
		return "", errors.New(`the tool input needs a "markdown" string`)
	}
	md, problems := llm.CheckSummary(*in.Markdown)
	if len(problems) > 0 {
		return "", errors.New("the summary was rejected: " + strings.Join(problems, "; "))
	}
	return md, nil
}

// validJSON returns raw when it is JSON, otherwise a JSON string holding it, so a record never
// carries invalid JSON.
func validJSON(raw json.RawMessage) json.RawMessage {
	if json.Valid(raw) {
		return raw
	}
	b, _ := marshal(truncateText(string(raw)))
	return b
}

func callsJSON(calls []toolCall) json.RawMessage {
	b, err := marshal(calls)
	if err != nil {
		return nil
	}
	return b
}

func truncateText(s string) string {
	if len(s) > maxStoredText {
		s = s[:maxStoredText]
	}
	return strings.ToValidUTF8(s, "�")
}
