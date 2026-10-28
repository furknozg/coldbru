// Package flowtest holds the test doubles of the proposal flow (D-025, D-070): a fake
// ContentGenerator, which T-024 replaces with the real one, and a Confirmer that records its calls.
// Nothing here contacts a network.
package flowtest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/llm"
)

// Content is a fake flow.ContentGenerator. By default it streams two fragments and returns
// {"text": "<language>/<channel> #<n>"} where n counts the calls for that slot; it reports one
// accepted llm.CallRecord per call, as the real generator does through llm.StreamStructured.
// Safe for concurrent use.
type Content struct {
	// GenerateFunc, when set, replaces the default behaviour. It runs before the record is
	// reported; return the content and an error as the real generator would.
	GenerateFunc func(ctx context.Context, req flow.ContentRequest, onDelta func(attempt int, text string)) (json.RawMessage, error)
	// ValidateFunc, when set, replaces the default Validate, which accepts any JSON object with a
	// non-empty "text" string and returns it unchanged.
	ValidateFunc func(req flow.ContentRequest, content json.RawMessage) (json.RawMessage, error)

	mu       sync.Mutex
	requests []flow.ContentRequest
	counts   map[string]int
}

var _ flow.ContentGenerator = (*Content)(nil)

// fakeSHA is a well-formed prompt hash for the records the fake reports.
var fakeSHA = strings.Repeat("0", 64)

// Generate records req and returns the scripted or default content.
func (c *Content) Generate(ctx context.Context, req flow.ContentRequest, onDelta func(attempt int, text string)) (json.RawMessage, error) {
	c.mu.Lock()
	c.requests = append(c.requests, req)
	if c.counts == nil {
		c.counts = map[string]int{}
	}
	key := req.Language + "/" + req.Channel
	c.counts[key]++
	n := c.counts[key]
	c.mu.Unlock()

	start := time.Now()
	var (
		raw json.RawMessage
		err error
	)
	if c.GenerateFunc != nil {
		raw, err = c.GenerateFunc(ctx, req, onDelta)
	} else {
		body := fmt.Sprintf("%s #%d", key, n)
		if onDelta != nil {
			onDelta(1, `{"text":"`+key)
			onDelta(1, fmt.Sprintf(` #%d"}`, n))
		}
		raw = json.RawMessage(fmt.Sprintf(`{"text":%q}`, body))
	}
	rec := llm.CallRecord{
		Purpose: llm.PurposeContent, Model: "claude-sonnet-5-5", Attempt: 1, StopReason: "end_turn",
		LatencyMs: time.Since(start).Milliseconds(), Prompt: llm.PromptRef{Name: "content", Version: "0.0.0", SHA256: fakeSHA},
		Outcome: llm.OutcomeAccepted, Response: raw,
	}
	if err != nil {
		rec.Outcome, rec.Response, rec.Error = llm.OutcomeAPIError, nil, err.Error()
	}
	llm.Report(ctx, rec)
	return raw, err
}

// Validate accepts a JSON object with a non-empty "text" string and returns it unchanged, or runs
// ValidateFunc.
func (c *Content) Validate(req flow.ContentRequest, content json.RawMessage) (json.RawMessage, error) {
	if c.ValidateFunc != nil {
		return c.ValidateFunc(req, content)
	}
	var v struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(content, &v); err != nil || strings.TrimSpace(v.Text) == "" {
		return nil, llm.NewProblemsError([]string{`"text" must be a non-empty string`})
	}
	return content, nil
}

// Requests returns the requests Generate received, in order.
func (c *Content) Requests() []flow.ContentRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]flow.ContentRequest(nil), c.requests...)
}

// Confirmer is a flow.Confirmer that records the campaigns it was called for.
type Confirmer struct {
	// Err is returned by every call when set.
	Err error

	mu    sync.Mutex
	calls []uuid.UUID
}

var _ flow.Confirmer = (*Confirmer)(nil)

// Confirm records the call.
func (c *Confirmer) Confirm(_ context.Context, campaignID uuid.UUID) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, campaignID)
	return c.Err
}

// Calls returns the campaign ids Confirm was called with.
func (c *Confirmer) Calls() []uuid.UUID {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]uuid.UUID(nil), c.calls...)
}
