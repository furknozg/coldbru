package anthropic

import (
	"context"
	"encoding/json"
	"errors"

	sdk "github.com/anthropics/anthropic-sdk-go/v2"

	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/prompts"
)

// StreamStructured streams one structured-output call (D-027, D-069): Sonnet 5.5 with thinking
// between_tools (Sonnet 5.5 rejects disabled), the effort of pkg, and pkg's JSON schema as the
// output format. pkg must carry an output schema, no tools, and a model that accepts
// between_tools; the content package of T-024 is the intended caller.
//
// Text fragments reach onDelta as they arrive. The stop reason is checked before the JSON is
// returned: a refusal or a cut-off answer is an error, and the deltas already delivered are only a
// preview the caller must drop. raw is valid JSON. validate (may be nil) is then called with it
// to check the schema and the channel limits: nil accepts, an error records the call as
// validation_failed and is returned wrapped in *llm.ContentInvalidError together with raw and the
// usage. The record is emitted once, after the outcome is known; there is no retry here.
func (c *Client) StreamStructured(ctx context.Context, pkg *prompts.Package, userJSON []byte, onDelta func(text string), validate func(json.RawMessage) error) (json.RawMessage, llm.Usage, error) {
	switch {
	case pkg == nil:
		return nil, llm.Usage{}, errors.New("anthropic: StreamStructured: nil prompt package")
	case pkg.OutputSchema == nil:
		return nil, llm.Usage{}, errorf("StreamStructured: %s has no output schema", pkg.ID())
	case len(pkg.Tools) > 0:
		return nil, llm.Usage{}, errorf("StreamStructured: %s declares tools", pkg.ID())
	}
	oc, err := outputConfig(pkg)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	cl := newCall(ctx, llm.PurposeContent, pkg, 1)
	stream := c.api.Messages.NewStreaming(ctx, sdk.MessageNewParams{
		Model:        sdk.Model(pkg.Model),
		MaxTokens:    ContentMaxTokens,
		System:       systemBlocks(pkg),
		Thinking:     sdk.ThinkingConfigParamUnion{OfBetweenTools: &sdk.ThinkingConfigBetweenToolsParam{}},
		OutputConfig: oc,
		Messages:     []sdk.MessageParam{userMessage(string(userJSON))},
	})
	defer stream.Close()

	var msg sdk.Message
	for stream.Next() {
		ev := stream.Current()
		if err := msg.Accumulate(ev); err != nil {
			return nil, llm.Usage{}, cl.failed(&msg, err)
		}
		if ev.Type == "content_block_delta" && ev.Delta.Type == "text_delta" && onDelta != nil {
			onDelta(ev.Delta.Text)
		}
	}
	if err := stream.Err(); err != nil {
		// Includes an `error` event in the middle of the stream (for example overloaded_error).
		return nil, llm.Usage{}, cl.failed(&msg, err)
	}
	usage := usageOf(msg.Usage)

	switch msg.StopReason {
	case sdk.StopReasonRefusal:
		return nil, usage, cl.refused(&msg)
	case sdk.StopReasonEndTurn:
	case "":
		return nil, usage, cl.invalid(&msg, textOf(&msg), "the stream ended before the message was complete")
	default:
		return nil, usage, cl.invalid(&msg, textOf(&msg), "stopped with "+string(msg.StopReason))
	}
	text := textOf(&msg)
	if !json.Valid([]byte(text)) {
		return nil, usage, cl.invalid(&msg, text, "the answer is not valid JSON")
	}
	raw := json.RawMessage(text)
	if validate != nil {
		if verr := validate(raw); verr != nil {
			cl.report(&msg, llm.OutcomeValidationFailed, raw, verr.Error())
			return raw, usage, &llm.ContentInvalidError{Err: verr}
		}
	}
	cl.report(&msg, llm.OutcomeAccepted, raw, "")
	return raw, usage, nil
}
