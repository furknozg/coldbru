package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/anthropics/anthropic-sdk-go/v2"

	"github.com/furknozg/coldbru/internal/proposal/dsl"
	"github.com/furknozg/coldbru/internal/proposal/llm"
)

// Tool names of the segment prompt package (segment/tools.json).
const (
	toolPropose = "propose_segment"
	toolCount   = "count_audience"
)

// Nudges are user messages that follow a failed turn; the call-specific part of the conversation
// lives in the messages, never in the cached system text.
const (
	nudgeNoTool = "You did not call propose_segment. Do not answer in text: call propose_segment now with the final DSL filter."
	nudgeCutOff = "Your answer was cut off before it was complete. Think less, keep the filter short, and call propose_segment with the final DSL filter."
)

// segmentUser is the user message of the segment call: one JSON object (D-060).
type segmentUser struct {
	Brief        string   `json:"brief"`
	ContextNotes []string `json:"context_notes,omitempty"`
}

// toolCall is one tool_use of a turn as stored in llm_call.response.
type toolCall struct {
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ProposeSegment runs the segment tool loop (D-027, D-069). The model sees the brief, may probe
// audience sizes with count_audience and ends by calling propose_segment. Each propose_segment is
// validated by tools; a rejected one returns to the model as an is_error tool_result with every
// problem (code, position, near text, suggestions) so it repairs in the same conversation, with
// its thinking blocks passed back unchanged. At most llm.MaxProposeAttempts propose attempts and
// llm.MaxSegmentTurns model turns; a refusal stops at once.
func (c *Client) ProposeSegment(ctx context.Context, in llm.SegmentInput, tools llm.SegmentTools) (llm.SegmentResult, error) {
	pkg := c.segment
	oc, err := outputConfig(pkg)
	if err != nil {
		return llm.SegmentResult{}, err
	}
	user, err := marshal(segmentUser{Brief: in.Brief, ContextNotes: in.ContextNotes})
	if err != nil {
		return llm.SegmentResult{}, errorf("segment: encode input: %w", err)
	}
	messages := []sdk.MessageParam{userMessage(string(user))}
	var (
		usage    llm.Usage
		proposes int
		lastErrs []*dsl.Error
		lastWhy  string
	)
	exhausted := func() error {
		return &llm.RepairExhaustedError{Attempts: proposes, Errors: lastErrs, Reason: lastWhy}
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
			return llm.SegmentResult{}, cl.failed(nil, err)
		}
		usage = usage.Add(usageOf(msg.Usage))

		// Branch on the stop reason before reading content.
		if msg.StopReason == sdk.StopReasonRefusal {
			return llm.SegmentResult{}, cl.refused(msg)
		}
		uses := toolUses(msg)

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
			accepted *llm.SegmentResult
			failure  string
		)
		for _, tu := range uses {
			calls = append(calls, toolCall{Name: tu.Name, Input: validJSON(tu.Input)})
			dslText, perr := parseDSLInput(tu.Input)
			switch tu.Name {
			case toolPropose:
				proposes++
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
					accepted = &llm.SegmentResult{DSL: dslText, Filter: f, Preview: p}
				}
			case toolCount:
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
			default:
				failure = "unknown tool " + tu.Name
				results = append(results, sdk.NewToolResultBlock(tu.ID, "unknown tool "+tu.Name+"; use propose_segment or count_audience", true))
			}
		}

		if accepted != nil {
			cl.report(msg, llm.OutcomeAccepted, callsJSON(calls), "")
			accepted.Turn, accepted.Turns, accepted.ProposeAttempts, accepted.Usage = turn, turn, proposes, usage
			return *accepted, nil
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
	return llm.SegmentResult{}, &llm.TurnLimitError{Turns: llm.MaxSegmentTurns}
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
