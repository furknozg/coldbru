package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	sdk "github.com/anthropics/anthropic-sdk-go/v2"

	"github.com/furknozg/coldbru/internal/proposal/llm"
)

// Screen asks Haiku 5.5 whether the brief tries to override instructions, bypass consent, leak
// data or change the system's rules (D-026, D-069). The call is fail-closed: anything but a clean
// allow verdict is an error, so a caller that stops on err never lets an unscreened brief through.
func (c *Client) Screen(ctx context.Context, brief string) (llm.ScreenResult, error) {
	pkg := c.screen
	oc, err := outputConfig(pkg)
	if err != nil {
		return llm.ScreenResult{}, err
	}
	user, err := marshal(struct {
		Brief string `json:"brief"`
	}{brief})
	if err != nil {
		return llm.ScreenResult{}, errorf("screen: encode brief: %w", err)
	}
	cl := newCall(ctx, llm.PurposeScreen, pkg, 1)
	msg, err := c.api.Messages.New(ctx, sdk.MessageNewParams{
		Model:        sdk.Model(pkg.Model),
		MaxTokens:    ScreenMaxTokens,
		System:       systemBlocks(pkg),
		Thinking:     sdk.ThinkingConfigParamUnion{OfDisabled: &sdk.ThinkingConfigDisabledParam{}},
		OutputConfig: oc,
		Messages:     []sdk.MessageParam{userMessage(string(user))},
	})
	if err != nil {
		return llm.ScreenResult{}, cl.failed(nil, err)
	}
	// Branch on the stop reason before reading the content (structured outputs: refusal and
	// max_tokens may not match the schema).
	switch msg.StopReason {
	case sdk.StopReasonRefusal:
		return llm.ScreenResult{}, cl.refused(msg)
	case sdk.StopReasonEndTurn:
	default:
		return llm.ScreenResult{}, cl.invalid(msg, textOf(msg), "stopped with "+string(msg.StopReason))
	}
	text := textOf(msg)
	res, err := parseVerdict(text)
	if err != nil {
		return llm.ScreenResult{}, cl.invalid(msg, text, err.Error())
	}
	out, _ := marshal(struct {
		Verdict llm.Verdict `json:"verdict"`
		Reason  string      `json:"reason"`
	}{res.Verdict, res.Reason})
	cl.report(msg, llm.OutcomeAccepted, out, "")
	if res.Verdict == llm.VerdictBlock {
		return res, &llm.ScreenBlockedError{Reason: res.Reason}
	}
	return res, nil
}

// invalid records an output the adapter cannot use and returns the typed error.
func (c *call) invalid(msg *sdk.Message, text, reason string) error {
	c.report(msg, llm.OutcomeValidationFailed, storableOutput(text), reason)
	return &llm.InvalidOutputError{Purpose: c.purpose, StopReason: string(msg.StopReason), Reason: reason}
}

// parseVerdict reads {verdict, reason}. The schema guarantees the shape; the check stays because
// the guarantee is the API's, not ours (D-069: validate what the model returns). The enum is
// compared case-insensitively.
func parseVerdict(text string) (llm.ScreenResult, error) {
	var raw struct {
		Verdict *string `json:"verdict"`
		Reason  *string `json:"reason"`
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(text)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return llm.ScreenResult{}, fmt.Errorf("not the verdict object: %v", err)
	}
	if raw.Verdict == nil || raw.Reason == nil {
		return llm.ScreenResult{}, errors.New("verdict and reason are both required")
	}
	switch v := llm.Verdict(strings.ToLower(strings.TrimSpace(*raw.Verdict))); v {
	case llm.VerdictAllow, llm.VerdictBlock:
		return llm.ScreenResult{Verdict: v, Reason: strings.TrimSpace(*raw.Reason)}, nil
	default:
		return llm.ScreenResult{}, fmt.Errorf("unknown verdict %q", *raw.Verdict)
	}
}
