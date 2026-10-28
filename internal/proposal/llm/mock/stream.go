package mock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/prompts"
)

// streamChunks is how many pieces StreamStructured cuts a document into.
const streamChunks = 5

// request is the part of the content user message (D-071) the mock reads.
type request struct {
	Language           string          `json:"language"`
	Channel            string          `json:"channel"`
	Brief              string          `json:"brief"`
	MaxFirstNameLength int             `json:"max_first_name_length"`
	Limits             *content.Limits `json:"limits"`
	Repair             json.RawMessage `json:"repair"`
	ContextNotes       []string        `json:"context_notes"`
	RejectionReason    string          `json:"rejection_reason"`
}

// StreamStructured writes the content of the requested language and channel from fixed templates
// (see the package comment), delivers it in a few chunks, and validates it like a real call. pkg
// should be the content package; the mock only takes its identity for the record. When the request
// carries a rejection reason or a context note the text gets the "[mock]" marker. After a repair
// request (the user message carries "repair") the mock takes the plainest text that fits.
func (m *LLM) StreamStructured(ctx context.Context, pkg *prompts.Package, userJSON []byte, onDelta func(text string), validate func(raw json.RawMessage) error) (json.RawMessage, llm.Usage, error) {
	if err := cancelled(ctx, llm.PurposeContent); err != nil {
		return nil, llm.Usage{}, err
	}
	if pkg == nil {
		return nil, llm.Usage{}, errors.New("mock: StreamStructured: nil prompt package")
	}
	started := time.Now()
	var req request
	if err := json.Unmarshal(userJSON, &req); err != nil {
		return nil, llm.Usage{}, fmt.Errorf("mock: StreamStructured: the user message is not a content request: %w", err)
	}
	doc, err := buildDocument(req)
	if err != nil {
		return nil, llm.Usage{}, err
	}
	raw := marshal(doc)

	for _, chunk := range split(string(raw), streamChunks) {
		if onDelta != nil {
			onDelta(chunk)
		}
		if err := m.pause(ctx); err != nil {
			record(ctx, llm.PurposeContent, pkg, started, llm.OutcomeAPIError, nil, err.Error())
			return nil, llm.Usage{}, &llm.APIError{Purpose: llm.PurposeContent, Err: err}
		}
	}
	if validate != nil {
		if verr := validate(raw); verr != nil {
			record(ctx, llm.PurposeContent, pkg, started, llm.OutcomeValidationFailed, raw, verr.Error())
			return raw, llm.Usage{}, &llm.ContentInvalidError{Err: verr}
		}
	}
	record(ctx, llm.PurposeContent, pkg, started, llm.OutcomeAccepted, raw, "")
	return raw, llm.Usage{}, nil
}

// pause waits for the chunk delay or the end of ctx.
func (m *LLM) pause(ctx context.Context) error {
	if m.chunkDelay <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(m.chunkDelay)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// split cuts s into at most n non-empty pieces on rune boundaries.
func split(s string, n int) []string {
	runes := []rune(s)
	if n < 1 || len(runes) <= n {
		return []string{s}
	}
	size := (len(runes) + n - 1) / n
	var out []string
	for i := 0; i < len(runes); i += size {
		out = append(out, string(runes[i:min(i+size, len(runes))]))
	}
	return out
}

// document is the content@1 answer: exactly one of the channels is filled.
type document struct {
	SMS   *smsDoc   `json:"sms"`
	Email *emailDoc `json:"email"`
}

type smsDoc struct {
	Text string `json:"text"`
}

type emailDoc struct {
	Subject    string   `json:"subject"`
	Preheader  string   `json:"preheader"`
	Headline   string   `json:"headline"`
	Paragraphs []string `json:"paragraphs"`
	CTALabel   string   `json:"cta_label"`
}

// buildDocument picks the template of the brief and language and fits it to the limits.
func buildDocument(req request) (document, error) {
	c, _ := Match(req.Brief)
	text := c.EN
	greeting, button, tail := "Hi "+content.FirstNameToken+",", "See the offer", "Use the button below to see the offer."
	if req.Language == "tr" {
		text = c.TR
		greeting, button, tail = "Merhaba "+content.FirstNameToken+",", "Kampanyayı gör", "Kampanyayı görmek için aşağıdaki butona tıkla."
	}
	// A regeneration (a rejection reason or a context note) says so in the text. The marker is part
	// of the canned text, so the fitting below keeps every field within the limits.
	if (req.RejectionReason != "" || len(req.ContextNotes) > 0) && !strings.HasPrefix(text.Line, mockTag) {
		text.Line, text.Head = mockTag+" "+text.Line, mockTag+" "+text.Head
	}
	plain := len(req.Repair) > 0 && string(req.Repair) != "null"
	maxName := req.MaxFirstNameLength

	switch req.Channel {
	case string(llm.ChannelSMS):
		candidates := []string{content.FirstNameToken + ", " + text.Line, text.Line, truncate(text.Line, content.SMSMaxChars)}
		if plain {
			candidates = candidates[1:]
		}
		for _, cand := range candidates {
			if _, problems := content.CheckSMS(cand, maxName); len(problems) == 0 {
				return document{SMS: &smsDoc{Text: cand}}, nil
			}
		}
		return document{SMS: &smsDoc{Text: truncate(text.Line, content.SMSMaxChars)}}, nil
	case string(llm.ChannelEmail):
		subjects := []string{content.FirstNameToken + ", " + text.Head, text.Head}
		if plain {
			subjects = subjects[1:]
		}
		for _, subject := range subjects {
			e := content.Email{
				Subject: subject, Preheader: truncate(text.Line, content.PreheaderMax), Headline: truncate(text.Head, content.HeadlineMax),
				Paragraphs: []string{greeting, text.Line, tail}, CTALabel: button,
			}
			if plain {
				e.Paragraphs = []string{text.Line, tail}
			}
			if _, problems := content.CheckEmail(e, maxName); len(problems) == 0 {
				return document{Email: &emailDoc{
					Subject: e.Subject, Preheader: e.Preheader, Headline: e.Headline, Paragraphs: e.Paragraphs, CTALabel: e.CTALabel,
				}}, nil
			}
		}
		return document{Email: &emailDoc{
			Subject: truncate(text.Head, content.SubjectMax), Preheader: truncate(text.Line, content.PreheaderMax),
			Headline: truncate(text.Head, content.HeadlineMax), Paragraphs: []string{text.Line}, CTALabel: button,
		}}, nil
	}
	return document{}, fmt.Errorf("mock: StreamStructured: unknown channel %q", req.Channel)
}

// truncate cuts s to at most n characters.
func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n]))
}
