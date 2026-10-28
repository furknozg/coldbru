package llmtest_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/furknozg/coldbru/internal/proposal/compile"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/dsl"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/llmtest"
	"github.com/furknozg/coldbru/internal/proposal/prompts"
)

// post sends a body to the server and returns status and body.
func post(t *testing.T, srv *llmtest.MessagesServer, path, body string) (int, http.Header, string) {
	t.Helper()
	resp, err := http.Post(srv.URL+path, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, string(b)
}

type sseEvent struct {
	Name string
	Data map[string]any
}

func events(t *testing.T, body string) []sseEvent {
	t.Helper()
	var out []sseEvent
	sc := bufio.NewScanner(strings.NewReader(body))
	var cur sseEvent
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			cur = sseEvent{Name: strings.TrimPrefix(line, "event: ")}
		case strings.HasPrefix(line, "data: "):
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &cur.Data); err != nil {
				t.Fatalf("bad data line %q: %v", line, err)
			}
			out = append(out, cur)
		}
	}
	return out
}

func names(evs []sseEvent) string {
	n := make([]string, len(evs))
	for i, e := range evs {
		n[i] = e.Name
	}
	return strings.Join(n, " ")
}

func TestJSONMessage(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, llmtest.JSON(llmtest.Message{
		Blocks: []llmtest.Block{llmtest.Thinking("hm", "sig"), llmtest.Text("hi"), llmtest.ToolUse("tu_1", "count_audience", map[string]any{"dsl": "x"})},
		Usage:  llmtest.Usage{Input: 1, Output: 2, CacheRead: 3, CacheCreation: 4},
	}))
	status, hdr, body := post(t, srv, "/v1/messages", `{"stream":false}`)
	if status != 200 || !strings.HasPrefix(hdr.Get("Content-Type"), "application/json") {
		t.Fatalf("status %d content type %q", status, hdr.Get("Content-Type"))
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatal(err)
	}
	content := m["content"].([]any)
	if m["stop_reason"] != "tool_use" || len(content) != 3 || content[0].(map[string]any)["signature"] != "sig" ||
		content[2].(map[string]any)["input"].(map[string]any)["dsl"] != "x" || m["model"] != "claude-sonnet-5-5" {
		t.Errorf("message = %s", body)
	}
	u := m["usage"].(map[string]any)
	if u["input_tokens"] != float64(1) || u["output_tokens"] != float64(2) || u["cache_read_input_tokens"] != float64(3) || u["cache_creation_input_tokens"] != float64(4) {
		t.Errorf("usage = %v", u)
	}
	if reqs := srv.Requests(); len(reqs) != 1 || reqs[0].Streaming() || string(reqs[0].Body) != `{"stream":false}` || reqs[0].Method != "POST" {
		t.Errorf("recorded = %+v", reqs)
	}
}

func TestStopReasonDefaults(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		llmtest.JSON(llmtest.Message{Blocks: []llmtest.Block{llmtest.Text("x")}}),
		llmtest.JSON(llmtest.Message{StopReason: "max_tokens", Blocks: []llmtest.Block{llmtest.Text("x")}}),
		llmtest.JSON(llmtest.Message{StopReason: "end_turn", Refusal: &llmtest.Refusal{Category: "bio", Explanation: "e"}}),
		llmtest.JSON(llmtest.Message{Refusal: &llmtest.Refusal{}}),
	)
	want := []string{"end_turn", "max_tokens", "refusal", "refusal"}
	for i, w := range want {
		_, _, body := post(t, srv, "/v1/messages", `{}`)
		var m map[string]any
		_ = json.Unmarshal([]byte(body), &m)
		if m["stop_reason"] != w {
			t.Errorf("response %d: stop_reason = %v, want %s", i, m["stop_reason"], w)
		}
		if i == 2 {
			sd := m["stop_details"].(map[string]any)
			if sd["category"] != "bio" || sd["explanation"] != "e" || sd["type"] != "refusal" {
				t.Errorf("stop_details = %v", sd)
			}
		}
		if i == 3 {
			sd := m["stop_details"].(map[string]any)
			if sd["category"] != nil || sd["explanation"] != nil {
				t.Errorf("empty refusal stop_details = %v", sd)
			}
		}
	}
}

func TestSSEEventSequence(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, llmtest.SSE(llmtest.Message{
		Blocks: []llmtest.Block{llmtest.Text("", "ab", "cd"), llmtest.ToolUse("tu", "propose_segment", map[string]any{"dsl": "city"})},
		Usage:  llmtest.Usage{Input: 7, Output: 9},
	}))
	status, hdr, body := post(t, srv, "/v1/messages", `{"stream":true}`)
	if status != 200 || hdr.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("status %d type %q", status, hdr.Get("Content-Type"))
	}
	evs := events(t, body)
	want := "message_start content_block_start ping content_block_delta content_block_delta content_block_stop " +
		"content_block_start content_block_delta content_block_delta content_block_stop message_delta message_stop"
	if got := names(evs); got != want {
		t.Fatalf("events:\n got %s\nwant %s", got, want)
	}
	if d := evs[3].Data["delta"].(map[string]any); d["type"] != "text_delta" || d["text"] != "ab" {
		t.Errorf("first delta = %v", d)
	}
	var partial string
	for _, e := range evs[7:9] {
		partial += e.Data["delta"].(map[string]any)["partial_json"].(string)
	}
	if partial != `{"dsl":"city"}` {
		t.Errorf("tool input fragments join to %q", partial)
	}
	md := evs[10].Data
	if md["delta"].(map[string]any)["stop_reason"] != "tool_use" || md["usage"].(map[string]any)["output_tokens"] != float64(9) {
		t.Errorf("message_delta = %v", md)
	}
}

func TestSSEDefaultChunkingKeepsRunesWhole(t *testing.T) {
	t.Parallel()
	text := "Merhaba İstanbul, şehrin ğüzel!"
	srv := llmtest.NewMessagesServer(t, llmtest.SSE(llmtest.Message{Blocks: []llmtest.Block{llmtest.Text(text)}}))
	_, _, body := post(t, srv, "/v1/messages", `{"stream":true}`)
	var got string
	n := 0
	for _, e := range events(t, body) {
		if e.Name == "content_block_delta" {
			n++
			got += e.Data["delta"].(map[string]any)["text"].(string)
		}
	}
	if got != text || n < 3 {
		t.Errorf("deltas joined to %q in %d pieces", got, n)
	}
}

func TestSSEThinkingBlockStream(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, llmtest.SSE(llmtest.Message{Blocks: []llmtest.Block{llmtest.Thinking("t", "s")}}))
	_, _, body := post(t, srv, "/v1/messages", `{"stream":true}`)
	evs := events(t, body)
	if d := evs[3].Data["delta"].(map[string]any); d["type"] != "thinking_delta" {
		t.Errorf("delta = %v", d)
	}
	if d := evs[4].Data["delta"].(map[string]any); d["type"] != "signature_delta" || d["signature"] != "s" {
		t.Errorf("delta = %v", d)
	}
}

func TestSSETruncatedAndMidStreamError(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		llmtest.SSETruncated(llmtest.Message{Blocks: []llmtest.Block{llmtest.Text("abc")}}),
		llmtest.SSEError(llmtest.Message{Blocks: []llmtest.Block{llmtest.Text("abc")}}, "overloaded_error", "Overloaded"),
	)
	_, _, body := post(t, srv, "/v1/messages", `{"stream":true}`)
	if got := names(events(t, body)); got != "message_start content_block_start ping content_block_delta" {
		t.Errorf("truncated stream = %s", got)
	}
	_, _, body = post(t, srv, "/v1/messages", `{"stream":true}`)
	evs := events(t, body)
	last := evs[len(evs)-1]
	if last.Name != "error" || last.Data["error"].(map[string]any)["type"] != "overloaded_error" {
		t.Errorf("last event = %+v", last)
	}
}

func TestHTTPError(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, llmtest.HTTPError(429, "rate_limit_error", "slow down"))
	status, hdr, body := post(t, srv, "/v1/messages", `{}`)
	if status != 429 || hdr.Get("Retry-After-Ms") != "1" || !strings.Contains(body, `"rate_limit_error"`) {
		t.Errorf("status %d headers %v body %s", status, hdr, body)
	}
}

func TestServerRejectsOtherPathsAndFlagsUnscriptedRequests(t *testing.T) {
	t.Parallel()
	rec := &errorRecorder{TB: t}
	srv := llmtest.NewMessagesServer(rec, llmtest.JSON(llmtest.Message{}))
	if status, _, _ := post(t, srv, "/v1/other", `{}`); status != 404 {
		t.Errorf("other path = %d", status)
	}
	// The script had one response and the 404 request consumed it; the next request is unscripted.
	status, _, body := post(t, srv, "/v1/messages", `{}`)
	if status != 400 || !strings.Contains(body, "no scripted response left") {
		t.Errorf("status %d body %s", status, body)
	}
	if len(rec.errs) == 0 {
		t.Error("an unscripted request must fail the test")
	}
	srv.Enqueue(llmtest.JSON(llmtest.Message{}))
	if status, _, _ := post(t, srv, "/v1/messages", `{}`); status != 200 {
		t.Errorf("enqueued response not served: %d", status)
	}
}

type errorRecorder struct {
	testing.TB
	errs []string
}

func (e *errorRecorder) Errorf(format string, args ...any) { e.errs = append(e.errs, format) }

func TestRequestHelpers(t *testing.T) {
	t.Parallel()
	if got := (llmtest.Request{Body: []byte(`nope`)}).JSON(); got != nil {
		t.Errorf("JSON of garbage = %v", got)
	}
	if (llmtest.Request{Body: []byte(`{}`)}).Streaming() {
		t.Error("Streaming on an empty body")
	}
}

func TestStreamQueueValidate(t *testing.T) {
	t.Parallel()
	bad := errors.New("too long")
	s := llmtest.NewScripted().StreamJSON(`{"a":1}`).StreamJSON(`{"a":2}`).StreamJSON(`{"a":3}`)
	var seen []string
	check := func(raw json.RawMessage) error { seen = append(seen, string(raw)); return nil }
	if raw, _, err := s.StreamStructured(t.Context(), nil, nil, nil, check); err != nil || string(raw) != `{"a":1}` {
		t.Errorf("accepted: %s, %v", raw, err)
	}
	raw, _, err := s.StreamStructured(t.Context(), nil, nil, nil, func(json.RawMessage) error { return bad })
	var ce *llm.ContentInvalidError
	if !errors.As(err, &ce) || !errors.Is(err, bad) || !errors.Is(err, llm.ErrContentInvalid) || string(raw) != `{"a":2}` {
		t.Errorf("rejected: %s, %v; the document comes back with the error", raw, err)
	}
	if raw, _, err := s.StreamStructured(t.Context(), nil, nil, nil, nil); err != nil || string(raw) != `{"a":3}` {
		t.Errorf("nil validate: %s, %v", raw, err)
	}
	if len(seen) != 1 || seen[0] != `{"a":1}` {
		t.Errorf("validate saw %v", seen)
	}
}

func TestScreenQueue(t *testing.T) {
	t.Parallel()
	boom := errors.New("api down")
	s := llmtest.NewScripted().AllowScreen().BlockScreen("rule 1").FailScreen(boom)
	res, err := s.Screen(t.Context(), "a")
	if err != nil || res.Verdict != llm.VerdictAllow {
		t.Errorf("first = %+v, %v", res, err)
	}
	res, err = s.Screen(t.Context(), "b")
	if res.Verdict != llm.VerdictBlock || !errors.Is(err, llm.ErrScreenBlocked) {
		t.Errorf("second = %+v, %v", res, err)
	}
	res, err = s.Screen(t.Context(), "c")
	if res != (llm.ScreenResult{}) || !errors.Is(err, boom) {
		t.Errorf("third = %+v, %v", res, err)
	}
	if _, err = s.Screen(t.Context(), "d"); !errors.Is(err, llmtest.ErrScriptExhausted) {
		t.Errorf("fourth err = %v", err)
	}
	if got := s.ScreenBriefs(); strings.Join(got, ",") != "a,b,c,d" {
		t.Errorf("briefs = %v", got)
	}
}

type previewer struct{ count int }

func (p previewer) Preview(context.Context, *compile.Filter) (*customer.Preview, error) {
	return &customer.Preview{Count: p.count}, nil
}

func TestSegmentQueueInvalidThenValid(t *testing.T) {
	t.Parallel()
	tools := llm.NewCustomerTools(previewer{count: 5})
	s := llmtest.NewScripted().ProposeDSLs(`citty === 1`, `city === "Ankara"`)
	res, err := s.ProposeSegment(t.Context(), llm.SegmentInput{Brief: "b", ContextNotes: []string{"n"}}, tools)
	if err != nil || res.DSL != `city === "Ankara"` || res.ProposeAttempts != 2 || res.Preview.Count != 5 || res.Filter == nil {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if in := s.SegmentInputs(); len(in) != 1 || in[0].Brief != "b" || in[0].ContextNotes[0] != "n" {
		t.Errorf("inputs = %+v", in)
	}
}

func TestSegmentQueueExhausts(t *testing.T) {
	t.Parallel()
	tools := llm.NewCustomerTools(previewer{count: 5})
	s := llmtest.NewScripted().ProposeDSLs(`a ===`, `b ===`, `c ===`, `city === "Ankara"`)
	_, err := s.ProposeSegment(t.Context(), llm.SegmentInput{}, tools)
	var ex *llm.RepairExhaustedError
	if !errors.As(err, &ex) || ex.Attempts != llm.MaxProposeAttempts || len(ex.Errors) == 0 || ex.Errors[0].Code != dsl.CodeSyntaxError {
		t.Errorf("err = %v; the fake stops after %d attempts like the real loop", err, llm.MaxProposeAttempts)
	}
	// Fewer DSLs than attempts: exhausted with what was tried.
	s.ProposeDSLs(`a ===`)
	_, err = s.ProposeSegment(t.Context(), llm.SegmentInput{}, tools)
	if !errors.As(err, &ex) || ex.Attempts != 1 {
		t.Errorf("err = %v", err)
	}
}

func TestSegmentQueueBackendFailure(t *testing.T) {
	t.Parallel()
	boom := errors.New("down")
	tools := &llmtest.Tools{ValidateFunc: func(context.Context, string) (*compile.Filter, *customer.Preview, []*dsl.Error, error) {
		return nil, nil, nil, boom
	}}
	_, err := llmtest.NewScripted().ProposeDSLs("x").ProposeSegment(t.Context(), llm.SegmentInput{}, tools)
	if !errors.Is(err, llm.ErrTools) || !errors.Is(err, boom) {
		t.Errorf("err = %v", err)
	}
}

func TestSegmentQueueDirect(t *testing.T) {
	t.Parallel()
	want := llm.SegmentResult{DSL: "d", Turn: 3}
	s := llmtest.NewScripted().ReplySegment(want).FailSegment(&llm.TurnLimitError{Turns: 8})
	if got, err := s.ProposeSegment(t.Context(), llm.SegmentInput{}, nil); err != nil || got.DSL != "d" || got.Turn != 3 {
		t.Errorf("got = %+v, %v", got, err)
	}
	if _, err := s.ProposeSegment(t.Context(), llm.SegmentInput{}, nil); !errors.Is(err, llm.ErrTurnLimit) {
		t.Errorf("err = %v", err)
	}
	if _, err := s.ProposeSegment(t.Context(), llm.SegmentInput{}, nil); !errors.Is(err, llmtest.ErrScriptExhausted) {
		t.Errorf("err = %v", err)
	}
}

func TestStreamQueue(t *testing.T) {
	t.Parallel()
	boom := errors.New("overloaded")
	s := llmtest.NewScripted().StreamJSON(`{"a":`, `1}`).FailStream(boom, `{"a`).FailStream(boom)
	pkg := prompts.Screen()
	var got []string
	raw, _, err := s.StreamStructured(t.Context(), pkg, []byte(`{"x":1}`), func(d string) { got = append(got, d) }, nil)
	if err != nil || string(raw) != `{"a":1}` || strings.Join(got, "|") != `{"a":|1}` {
		t.Errorf("raw = %s, deltas = %v, err = %v", raw, got, err)
	}
	got = nil
	if raw, _, err = s.StreamStructured(t.Context(), pkg, nil, func(d string) { got = append(got, d) }, nil); !errors.Is(err, boom) || raw != nil || len(got) != 1 {
		t.Errorf("raw = %s, deltas = %v, err = %v; a mid-stream failure delivers the fragments first", raw, got, err)
	}
	if _, _, err = s.StreamStructured(t.Context(), pkg, nil, nil, nil); !errors.Is(err, boom) {
		t.Errorf("err = %v", err)
	}
	if _, _, err = s.StreamStructured(t.Context(), pkg, nil, nil, nil); !errors.Is(err, llmtest.ErrScriptExhausted) {
		t.Errorf("err = %v", err)
	}
	calls := s.StreamCalls()
	if len(calls) != 4 || calls[0].Package != pkg || string(calls[0].UserJSON) != `{"x":1}` {
		t.Errorf("calls = %+v", calls)
	}
}

func TestScriptedReportsAttachedRecords(t *testing.T) {
	t.Parallel()
	rec := llm.CallRecord{Purpose: llm.PurposeScreen, Outcome: llm.OutcomeAccepted, Attempt: 1}
	srec := llm.CallRecord{Purpose: llm.PurposeSegment, Outcome: llm.OutcomeAccepted, Attempt: 2}
	crec := llm.CallRecord{Purpose: llm.PurposeContent, Outcome: llm.OutcomeAccepted, Attempt: 1}
	s := llmtest.NewScripted()
	s.AllowScreen().WithRecords(llm.PurposeScreen, rec)
	s.ReplySegment(llm.SegmentResult{}).WithRecords(llm.PurposeSegment, srec)
	s.StreamJSON(`{}`).WithRecords(llm.PurposeContent, crec)
	col := &llm.Collector{}
	ctx := llm.WithRecorder(t.Context(), col)
	_, _ = s.Screen(ctx, "b")
	_, _ = s.ProposeSegment(ctx, llm.SegmentInput{}, nil)
	_, _, _ = s.StreamStructured(ctx, nil, nil, nil, nil)
	got := col.Records()
	if len(got) != 3 || got[0].Purpose != llm.PurposeScreen || got[1].Attempt != 2 || got[2].Purpose != llm.PurposeContent {
		t.Errorf("records = %+v", got)
	}
}

func TestToolsDefaultsAndOverrides(t *testing.T) {
	t.Parallel()
	var tools llmtest.Tools
	f, p, errs, err := tools.Validate(t.Context(), "x")
	if f != nil || p == nil || p.Count != 10 || len(errs) != 0 || err != nil {
		t.Errorf("Validate = %v %v %v %v", f, p, errs, err)
	}
	if p, err := tools.CountAudience(t.Context(), "y"); err != nil || p.Count != 10 {
		t.Errorf("CountAudience = %v %v", p, err)
	}
	if v, c := tools.Validated(), tools.Counted(); len(v) != 1 || v[0] != "x" || len(c) != 1 || c[0] != "y" {
		t.Errorf("recorded %v %v", v, c)
	}
}
