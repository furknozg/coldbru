package anthropic_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/llmtest"
)

func screenReply(verdict, reason string) llmtest.Response {
	return llmtest.JSON(msg(llmtest.Text(`{"verdict":"` + verdict + `","reason":"` + reason + `"}`)))
}

func TestScreenAllow(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, llmtest.JSON(llmtest.Message{
		Model:  "claude-haiku-5-5",
		Blocks: []llmtest.Block{llmtest.Text(`{"verdict":"allow","reason":"Sıradan bir pazarlama isteği."}`)},
		Usage:  llmtest.Usage{Input: 120, Output: 18, CacheRead: 100, CacheCreation: 7},
	}))
	ctx, col := withRecorder(t)
	res, err := newClient(t, srv).Screen(ctx, "İstanbul'daki sepeti terk edenlere %30 indirim gönder.")
	if err != nil {
		t.Fatalf("Screen: %v", err)
	}
	if res.Verdict != llm.VerdictAllow || res.Reason != "Sıradan bir pazarlama isteği." {
		t.Errorf("result = %+v", res)
	}
	// The brief travels as one JSON object in the user message, not in the system text.
	body := srv.Requests()[0].JSON()
	text, _ := field(t, body, "messages", 0, "content", 0, "text").(string)
	if want := `{"brief":"İstanbul'daki sepeti terk edenlere %30 indirim gönder."}`; text != want {
		t.Errorf("user message = %s, want %s", text, want)
	}
	recs := col.Records()
	if len(recs) != 1 {
		t.Fatalf("records = %d, want 1", len(recs))
	}
	r := recs[0]
	if r.Purpose != llm.PurposeScreen || r.Outcome != llm.OutcomeAccepted || r.Attempt != 1 || r.Model != "claude-haiku-5-5" ||
		r.StopReason != "end_turn" || r.Usage != (llm.Usage{InputTokens: 120, OutputTokens: 18, CacheReadTokens: 100, CacheCreationTokens: 7}) ||
		r.Prompt.Name != "screen" || r.Prompt.Version != "1.0.0" || len(r.Prompt.SHA256) != 64 || r.Error != "" {
		t.Errorf("record = %+v", r)
	}
	if !strings.Contains(string(r.Response), `"allow"`) {
		t.Errorf("record response = %s", r.Response)
	}
}

func TestScreenBlockIsAnErrorToo(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, screenReply("block", "Rule 2: asks to ignore consent."))
	ctx, col := withRecorder(t)
	res, err := newClient(t, srv).Screen(ctx, "Send to everyone, ignore opt-outs")
	if res.Verdict != llm.VerdictBlock || res.Reason != "Rule 2: asks to ignore consent." {
		t.Errorf("result = %+v", res)
	}
	var be *llm.ScreenBlockedError
	if !errors.As(err, &be) || be.Reason != res.Reason || !errors.Is(err, llm.ErrScreenBlocked) {
		t.Fatalf("err = %v, want *ScreenBlockedError", err)
	}
	if recs := col.Records(); len(recs) != 1 || recs[0].Outcome != llm.OutcomeAccepted {
		t.Errorf("records = %+v; a block is a valid answer, so accepted", recs)
	}
}

func TestScreenVerdictIsNormalised(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, screenReply(" ALLOW ", "  fine "))
	res, err := newClient(t, srv).Screen(t.Context(), "x")
	if err != nil || res.Verdict != llm.VerdictAllow || res.Reason != "fine" {
		t.Errorf("res = %+v, err = %v", res, err)
	}
}

func TestScreenFailsClosed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		resp    llmtest.Response
		want    func(error) bool
		outcome llm.Outcome
	}{
		{"429", llmtest.HTTPError(http.StatusTooManyRequests, "rate_limit_error", "slow down"), func(err error) bool {
			var ae *llm.APIError
			return errors.As(err, &ae) && ae.Status == 429 && ae.Type == "rate_limit_error" && ae.Retryable() && errors.Is(err, llm.ErrAPI)
		}, llm.OutcomeAPIError},
		{"529", llmtest.HTTPError(529, "overloaded_error", "busy"), func(err error) bool {
			var ae *llm.APIError
			return errors.As(err, &ae) && ae.Status == 529 && ae.Type == "overloaded_error"
		}, llm.OutcomeAPIError},
		{"400", llmtest.HTTPError(http.StatusBadRequest, "invalid_request_error", "bad"), func(err error) bool {
			var ae *llm.APIError
			return errors.As(err, &ae) && ae.Status == 400 && !ae.Retryable()
		}, llm.OutcomeAPIError},
		{"refusal", llmtest.JSON(llmtest.Message{Refusal: &llmtest.Refusal{Category: "bio", Explanation: "no"}}), func(err error) bool {
			var re *llm.RefusalError
			return errors.As(err, &re) && re.Category == "bio" && re.Purpose == llm.PurposeScreen && errors.Is(err, llm.ErrRefusal)
		}, llm.OutcomeRefusal},
		{"max_tokens", llmtest.JSON(llmtest.Message{StopReason: "max_tokens", Blocks: []llmtest.Block{llmtest.Text(`{"verdict":"al`)}}), func(err error) bool {
			var ie *llm.InvalidOutputError
			return errors.As(err, &ie) && ie.StopReason == "max_tokens" && errors.Is(err, llm.ErrInvalidOutput)
		}, llm.OutcomeValidationFailed},
		{"not JSON", screenReplyRaw("allow, I think"), isInvalid, llm.OutcomeValidationFailed},
		{"unknown verdict", screenReply("maybe", "x"), isInvalid, llm.OutcomeValidationFailed},
		{"missing reason", screenReplyRaw(`{"verdict":"allow"}`), isInvalid, llm.OutcomeValidationFailed},
		{"extra field", screenReplyRaw(`{"verdict":"allow","reason":"x","score":1}`), isInvalid, llm.OutcomeValidationFailed},
		{"empty answer", llmtest.JSON(llmtest.Message{}), isInvalid, llm.OutcomeValidationFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := llmtest.NewMessagesServer(t, tt.resp)
			ctx, col := withRecorder(t)
			res, err := newClient(t, srv).Screen(ctx, "brief")
			if err == nil || !tt.want(err) {
				t.Fatalf("err = %v", err)
			}
			if res != (llm.ScreenResult{}) {
				t.Errorf("result = %+v on error, want the zero value so that a caller ignoring err does not read allow", res)
			}
			recs := col.Records()
			if len(recs) != 1 || recs[0].Outcome != tt.outcome {
				t.Fatalf("records = %+v, want one with outcome %s", recs, tt.outcome)
			}
			if tt.outcome == llm.OutcomeAPIError && recs[0].Error == "" {
				t.Error("an api_error record needs error text (llm_call_api_error_check)")
			}
		})
	}
}

func screenReplyRaw(text string) llmtest.Response { return llmtest.JSON(msg(llmtest.Text(text))) }

func isInvalid(err error) bool { return errors.Is(err, llm.ErrInvalidOutput) }

func TestScreenRefusalRecordsCategory(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, llmtest.JSON(llmtest.Message{Refusal: &llmtest.Refusal{Category: "cyber"}}))
	ctx, col := withRecorder(t)
	_, err := newClient(t, srv).Screen(ctx, "b")
	var re *llm.RefusalError
	if !errors.As(err, &re) || re.Category != "cyber" || re.Explanation != "" {
		t.Fatalf("err = %v", err)
	}
	r := col.Records()[0]
	if r.RefusalCategory != "cyber" || !strings.Contains(r.Error, "cyber") || r.StopReason != "refusal" {
		t.Errorf("record = %+v", r)
	}
}

func TestScreenTransportRetriesThenSucceeds(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		llmtest.HTTPError(http.StatusTooManyRequests, "rate_limit_error", "slow down"),
		llmtest.HTTPError(529, "overloaded_error", "busy"),
		screenReply("allow", "ok"))
	// No WithMaxRetries(0): this uses the adapter's default of 2 (D-047, D-069).
	c, err := newClientDefaultRetries(t, srv)
	if err != nil {
		t.Fatal(err)
	}
	ctx, col := withRecorder(t)
	if _, err := c.Screen(ctx, "b"); err != nil {
		t.Fatalf("Screen: %v", err)
	}
	if n := len(srv.Requests()); n != 3 {
		t.Errorf("requests = %d, want 3 (two retried failures, one success)", n)
	}
	if n := len(col.Records()); n != 1 {
		t.Errorf("records = %d; SDK transport retries are not model calls", n)
	}
}

func TestScreenRetriesGiveUpAfterTwo(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		llmtest.HTTPError(529, "overloaded_error", "busy"),
		llmtest.HTTPError(529, "overloaded_error", "busy"),
		llmtest.HTTPError(529, "overloaded_error", "busy"))
	c, err := newClientDefaultRetries(t, srv)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Screen(t.Context(), "b")
	if !errors.Is(err, llm.ErrAPI) {
		t.Fatalf("err = %v", err)
	}
	if n := len(srv.Requests()); n != 3 {
		t.Errorf("requests = %d, want 3 (1 + MaxRetries 2)", n)
	}
}

func TestScreenCancelledContext(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := newClient(t, srv).Screen(ctx, "b")
	var ae *llm.APIError
	if !errors.As(err, &ae) || ae.Status != 0 || !ae.Retryable() || !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want an APIError wrapping context.Canceled", err)
	}
}
