// Package contentgen is the ContentGenerator of the proposal flow (D-009, D-070, D-071). For one
// (language, channel) slot it asks the model for the SMS or the email blocks with the content@1
// prompt package, validates the answer with package content, and, when the answer breaks a rule,
// repairs it: a new streamed call carries the previous output and the structured list of problems.
// There are three attempts in total (one plus two repairs); a cut-off or non-JSON answer
// (llm.InvalidOutputError) is a failed attempt like a rule violation; a refusal or an API failure
// stops at once. Every attempt is reported to the llm.Recorder of the context, so each one becomes an
// llm_call row.
//
// What is stored is built by package content from the validated blocks: for an SMS the text with
// its encoding, segment count, rendered length and estimated cost; for an email the blocks with the
// longest rendered lengths and the estimated cost. HTML is not stored; Notification and the preview
// render it from the blocks (content.RenderEmail).
package contentgen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/prompts"
)

// MaxAttempts is the number of model calls for one slot: one plus two repairs (D-071).
const MaxAttempts = 3

// Generator implements flow.ContentGenerator. It is safe for concurrent use.
type Generator struct {
	llm    llm.LLM
	pkg    *prompts.Package
	prices content.Prices
}

var _ flow.ContentGenerator = (*Generator)(nil)

// New returns a Generator that calls l with the content package and prices the estimates with prices.
func New(l llm.LLM, prices content.Prices) *Generator {
	return &Generator{llm: l, pkg: prompts.Content(), prices: prices}
}

// slot is what a slot's validation and measurement depend on.
type slot struct {
	language, channel string
	maxName           int
	recipients        int
}

func slotOf(req flow.ContentRequest) (slot, error) {
	if req.Language == "" {
		return slot{}, errors.New("contentgen: the request has no language")
	}
	ch := llm.Channel(req.Channel)
	if !ch.Valid() {
		return slot{}, fmt.Errorf("contentgen: unknown channel %q", req.Channel)
	}
	return slot{
		language: req.Language, channel: req.Channel,
		maxName: req.Audience.MaxFirstNameLengthOf(req.Language), recipients: req.Recipients,
	}, nil
}

// Generate writes the content of the slot of req (flow.ContentGenerator). The attempts are made in
// turn; onDelta (may be nil) gets the text fragments of every attempt as they arrive, with the
// 1-based attempt number, so after a repair the preview restarts. Every attempt starts with a call
// with an empty text, which tells the flow about the attempt before its first fragment (D-072).
// It returns the stored content of the first valid answer, or:
//
//   - *llm.RefusalError or *llm.APIError as the adapter returned them (no repair);
//   - *llm.RepairExhaustedError (Purpose content, Problems = the messages of the last attempt) when
//     MaxAttempts answers broke the rules or were unusable.
func (g *Generator) Generate(ctx context.Context, req flow.ContentRequest, onDelta func(attempt int, text string)) (json.RawMessage, error) {
	sl, err := slotOf(req)
	if err != nil {
		return nil, err
	}
	var (
		previous json.RawMessage
		problems []content.Problem
	)
	for attempt := 1; attempt <= MaxAttempts; attempt++ {
		user, err := g.userMessage(req, sl, attempt, previous, problems)
		if err != nil {
			return nil, err
		}
		var (
			built   json.RawMessage
			failure []content.Problem
		)
		acall := &llm.Collector{}
		var delta func(text string)
		if onDelta != nil {
			onDelta(attempt, "")
			delta = func(text string) { onDelta(attempt, text) }
		}
		raw, _, err := g.llm.StreamStructured(llm.WithRecorder(ctx, acall), g.pkg, user, delta, func(raw json.RawMessage) error {
			stored, ps := g.fromModel(raw, sl)
			if len(ps) > 0 {
				failure = ps
				return content.AsError(ps)
			}
			built = stored
			return nil
		})
		for _, rec := range acall.Records() {
			rec.Attempt = attempt // the adapter numbers every StreamStructured call 1
			llm.Report(ctx, rec)
		}
		if err == nil {
			return built, nil
		}
		var (
			invalid  *llm.ContentInvalidError
			unusable *llm.InvalidOutputError
		)
		switch {
		case errors.As(err, &invalid) && len(failure) > 0:
			previous, problems = raw, failure
		case errors.As(err, &unusable):
			// Cut off or not JSON: a failed attempt like any other. The adapter returns no raw; what
			// it received is in its record, as a JSON string.
			previous, problems = nil, []content.Problem{{Rule: content.RuleInvalidOutput}}
			if recs := acall.Records(); len(recs) > 0 {
				previous = recs[len(recs)-1].Response
			}
		default:
			return nil, err // a refusal or an API failure: no repair
		}
	}
	return nil, &llm.RepairExhaustedError{
		Purpose: llm.PurposeContent, Attempts: MaxAttempts, Problems: content.Messages(problems),
		Reason: "the answers broke the content rules",
	}
}

// Validate checks hand-edited content of the slot of req with the rules Generate applies and returns
// the content to store, measurements and cost recomputed (flow.ContentGenerator). content is the
// stored shape of the channel: {"text"} for sms, {"subject", "preheader", "headline", "paragraphs",
// "cta_label"} for email; the computed fields of a stored variant may be present and are ignored.
// A failure is an error whose llm.Problems are the messages of the problems.
func (g *Generator) Validate(req flow.ContentRequest, edited json.RawMessage) (json.RawMessage, error) {
	sl, err := slotOf(req)
	if err != nil {
		return nil, err
	}
	stored, problems := g.build(edited, sl, true)
	if len(problems) > 0 {
		return nil, llm.NewProblemsError(content.Messages(problems))
	}
	return stored, nil
}

// fromModel checks the document the model returned: exactly the requested channel is filled and the
// other is null (the schema asks for it, the server enforces it), then the blocks are validated.
func (g *Generator) fromModel(raw json.RawMessage, sl slot) (json.RawMessage, []content.Problem) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, []content.Problem{{Field: "$", Rule: ruleInvalidJSON, Got: err.Error()}}
	}
	var problems []content.Problem
	for key := range doc {
		if key != "sms" && key != "email" {
			problems = append(problems, content.Problem{Field: key, Rule: ruleUnknownField})
		}
	}
	for _, ch := range []string{"sms", "email"} {
		if _, ok := doc[ch]; !ok {
			problems = append(problems, content.Problem{Field: ch, Rule: ruleRequired})
		}
	}
	if len(problems) > 0 {
		return nil, problems
	}
	want, other := sl.channel, llm.Channel(sl.channel).Other()
	if isNull(doc[want]) {
		problems = append(problems, content.Problem{Field: want, Rule: ruleWrongChannel, Got: "null", Limit: "an object: " + want + " is the requested channel"})
	}
	if !isNull(doc[string(other)]) {
		problems = append(problems, content.Problem{Field: string(other), Rule: ruleWrongChannel, Got: "an object", Limit: "null: " + string(other) + " is not the requested channel"})
	}
	if len(problems) > 0 {
		return nil, problems
	}
	return g.build(doc[want], sl, false)
}

// Rules that only the document check of contentgen reports; the others live in package content.
const (
	ruleInvalidJSON  = "invalid_json"
	ruleUnknownField = "unknown_field"
	ruleRequired     = "required"
	ruleWrongChannel = "wrong_channel"
)

func isNull(raw json.RawMessage) bool { return bytes.Equal(bytes.TrimSpace(raw), []byte("null")) }

// smsIn and emailIn are the fields a channel object may carry. Pointers tell a missing field from
// an empty one. The computed fields of a stored variant are accepted when ignoreComputed is set (an
// edit that echoes the stored content) and ignored.
type smsIn struct {
	Text *string `json:"text"`
	computed
}

type emailIn struct {
	Subject    *string   `json:"subject"`
	Preheader  *string   `json:"preheader"`
	Headline   *string   `json:"headline"`
	Paragraphs *[]string `json:"paragraphs"`
	CTALabel   *string   `json:"cta_label"`
	computed
	MaxRenderedLengths json.RawMessage `json:"max_rendered_lengths"`
}

type computed struct {
	Encoding         json.RawMessage `json:"encoding"`
	Segments         json.RawMessage `json:"segments"`
	RenderedLength   json.RawMessage `json:"rendered_length"`
	EstimatedCostGBP json.RawMessage `json:"estimated_cost_gbp"`
}

func (c computed) present() bool {
	return c.Encoding != nil || c.Segments != nil || c.RenderedLength != nil || c.EstimatedCostGBP != nil
}

// build decodes one channel object, validates it and returns the content to store. A model answer
// must not carry computed fields (the schema has none), an edit may.
func (g *Generator) build(obj json.RawMessage, sl slot, ignoreComputed bool) (json.RawMessage, []content.Problem) {
	dec := json.NewDecoder(bytes.NewReader(obj))
	dec.DisallowUnknownFields()
	field := func(name string) string { return sl.channel + "." + name }
	switch sl.channel {
	case string(llm.ChannelSMS):
		var in smsIn
		if err := dec.Decode(&in); err != nil {
			return nil, []content.Problem{{Field: sl.channel, Rule: ruleInvalidJSON, Got: err.Error()}}
		}
		if !ignoreComputed && in.computed.present() {
			return nil, []content.Problem{{Field: sl.channel, Rule: ruleUnknownField, Got: "computed fields"}}
		}
		if in.Text == nil {
			return nil, []content.Problem{{Field: field("text"), Rule: ruleRequired}}
		}
		out, problems := content.BuildSMS(*in.Text, sl.maxName, sl.recipients, g.prices)
		if len(problems) > 0 {
			return nil, problems
		}
		return encode(out)
	default:
		var in emailIn
		if err := dec.Decode(&in); err != nil {
			return nil, []content.Problem{{Field: sl.channel, Rule: ruleInvalidJSON, Got: err.Error()}}
		}
		if !ignoreComputed && (in.computed.present() || in.MaxRenderedLengths != nil) {
			return nil, []content.Problem{{Field: sl.channel, Rule: ruleUnknownField, Got: "computed fields"}}
		}
		var missing []content.Problem
		for _, f := range []struct {
			name string
			nil  bool
		}{
			{"subject", in.Subject == nil}, {"preheader", in.Preheader == nil}, {"headline", in.Headline == nil},
			{"paragraphs", in.Paragraphs == nil}, {"cta_label", in.CTALabel == nil},
		} {
			if f.nil {
				missing = append(missing, content.Problem{Field: field(f.name), Rule: ruleRequired})
			}
		}
		if len(missing) > 0 {
			return nil, missing
		}
		out, problems := content.BuildEmail(content.Email{
			Subject: *in.Subject, Preheader: *in.Preheader, Headline: *in.Headline, Paragraphs: *in.Paragraphs, CTALabel: *in.CTALabel,
		}, sl.maxName, sl.recipients, g.prices)
		if len(problems) > 0 {
			return nil, problems
		}
		return encode(out)
	}
}

// encode marshals the stored content without escaping &, < and > (the rows stay readable).
func encode(v any) (json.RawMessage, []content.Problem) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, []content.Problem{{Field: "$", Rule: ruleInvalidJSON, Got: err.Error()}}
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
