package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	sdk "github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/packages/param"

	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/prompts"
)

// maxStoredText caps the model text kept in a record when the output was not JSON.
const maxStoredText = 8 << 10

// systemBlocks is the system prompt with the cache breakpoint on its last block, which caches the
// tools that render before it as well (prompt caching: tools, system, messages).
func systemBlocks(p *prompts.Package) []sdk.TextBlockParam {
	return []sdk.TextBlockParam{{Text: p.System, CacheControl: sdk.NewCacheControlEphemeralParam()}}
}

// toolParams sends each tool definition as the canonical bytes of the package, so the request
// carries exactly what the hash covers.
func toolParams(p *prompts.Package) []sdk.ToolUnionParam {
	out := make([]sdk.ToolUnionParam, len(p.Tools))
	for i, t := range p.Tools {
		out[i] = param.Override[sdk.ToolUnionParam](json.RawMessage(t.JSON))
	}
	return out
}

// outputConfig sets the effort of the package and, when it has one, the structured output schema.
func outputConfig(p *prompts.Package) (sdk.OutputConfigParam, error) {
	oc := sdk.OutputConfigParam{Effort: sdk.OutputConfigEffort(p.Effort)}
	if p.OutputSchema != nil {
		dec := json.NewDecoder(bytes.NewReader(p.OutputSchema))
		dec.UseNumber()
		var schema map[string]any
		if err := dec.Decode(&schema); err != nil {
			return oc, errorf("%s: output schema: %w", p.ID(), err)
		}
		oc.Format = sdk.JSONOutputFormatParam{Schema: schema}
	}
	return oc, nil
}

func userMessage(text string) sdk.MessageParam { return sdk.NewUserMessage(sdk.NewTextBlock(text)) }

func ref(p *prompts.Package) llm.PromptRef {
	return llm.PromptRef{Name: p.Name, Version: p.Version, SHA256: p.SHA256}
}

func usageOf(u sdk.Usage) llm.Usage {
	return llm.Usage{
		InputTokens:         int(u.InputTokens),
		OutputTokens:        int(u.OutputTokens),
		CacheReadTokens:     int(u.CacheReadInputTokens),
		CacheCreationTokens: int(u.CacheCreationInputTokens),
	}
}

// marshal encodes v without HTML escaping, so Turkish text and quotes stay readable in logs and rows.
func marshal(v any) (json.RawMessage, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// asAPIError classifies a failed call.
func asAPIError(purpose llm.Purpose, err error) *llm.APIError {
	out := &llm.APIError{Purpose: purpose, Err: err}
	var ae *sdk.Error
	if errors.As(err, &ae) {
		out.Status = ae.StatusCode
		out.Type = string(ae.Type())
		out.RequestID = ae.RequestID
	}
	return out
}

// call is the bookkeeping of one model call: which prompt, which attempt, and when it started.
type call struct {
	ctx     context.Context
	purpose llm.Purpose
	pkg     *prompts.Package
	attempt int
	start   time.Time
}

func newCall(ctx context.Context, purpose llm.Purpose, pkg *prompts.Package, attempt int) *call {
	return &call{ctx: ctx, purpose: purpose, pkg: pkg, attempt: attempt, start: time.Now()}
}

// report sends the record of the call. msg may be nil (the call failed before a message existed).
func (c *call) report(msg *sdk.Message, outcome llm.Outcome, response json.RawMessage, errText string) {
	rec := llm.CallRecord{
		Purpose:   c.purpose,
		Model:     c.pkg.Model,
		Attempt:   c.attempt,
		LatencyMs: time.Since(c.start).Milliseconds(),
		Prompt:    ref(c.pkg),
		Outcome:   outcome,
		Response:  response,
		Error:     errText,
	}
	if msg != nil {
		rec.Usage = usageOf(msg.Usage)
		rec.StopReason = string(msg.StopReason)
		if msg.StopReason == sdk.StopReasonRefusal {
			rec.RefusalCategory = string(msg.StopDetails.Category)
		}
	}
	llm.Report(c.ctx, rec)
}

// failed records an API failure and returns the typed error.
func (c *call) failed(msg *sdk.Message, err error) error {
	ae := asAPIError(c.purpose, err)
	c.report(msg, llm.OutcomeAPIError, nil, ae.Error())
	return ae
}

// refused records a refusal and returns the typed error. There is no repair and no fallback (D-069).
func (c *call) refused(msg *sdk.Message) error {
	re := &llm.RefusalError{
		Purpose:     c.purpose,
		Category:    string(msg.StopDetails.Category),
		Explanation: msg.StopDetails.Explanation,
	}
	text := "refusal"
	if re.Category != "" {
		text += " (stop_details.category: " + re.Category + ")"
	}
	if re.Explanation != "" {
		text += ": " + re.Explanation
	}
	c.report(msg, llm.OutcomeRefusal, nil, text)
	return re
}

// textOf joins the text blocks of a message; thinking blocks are not part of the answer.
func textOf(msg *sdk.Message) string {
	var b strings.Builder
	for _, blk := range msg.Content {
		if blk.Type == "text" {
			b.WriteString(blk.Text)
		}
	}
	return b.String()
}

// storableOutput turns model text into a value for llm_call.response (jsonb): the text itself when
// it is JSON, otherwise a JSON string holding a bounded copy of it.
func storableOutput(text string) json.RawMessage {
	if text == "" {
		return nil
	}
	if json.Valid([]byte(text)) {
		return json.RawMessage(text)
	}
	if len(text) > maxStoredText {
		text = text[:maxStoredText]
	}
	b, err := marshal(strings.ToValidUTF8(text, "�"))
	if err != nil {
		return nil
	}
	return b
}
