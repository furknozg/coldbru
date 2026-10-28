package anthropic_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/llmtest"
	"github.com/furknozg/coldbru/internal/proposal/prompts"
)

// contentPackage stands in for the content package of T-024: a schema and a Sonnet 5.5 model.
func contentPackage(t *testing.T) *prompts.Package {
	t.Helper()
	p, err := prompts.Build(
		prompts.Manifest{Name: "content", Version: "0.0.1", Model: "claude-sonnet-5-5", Effort: "low"},
		"You write the SMS and email of a campaign.",
		nil,
		[]byte(`{"type":"object","properties":{"sms":{"type":"object","properties":{"text":{"type":"string"}},"required":["text"],"additionalProperties":false}},"required":["sms"],"additionalProperties":false}`),
	)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

const contentDoc = `{"sms":{"text":"Merhaba {{first_name}}, sepetin seni bekliyor!"}}`

func TestStreamStructuredDeltasInOrderThenFinalJSON(t *testing.T) {
	t.Parallel()
	chunks := []string{`{"sms":`, `{"text":"Merhaba {{first_`, `name}}, sepetin seni `, `bekliyor!"}}`}
	srv := llmtest.NewMessagesServer(t, llmtest.SSE(llmtest.Message{
		// Between-tools thinking can put a (here empty) thinking block first; it is not output.
		Blocks: []llmtest.Block{llmtest.Thinking("", "sig"), llmtest.Text("", chunks...)},
		Usage:  llmtest.Usage{Input: 300, Output: 40, CacheRead: 250, CacheCreation: 5},
	}))
	var got []string
	ctx, col := withRecorder(t)
	raw, usage, err := newClient(t, srv).StreamStructured(ctx, contentPackage(t), []byte(`{"language":"tr"}`), func(s string) { got = append(got, s) }, nil)
	if err != nil {
		t.Fatalf("StreamStructured: %v", err)
	}
	if strings.Join(got, "|") != strings.Join(chunks, "|") {
		t.Errorf("deltas = %q, want %q in that order", got, chunks)
	}
	if string(raw) != contentDoc {
		t.Errorf("raw = %s", raw)
	}
	if want := (llm.Usage{InputTokens: 300, OutputTokens: 40, CacheReadTokens: 250, CacheCreationTokens: 5}); usage != want {
		t.Errorf("usage = %+v, want %+v", usage, want)
	}
	if body := srv.Requests()[0]; !body.Streaming() {
		t.Error("the request did not ask for a stream")
	} else if text, _ := field(t, body.JSON(), "messages", 0, "content", 0, "text").(string); text != `{"language":"tr"}` {
		t.Errorf("user message = %q", text)
	}
	recs := col.Records()
	if len(recs) != 1 || recs[0].Purpose != llm.PurposeContent || recs[0].Outcome != llm.OutcomeAccepted || recs[0].Attempt != 1 ||
		recs[0].Prompt.Name != "content" || string(recs[0].Response) != contentDoc || recs[0].StopReason != "end_turn" || recs[0].Usage != usage {
		t.Errorf("records = %+v", recs)
	}
}

func TestStreamStructuredNilCallbackIsFine(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, llmtest.SSE(msg(llmtest.Text(contentDoc))))
	raw, _, err := newClient(t, srv).StreamStructured(t.Context(), contentPackage(t), nil, nil, nil)
	if err != nil || string(raw) != contentDoc {
		t.Errorf("raw = %s, err = %v", raw, err)
	}
}

func TestStreamStructuredFailures(t *testing.T) {
	t.Parallel()
	partial := llmtest.Text(`{"sms":{"text":"Mer`)
	tests := []struct {
		name    string
		resp    llmtest.Response
		check   func(error) bool
		outcome llm.Outcome
		deltas  bool // deltas reached the callback before the failure
	}{
		{"refusal after output", llmtest.SSE(llmtest.Message{Refusal: &llmtest.Refusal{Category: "reasoning_extraction"}, Blocks: []llmtest.Block{partial}}),
			func(err error) bool {
				var re *llm.RefusalError
				return errors.As(err, &re) && re.Category == "reasoning_extraction" && re.Purpose == llm.PurposeContent
			}, llm.OutcomeRefusal, true},
		{"refusal before output", llmtest.SSE(llmtest.Message{Refusal: &llmtest.Refusal{Category: "bio"}}),
			func(err error) bool { return errors.Is(err, llm.ErrRefusal) }, llm.OutcomeRefusal, false},
		{"cut off by max_tokens", llmtest.SSE(llmtest.Message{StopReason: "max_tokens", Blocks: []llmtest.Block{partial}}),
			func(err error) bool {
				var ie *llm.InvalidOutputError
				return errors.As(err, &ie) && ie.StopReason == "max_tokens"
			}, llm.OutcomeValidationFailed, true},
		{"truncated JSON that ended normally", llmtest.SSE(msg(partial)),
			func(err error) bool {
				var ie *llm.InvalidOutputError
				return errors.As(err, &ie) && strings.Contains(ie.Reason, "not valid JSON")
			}, llm.OutcomeValidationFailed, true},
		{"empty answer", llmtest.SSE(llmtest.Message{}), func(err error) bool { return errors.Is(err, llm.ErrInvalidOutput) }, llm.OutcomeValidationFailed, false},
		{"error event mid-stream", llmtest.SSEError(msg(partial), "overloaded_error", "Overloaded"),
			func(err error) bool {
				var ae *llm.APIError
				return errors.As(err, &ae) && ae.Purpose == llm.PurposeContent && strings.Contains(ae.Error(), "Overloaded")
			}, llm.OutcomeAPIError, true},
		{"connection dropped before message_stop", llmtest.SSETruncated(msg(partial)),
			func(err error) bool {
				var ie *llm.InvalidOutputError
				return errors.As(err, &ie) && strings.Contains(ie.Reason, "stream ended")
			}, llm.OutcomeValidationFailed, true},
		{"HTTP 429", llmtest.HTTPError(http.StatusTooManyRequests, "rate_limit_error", "slow"),
			func(err error) bool {
				var ae *llm.APIError
				return errors.As(err, &ae) && ae.Status == 429 && ae.Retryable()
			}, llm.OutcomeAPIError, false},
		{"HTTP 529", llmtest.HTTPError(529, "overloaded_error", "busy"),
			func(err error) bool {
				var ae *llm.APIError
				return errors.As(err, &ae) && ae.Status == 529
			}, llm.OutcomeAPIError, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := llmtest.NewMessagesServer(t, tt.resp)
			var got []string
			ctx, col := withRecorder(t)
			raw, _, err := newClient(t, srv).StreamStructured(ctx, contentPackage(t), []byte(`{}`), func(s string) { got = append(got, s) }, nil)
			if err == nil || !tt.check(err) {
				t.Fatalf("err = %v", err)
			}
			if raw != nil {
				t.Errorf("raw = %s on error", raw)
			}
			if (len(got) > 0) != tt.deltas {
				t.Errorf("deltas = %q, want delivered = %v", got, tt.deltas)
			}
			recs := col.Records()
			if len(recs) != 1 || recs[0].Outcome != tt.outcome {
				t.Fatalf("records = %+v, want one with outcome %s", recs, tt.outcome)
			}
			if tt.outcome != llm.OutcomeAccepted && recs[0].Error == "" {
				t.Error("a failed record needs error text")
			}
		})
	}
}

func TestStreamStructuredKeepsUnparseableOutputAsAJSONString(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, llmtest.SSE(msg(llmtest.Text(`{"sms":{"text":"Mer`))))
	ctx, col := withRecorder(t)
	_, _, _ = newClient(t, srv).StreamStructured(ctx, contentPackage(t), nil, nil, nil)
	r := col.Records()[0]
	// llm_call.response is jsonb: the cut-off text must be stored as a JSON string, not as invalid JSON.
	var s string
	decode(t, string(r.Response), &s)
	if s != `{"sms":{"text":"Mer` {
		t.Errorf("response = %s", r.Response)
	}
}

func TestStreamStructuredRejectsUnusablePackages(t *testing.T) {
	t.Parallel()
	c := newClient(t, llmtest.NewMessagesServer(t))
	noSchema, _ := prompts.Build(prompts.Manifest{Name: "x", Version: "1.0.0", Model: "m", Effort: "low"}, "s", nil, nil)
	withTools := prompts.Segment()
	for name, pkg := range map[string]*prompts.Package{"nil": nil, "no schema": noSchema, "tools": withTools} {
		if _, _, err := c.StreamStructured(t.Context(), pkg, nil, nil, nil); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestStreamStructuredValidate(t *testing.T) {
	t.Parallel()
	tooLong := errors.New("sms.text is 181 characters, the limit is 160")
	tests := []struct {
		name     string
		validate func(json.RawMessage) error
		wantErr  error
		outcome  llm.Outcome
		errText  string
	}{
		{"nil validate is accepted", nil, nil, llm.OutcomeAccepted, ""},
		{"validate accepts", func(json.RawMessage) error { return nil }, nil, llm.OutcomeAccepted, ""},
		{"validate rejects", func(json.RawMessage) error { return tooLong }, tooLong, llm.OutcomeValidationFailed, tooLong.Error()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := llmtest.NewMessagesServer(t, llmtest.SSE(llmtest.Message{
				Blocks: []llmtest.Block{llmtest.Text(contentDoc)}, Usage: llmtest.Usage{Input: 9, Output: 3},
			}))
			var seen json.RawMessage
			validate := tt.validate
			if validate != nil {
				inner := validate
				validate = func(raw json.RawMessage) error { seen = raw; return inner(raw) }
			}
			ctx, col := withRecorder(t)
			raw, usage, err := newClient(t, srv).StreamStructured(ctx, contentPackage(t), []byte(`{}`), nil, validate)

			// The document and usage come back either way; a rejection also returns a typed error.
			if string(raw) != contentDoc || usage.InputTokens != 9 || usage.OutputTokens != 3 {
				t.Errorf("raw = %s, usage = %+v", raw, usage)
			}
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("err = %v", err)
				}
			} else {
				var ce *llm.ContentInvalidError
				if !errors.As(err, &ce) || !errors.Is(err, llm.ErrContentInvalid) || !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want *ContentInvalidError wrapping %v", err, tt.wantErr)
				}
			}
			if validate != nil && string(seen) != contentDoc {
				t.Errorf("validate saw %s", seen)
			}
			recs := col.Records()
			if len(recs) != 1 {
				t.Fatalf("records = %d, want exactly one, emitted after the outcome is known", len(recs))
			}
			r := recs[0]
			if r.Outcome != tt.outcome || r.Error != tt.errText || string(r.Response) != contentDoc || r.Attempt != 1 {
				t.Errorf("record = %+v", r)
			}
			if n := len(srv.Requests()); n != 1 {
				t.Errorf("requests = %d; no retry inside StreamStructured", n)
			}
		})
	}
}

func TestStreamStructuredDoesNotValidateAFailedStream(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, llmtest.SSE(llmtest.Message{Refusal: &llmtest.Refusal{Category: "bio"}}))
	called := false
	_, _, err := newClient(t, srv).StreamStructured(t.Context(), contentPackage(t), nil, nil, func(json.RawMessage) error { called = true; return nil })
	if !errors.Is(err, llm.ErrRefusal) || called {
		t.Errorf("err = %v, validate called = %v", err, called)
	}
}
