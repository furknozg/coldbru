package anthropic

import (
	"context"
	"strings"

	sdk "github.com/anthropics/anthropic-sdk-go/v2"

	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/prompts"
)

// repairLoop describes one structured call whose answer is decoded and validated in code (D-070).
type repairLoop[T any] struct {
	purpose   llm.Purpose
	pkg       *prompts.Package
	maxTokens int64
	// user is the user message: one JSON object with the call-specific data (D-060).
	user string
	// decode turns the model text into a value; its error lists the problems with the answer.
	decode func(text string) (T, []string)
	// validate is the caller's check (may be nil); its error is turned into problems.
	validate func(T) error
}

// repairIntro opens the user message that follows a rejected answer. It is the adapter's fixed
// text and not part of the prompt hash, like the nudges of the segment loop (D-069).
const repairIntro = "Your answer was rejected. Fix every problem below and answer again with the complete JSON object only:"

// repairMessage renders problems as the text of the corrective user message.
func repairMessage(problems []string) string {
	var b strings.Builder
	b.WriteString(repairIntro)
	for _, p := range problems {
		b.WriteString("\n- " + p)
	}
	return b.String()
}

// run makes at most llm.MaxProposeAttempts calls in one conversation. A rejected answer goes back
// as the assistant turn, followed by a user message that lists the problems; a refusal stops at
// once. Every call is reported once to the recorder in ctx. attempts is the number of calls made.
func (l repairLoop[T]) run(ctx context.Context, c *Client) (val T, attempts int, usage llm.Usage, err error) {
	var zero T
	oc, err := outputConfig(l.pkg)
	if err != nil {
		return zero, 0, usage, err
	}
	messages := []sdk.MessageParam{userMessage(l.user)}
	var last []string
	for attempt := 1; attempt <= llm.MaxProposeAttempts; attempt++ {
		cl := newCall(ctx, l.purpose, l.pkg, attempt)
		msg, err := c.api.Messages.New(ctx, sdk.MessageNewParams{
			Model:        sdk.Model(l.pkg.Model),
			MaxTokens:    l.maxTokens,
			System:       systemBlocks(l.pkg),
			Thinking:     sdk.ThinkingConfigParamUnion{OfDisabled: &sdk.ThinkingConfigDisabledParam{}},
			OutputConfig: oc,
			Messages:     messages,
		})
		if err != nil {
			return zero, attempt, usage, cl.failed(nil, err)
		}
		usage = usage.Add(usageOf(msg.Usage))

		// Branch on the stop reason before reading the content (structured outputs: refusal and
		// max_tokens may not match the schema).
		if msg.StopReason == sdk.StopReasonRefusal {
			return zero, attempt, usage, cl.refused(msg)
		}
		text := textOf(msg)
		var problems []string
		if msg.StopReason == sdk.StopReasonEndTurn {
			v, dp := l.decode(text)
			if len(dp) == 0 && l.validate != nil {
				if verr := l.validate(v); verr != nil {
					dp = llm.Problems(verr)
				}
			}
			if len(dp) == 0 {
				cl.report(msg, llm.OutcomeAccepted, storableOutput(text), "")
				return v, attempt, usage, nil
			}
			problems = dp
		} else {
			problems = []string{"the answer was cut off (stop reason " + string(msg.StopReason) + "); keep it short and complete"}
		}
		cl.report(msg, llm.OutcomeValidationFailed, storableOutput(text), strings.Join(problems, "; "))
		last = problems
		if attempt == llm.MaxProposeAttempts {
			return zero, attempt, usage, &llm.RepairExhaustedError{Purpose: l.purpose, Attempts: attempt, Problems: last}
		}
		if len(msg.Content) > 0 {
			messages = append(messages, msg.ToParam())
		}
		messages = append(messages, userMessage(repairMessage(problems)))
	}
	return zero, llm.MaxProposeAttempts, usage, &llm.RepairExhaustedError{Purpose: l.purpose, Attempts: llm.MaxProposeAttempts, Problems: last}
}
