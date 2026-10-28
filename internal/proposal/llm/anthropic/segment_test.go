package anthropic_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/furknozg/coldbru/internal/proposal/compile"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/dsl"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/llmtest"
)

const sampleDSL = `city === "İstanbul" && cartItems.some(c => c.addedAt >= ago("30d")) && orders.none(o => o.orderDate >= ago("30d"))`

var brief = llm.SegmentInput{Brief: "Son 30 günde sepete ürün ekleyip alışveriş yapmayan İstanbul'daki kullanıcılara %30 indirim hatırlatması gönder."}

func TestSegmentValidFirstTry(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, llmtest.JSON(llmtest.Message{
		Blocks: []llmtest.Block{llmtest.Thinking("", "sig-1"), proposeBlock("toolu_1", sampleDSL)},
		Usage:  llmtest.Usage{Input: 900, Output: 60, CacheRead: 800, CacheCreation: 10},
	}))
	tools := llm.NewCustomerTools(previewOf(46))
	ctx, col := withRecorder(t)
	in := llm.SegmentInput{Brief: brief.Brief, ContextNotes: []string{"previous proposal was too wide"}}
	res, err := newClient(t, srv).ProposeSegment(ctx, in, tools)
	if err != nil {
		t.Fatalf("ProposeSegment: %v", err)
	}
	if res.DSL != sampleDSL || res.Filter == nil || res.Preview == nil || res.Preview.Count != 46 {
		t.Errorf("result = %+v", res)
	}
	if res.Turn != 1 || res.Turns != 1 || res.ProposeAttempts != 1 ||
		res.Usage != (llm.Usage{InputTokens: 900, OutputTokens: 60, CacheReadTokens: 800, CacheCreationTokens: 10}) {
		t.Errorf("accounting = turn %d, turns %d, attempts %d, usage %+v", res.Turn, res.Turns, res.ProposeAttempts, res.Usage)
	}
	if f, _ := compile.Compile(sampleDSL); string(mustJSON(t, res.Filter)) != string(mustJSON(t, f)) {
		t.Error("the filter is not the compiled DSL")
	}

	// The user message is a JSON block with the brief and the notes; nothing else is in it.
	var user struct {
		Brief        string   `json:"brief"`
		ContextNotes []string `json:"context_notes"`
	}
	text, _ := field(t, srv.Requests()[0].JSON(), "messages", 0, "content", 0, "text").(string)
	decode(t, text, &user)
	if user.Brief != in.Brief || !reflect.DeepEqual(user.ContextNotes, in.ContextNotes) {
		t.Errorf("user message = %+v", user)
	}

	recs := col.Records()
	if len(recs) != 1 {
		t.Fatalf("records = %d, want 1", len(recs))
	}
	r := recs[0]
	if r.Purpose != llm.PurposeSegment || r.Attempt != 1 || r.Outcome != llm.OutcomeAccepted || r.Model != "claude-sonnet-5-5" ||
		r.StopReason != "tool_use" || r.Prompt.Name != "segment" || r.Prompt.Version != "1.0.0" {
		t.Errorf("record = %+v", r)
	}
	var calls []struct {
		Name  string `json:"name"`
		Input struct {
			DSL string `json:"dsl"`
		} `json:"input"`
	}
	decode(t, string(r.Response), &calls)
	if len(calls) != 1 || calls[0].Name != "propose_segment" || calls[0].Input.DSL != sampleDSL {
		t.Errorf("record response = %s", r.Response)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := marshalJSON(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestSegmentRepairsInTheSameConversation(t *testing.T) {
	t.Parallel()
	bad := `citty === "İstanbul"`
	srv := llmtest.NewMessagesServer(t,
		llmtest.JSON(llmtest.Message{Blocks: []llmtest.Block{llmtest.Thinking("", "sig-A"), llmtest.Text("Drafting.", "Draft", "ing."), proposeBlock("toolu_bad", bad)}}),
		llmtest.JSON(llmtest.Message{Blocks: []llmtest.Block{llmtest.Thinking("", "sig-B"), proposeBlock("toolu_good", sampleDSL)}}),
	)
	ctx, col := withRecorder(t)
	res, err := newClient(t, srv).ProposeSegment(ctx, brief, llm.NewCustomerTools(previewOf(46)))
	if err != nil {
		t.Fatalf("ProposeSegment: %v", err)
	}
	if res.DSL != sampleDSL || res.Turn != 2 || res.ProposeAttempts != 2 {
		t.Errorf("result = turn %d attempts %d dsl %q", res.Turn, res.ProposeAttempts, res.DSL)
	}
	reqs := srv.Requests()
	if len(reqs) != 2 {
		t.Fatalf("requests = %d, want 2", len(reqs))
	}

	// The second request is the first plus the assistant turn, unchanged, plus the tool result.
	first, second := messagesOf(t, reqs[0]), messagesOf(t, reqs[1])
	if len(first) != 1 || len(second) != 3 {
		t.Fatalf("messages = %d then %d, want 1 then 3", len(first), len(second))
	}
	if !reflect.DeepEqual(first[0], second[0]) {
		t.Error("the user message changed between turns")
	}
	if field(t, second[1], "role") != "assistant" {
		t.Fatalf("message 1 = %v", second[1])
	}
	assistant := field(t, second[1], "content").([]any)
	if len(assistant) != 3 || field(t, assistant[0], "type") != "thinking" || field(t, assistant[0], "signature") != "sig-A" ||
		field(t, assistant[1], "text") != "Drafting." || field(t, assistant[2], "id") != "toolu_bad" {
		t.Errorf("assistant content was not passed back unchanged: %v", assistant)
	}

	// The tool_result is an error that carries what the model needs to fix the filter.
	id, text, isErr := lastToolResult(t, reqs[1])
	if id != "toolu_bad" || !isErr {
		t.Fatalf("tool_result id %q is_error %v", id, isErr)
	}
	var fb struct {
		Error  string `json:"error"`
		Errors []struct {
			Code        string   `json:"code"`
			Line        int      `json:"line"`
			Column      int      `json:"column"`
			Message     string   `json:"message"`
			Near        string   `json:"near"`
			Suggestions []string `json:"suggestions"`
		} `json:"errors"`
	}
	decode(t, text, &fb)
	if len(fb.Errors) != 1 {
		t.Fatalf("errors = %+v", fb.Errors)
	}
	e := fb.Errors[0]
	if e.Code != "UNKNOWN_FIELD" || e.Line != 1 || e.Column != 1 || e.Near != "citty" || e.Message == "" || !contains(e.Suggestions, "city") {
		t.Errorf("error = %+v", e)
	}

	recs := col.Records()
	if len(recs) != 2 || recs[0].Attempt != 1 || recs[0].Outcome != llm.OutcomeValidationFailed || recs[1].Attempt != 2 || recs[1].Outcome != llm.OutcomeAccepted {
		t.Fatalf("records = %+v", recs)
	}
	if !strings.Contains(recs[0].Error, "rejected") {
		t.Errorf("record error = %q", recs[0].Error)
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func TestSegmentCountAudienceRoundTrip(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		llmtest.JSON(msg(countBlock("toolu_c", `city === "Ankara"`))),
		llmtest.JSON(msg(proposeBlock("toolu_p", `city === "Ankara"`))),
	)
	tools := &llmtest.Tools{CountFunc: func(context.Context, string) (*customer.Preview, error) {
		return &customer.Preview{Count: 7, Coverage: customer.Coverage{Both: 3, EmailOnly: 2, SmsOnly: 2, Unreachable: 1}}, nil
	}}
	ctx, col := withRecorder(t)
	res, err := newClient(t, srv).ProposeSegment(ctx, brief, tools)
	if err != nil {
		t.Fatalf("ProposeSegment: %v", err)
	}
	if res.Turn != 2 || res.ProposeAttempts != 1 || res.Turns != 2 {
		t.Errorf("turn %d turns %d attempts %d; a probe is a turn but not an attempt", res.Turn, res.Turns, res.ProposeAttempts)
	}
	if got := tools.Counted(); len(got) != 1 || got[0] != `city === "Ankara"` {
		t.Errorf("counted = %q", got)
	}
	id, text, isErr := lastToolResult(t, srv.Requests()[1])
	if id != "toolu_c" || isErr {
		t.Fatalf("tool_result id %q is_error %v", id, isErr)
	}
	var p customer.Preview
	decode(t, text, &p)
	if p.Count != 7 || p.Coverage.Unreachable != 1 {
		t.Errorf("preview in the tool_result = %+v", p)
	}
	recs := col.Records()
	if len(recs) != 2 || recs[0].Outcome != llm.OutcomeAccepted || recs[1].Outcome != llm.OutcomeAccepted || recs[0].Attempt != 1 || recs[1].Attempt != 2 {
		t.Errorf("records = %+v", recs)
	}
}

func TestSegmentCountAudienceRejectedDSLComesBackAsError(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		llmtest.JSON(msg(countBlock("toolu_c", "city =="))),
		llmtest.JSON(msg(proposeBlock("toolu_p", sampleDSL))),
	)
	ctx, col := withRecorder(t)
	if _, err := newClient(t, srv).ProposeSegment(ctx, brief, llm.NewCustomerTools(previewOf(5))); err != nil {
		t.Fatalf("ProposeSegment: %v", err)
	}
	_, text, isErr := lastToolResult(t, srv.Requests()[1])
	if !isErr || !strings.Contains(text, "UNSUPPORTED_OPERATOR") {
		t.Errorf("is_error %v text %s", isErr, text)
	}
	if recs := col.Records(); recs[0].Outcome != llm.OutcomeValidationFailed {
		t.Errorf("first record = %+v", recs[0])
	}
}

func TestSegmentRepairExhaustedAfterThreeProposes(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		llmtest.JSON(msg(proposeBlock("t1", `citty === "x"`))),
		llmtest.JSON(msg(proposeBlock("t2", `city == "x"`))),
		llmtest.JSON(msg(proposeBlock("t3", `retired === 1`))),
	)
	ctx, col := withRecorder(t)
	_, err := newClient(t, srv).ProposeSegment(ctx, brief, llm.NewCustomerTools(previewOf(5)))
	var ex *llm.RepairExhaustedError
	if !errors.As(err, &ex) || !errors.Is(err, llm.ErrRepairExhausted) {
		t.Fatalf("err = %v, want RepairExhaustedError", err)
	}
	if ex.Attempts != 3 || len(ex.Errors) != 1 || ex.Errors[0].Code != dsl.CodeTypeMismatch {
		t.Errorf("exhausted = %+v; the last errors must be those of the third attempt", ex)
	}
	if n := len(srv.Requests()); n != 3 {
		t.Errorf("requests = %d, want 3: no fourth call after the third failure", n)
	}
	recs := col.Records()
	if len(recs) != 3 {
		t.Fatalf("records = %d", len(recs))
	}
	for i, r := range recs {
		if r.Attempt != i+1 || r.Outcome != llm.OutcomeValidationFailed {
			t.Errorf("record %d = attempt %d outcome %s", i, r.Attempt, r.Outcome)
		}
	}
}

func TestSegmentEndTurnWithoutToolCountsAsAnAttempt(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		llmtest.JSON(msg(llmtest.Thinking("", "sig-1"), llmtest.Text("The filter is city === \"Ankara\"."))),
		llmtest.JSON(msg(proposeBlock("t", `city === "Ankara"`))),
	)
	ctx, col := withRecorder(t)
	res, err := newClient(t, srv).ProposeSegment(ctx, brief, &llmtest.Tools{})
	if err != nil {
		t.Fatalf("ProposeSegment: %v", err)
	}
	if res.ProposeAttempts != 2 || res.Turn != 2 {
		t.Errorf("attempts %d turn %d, want 2 and 2", res.ProposeAttempts, res.Turn)
	}
	// The second request carries the assistant turn unchanged and a plain user nudge.
	msgs := messagesOf(t, srv.Requests()[1])
	if len(msgs) != 3 || field(t, msgs[1], "content", 0, "type") != "thinking" ||
		!strings.Contains(field(t, msgs[2], "content", 0, "text").(string), "propose_segment") {
		t.Errorf("messages = %v", msgs)
	}
	recs := col.Records()
	if recs[0].Outcome != llm.OutcomeValidationFailed || !strings.Contains(recs[0].Error, "without calling propose_segment") ||
		!strings.Contains(string(recs[0].Response), "Ankara") {
		t.Errorf("first record = %+v", recs[0])
	}
}

func TestSegmentThreeTextAnswersExhaust(t *testing.T) {
	t.Parallel()
	text := llmtest.JSON(msg(llmtest.Text("Here you go.")))
	srv := llmtest.NewMessagesServer(t, text, text, text)
	_, err := newClient(t, srv).ProposeSegment(t.Context(), brief, &llmtest.Tools{})
	var ex *llm.RepairExhaustedError
	if !errors.As(err, &ex) || ex.Attempts != 3 || len(ex.Errors) != 0 || !strings.Contains(ex.Reason, "without calling propose_segment") {
		t.Fatalf("err = %v", err)
	}
	if n := len(srv.Requests()); n != 3 {
		t.Errorf("requests = %d, want 3", n)
	}
}

func TestSegmentTurnLimit(t *testing.T) {
	t.Parallel()
	var script []llmtest.Response
	for range llm.MaxSegmentTurns + 2 {
		script = append(script, llmtest.JSON(msg(countBlock("t", `city === "Ankara"`))))
	}
	srv := llmtest.NewMessagesServer(t, script...)
	ctx, col := withRecorder(t)
	_, err := newClient(t, srv).ProposeSegment(ctx, brief, &llmtest.Tools{})
	var tl *llm.TurnLimitError
	if !errors.As(err, &tl) || !errors.Is(err, llm.ErrTurnLimit) || tl.Turns != 8 {
		t.Fatalf("err = %v, want TurnLimitError{8}", err)
	}
	if n := len(srv.Requests()); n != llm.MaxSegmentTurns {
		t.Errorf("requests = %d, want %d", n, llm.MaxSegmentTurns)
	}
	if n := len(col.Records()); n != llm.MaxSegmentTurns {
		t.Errorf("records = %d", n)
	}
	// Dropping the unused script entries keeps the server from flagging leftovers; none are required.
}

func TestSegmentRefusalStopsAtOnce(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		llmtest.JSON(msg(countBlock("t", `city === "Ankara"`))),
		llmtest.JSON(llmtest.Message{Refusal: &llmtest.Refusal{Category: "general_harms", Explanation: "declined"}, Usage: llmtest.Usage{Input: 5}}),
	)
	ctx, col := withRecorder(t)
	_, err := newClient(t, srv).ProposeSegment(ctx, brief, &llmtest.Tools{})
	var re *llm.RefusalError
	if !errors.As(err, &re) || re.Category != "general_harms" || re.Purpose != llm.PurposeSegment {
		t.Fatalf("err = %v", err)
	}
	if n := len(srv.Requests()); n != 2 {
		t.Errorf("requests = %d; a refusal must not be repaired or retried", n)
	}
	recs := col.Records()
	last := recs[len(recs)-1]
	if last.Outcome != llm.OutcomeRefusal || last.RefusalCategory != "general_harms" || !strings.Contains(last.Error, "general_harms") || last.Attempt != 2 || last.Usage.InputTokens != 5 {
		t.Errorf("record = %+v", last)
	}
}

func TestSegmentMaxTokensIsAFailedAttempt(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		llmtest.JSON(llmtest.Message{StopReason: "max_tokens", Blocks: []llmtest.Block{llmtest.Thinking("", "sig"), proposeBlock("t_cut", `city === "An`)}}),
		llmtest.JSON(msg(proposeBlock("t", `city === "Ankara"`))),
	)
	ctx, col := withRecorder(t)
	res, err := newClient(t, srv).ProposeSegment(ctx, brief, &llmtest.Tools{})
	if err != nil {
		t.Fatalf("ProposeSegment: %v", err)
	}
	if res.ProposeAttempts != 2 {
		t.Errorf("attempts = %d, want 2", res.ProposeAttempts)
	}
	reqs := srv.Requests()
	if got := reqs[1].JSON()["max_tokens"]; got != float64(4096) {
		t.Errorf("max_tokens on the second try = %v: there is no retry with a higher limit", got)
	}
	// A cut-off tool_use is neither run nor sent back.
	for _, m := range messagesOf(t, reqs[1]) {
		if strings.Contains(string(mustJSON(t, m)), "t_cut") {
			t.Errorf("cut-off tool_use was sent back: %v", m)
		}
	}
	if recs := col.Records(); recs[0].Outcome != llm.OutcomeValidationFailed || recs[0].StopReason != "max_tokens" || !strings.Contains(recs[0].Error, "max_tokens") {
		t.Errorf("record = %+v", recs[0])
	}
}

func TestSegmentMaxTokensThreeTimesExhausts(t *testing.T) {
	t.Parallel()
	cut := llmtest.JSON(llmtest.Message{StopReason: "max_tokens", Blocks: []llmtest.Block{llmtest.Text("The filter")}})
	srv := llmtest.NewMessagesServer(t, cut, cut, cut)
	_, err := newClient(t, srv).ProposeSegment(t.Context(), brief, &llmtest.Tools{})
	var ex *llm.RepairExhaustedError
	if !errors.As(err, &ex) || !strings.Contains(ex.Reason, "max_tokens") {
		t.Fatalf("err = %v", err)
	}
	// The cut-off text turn is passed back, followed by the nudge.
	msgs := messagesOf(t, srv.Requests()[1])
	if len(msgs) != 3 || field(t, msgs[1], "content", 0, "text") != "The filter" {
		t.Errorf("messages = %v", msgs)
	}
}

func TestSegmentMaxTokensWithOnlyAToolUseSendsNoAssistantTurn(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		llmtest.JSON(llmtest.Message{StopReason: "max_tokens", Blocks: []llmtest.Block{proposeBlock("t_cut", `city === "An`)}}),
		llmtest.JSON(msg(proposeBlock("t", `city === "Ankara"`))),
	)
	if _, err := newClient(t, srv).ProposeSegment(t.Context(), brief, &llmtest.Tools{}); err != nil {
		t.Fatal(err)
	}
	msgs := messagesOf(t, srv.Requests()[1])
	if len(msgs) != 2 || field(t, msgs[1], "role") != "user" {
		t.Errorf("messages = %v, want the original user message and the nudge", msgs)
	}
}

func TestSegmentToolInputProblems(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		block llmtest.Block
		count bool // does it count as a propose attempt
		want  string
	}{
		{"empty dsl", llmtest.ToolUse("t", "propose_segment", map[string]any{"dsl": "  "}), true, "non-empty"},
		{"missing dsl", llmtest.ToolUse("t", "propose_segment", map[string]any{"filter": "x"}), true, `{"dsl"`},
		{"dsl not a string", llmtest.ToolUse("t", "propose_segment", map[string]any{"dsl": 7}), true, `{"dsl"`},
		{"extra field", llmtest.ToolUse("t", "propose_segment", map[string]any{"dsl": "x", "extra": 1}), true, `{"dsl"`},
		{"count with bad input", llmtest.ToolUse("t", "count_audience", map[string]any{}), false, "non-empty"},
		{"unknown tool", llmtest.ToolUse("t", "delete_everything", map[string]any{"dsl": "x"}), false, "unknown tool"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := llmtest.NewMessagesServer(t,
				llmtest.JSON(msg(tt.block)),
				llmtest.JSON(msg(proposeBlock("t2", `city === "Ankara"`))),
			)
			tools := &llmtest.Tools{}
			res, err := newClient(t, srv).ProposeSegment(t.Context(), brief, tools)
			if err != nil {
				t.Fatal(err)
			}
			wantAttempts := 1
			if tt.count {
				wantAttempts = 2
			}
			if res.ProposeAttempts != wantAttempts {
				t.Errorf("attempts = %d, want %d", res.ProposeAttempts, wantAttempts)
			}
			_, text, isErr := lastToolResult(t, srv.Requests()[1])
			if !isErr || !strings.Contains(text, tt.want) {
				t.Errorf("is_error %v text %q, want it to contain %q", isErr, text, tt.want)
			}
			if got := len(tools.Validated()) + len(tools.Counted()); got != 1 {
				t.Errorf("the tools ran %d times (only the final valid propose should reach them)", got)
			}
		})
	}
}

func TestSegmentParallelToolCalls(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		llmtest.JSON(msg(countBlock("c1", `city === "Ankara"`), proposeBlock("p1", `nonsense ===`), countBlock("c2", `retired === true`))),
		llmtest.JSON(msg(proposeBlock("p2", `city === "Ankara"`))),
	)
	ctx, col := withRecorder(t)
	res, err := newClient(t, srv).ProposeSegment(ctx, brief, llm.NewCustomerTools(previewOf(3)))
	if err != nil {
		t.Fatal(err)
	}
	if res.ProposeAttempts != 2 {
		t.Errorf("attempts = %d", res.ProposeAttempts)
	}
	// One user message answers all three tool_use blocks, in order.
	msgs := messagesOf(t, srv.Requests()[1])
	results := field(t, msgs[2], "content").([]any)
	if len(results) != 3 {
		t.Fatalf("tool results = %d, want 3", len(results))
	}
	for i, id := range []string{"c1", "p1", "c2"} {
		if field(t, results[i], "tool_use_id") != id {
			t.Errorf("result %d answers %v, want %s", i, field(t, results[i], "tool_use_id"), id)
		}
	}
	if recs := col.Records(); recs[0].Outcome != llm.OutcomeValidationFailed {
		t.Errorf("the turn had a rejected propose, outcome = %s", recs[0].Outcome)
	}
}

func TestSegmentFirstValidProposeWinsInATurn(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, llmtest.JSON(msg(proposeBlock("p1", `nonsense ===`), proposeBlock("p2", `city === "Ankara"`))))
	res, err := newClient(t, srv).ProposeSegment(t.Context(), brief, llm.NewCustomerTools(previewOf(3)))
	if err != nil || res.DSL != `city === "Ankara"` || res.ProposeAttempts != 2 || res.Turn != 1 {
		t.Errorf("res = %+v, err = %v", res, err)
	}
}

func TestSegmentEmptyAudienceIsAValidationError(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		llmtest.JSON(msg(proposeBlock("t1", `loyaltyTier === "Gold" && totalOrders > 100`))),
		llmtest.JSON(msg(proposeBlock("t2", `loyaltyTier === "Gold"`))),
	)
	n := 0
	prev := previewerFunc(func(context.Context, *compile.Filter) (*customer.Preview, error) {
		n++
		if n == 1 {
			return &customer.Preview{Count: 0}, nil
		}
		return &customer.Preview{Count: 12}, nil
	})
	res, err := newClient(t, srv).ProposeSegment(t.Context(), brief, llm.NewCustomerTools(prev))
	if err != nil || res.Preview.Count != 12 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	_, text, isErr := lastToolResult(t, srv.Requests()[1])
	if !isErr || !strings.Contains(text, string(llm.CodeEmptyAudience)) {
		t.Errorf("is_error %v text %s", isErr, text)
	}
}

func TestSegmentCustomerFilterErrorIsRepaired(t *testing.T) {
	t.Parallel()
	// Customer rejects an unknown city with a dsl.Error at the literal and a suggestion (D-067).
	prev := previewerFunc(func(_ context.Context, f *compile.Filter) (*customer.Preview, error) {
		if strings.Contains(f.Source(), "Istnbul") {
			de := dsl.NewError(f.Source(), dsl.CodeUnknownValue, `unknown city "Istnbul"`, dsl.Pos{Offset: 9, Line: 1, Col: 10}, dsl.Pos{Offset: 18, Line: 1, Col: 19})
			de.Suggestions = []string{"İstanbul"}
			return nil, de
		}
		return &customer.Preview{Count: 46}, nil
	})
	srv := llmtest.NewMessagesServer(t,
		llmtest.JSON(msg(proposeBlock("t1", `city === "Istnbul"`))),
		llmtest.JSON(msg(proposeBlock("t2", `city === "İstanbul"`))),
	)
	res, err := newClient(t, srv).ProposeSegment(t.Context(), brief, llm.NewCustomerTools(prev))
	if err != nil || res.Preview.Count != 46 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	_, text, _ := lastToolResult(t, srv.Requests()[1])
	if !strings.Contains(text, "UNKNOWN_VALUE") || !strings.Contains(text, "İstanbul") || !strings.Contains(text, `"column":10`) {
		t.Errorf("tool_result = %s", text)
	}
}

func TestSegmentBackendFailureStopsTheLoop(t *testing.T) {
	t.Parallel()
	boom := errors.New("customer down")
	tests := []struct {
		name   string
		script llmtest.Response
		tools  llm.SegmentTools
	}{
		{"validate", llmtest.JSON(msg(proposeBlock("t", `city === "Ankara"`))), &llmtest.Tools{ValidateFunc: func(context.Context, string) (*compile.Filter, *customer.Preview, []*dsl.Error, error) {
			return nil, nil, nil, boom
		}}},
		{"count", llmtest.JSON(msg(countBlock("t", `city === "Ankara"`))), &llmtest.Tools{CountFunc: func(context.Context, string) (*customer.Preview, error) { return nil, boom }}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := llmtest.NewMessagesServer(t, tt.script)
			ctx, col := withRecorder(t)
			_, err := newClient(t, srv).ProposeSegment(ctx, brief, tt.tools)
			var te *llm.ToolsError
			if !errors.As(err, &te) || !errors.Is(err, boom) || !errors.Is(err, llm.ErrTools) {
				t.Fatalf("err = %v", err)
			}
			if n := len(srv.Requests()); n != 1 {
				t.Errorf("requests = %d; a backend failure is not for the model to repair", n)
			}
			recs := col.Records()
			if len(recs) != 1 || recs[0].Outcome != llm.OutcomeAPIError || !strings.Contains(recs[0].Error, "customer down") {
				t.Errorf("records = %+v", recs)
			}
		})
	}
}

func TestSegmentAPIErrorMidLoop(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		llmtest.JSON(msg(countBlock("t", `city === "Ankara"`))),
		llmtest.HTTPError(529, "overloaded_error", "busy"),
	)
	ctx, col := withRecorder(t)
	_, err := newClient(t, srv).ProposeSegment(ctx, brief, &llmtest.Tools{})
	var ae *llm.APIError
	if !errors.As(err, &ae) || ae.Status != 529 || ae.Purpose != llm.PurposeSegment {
		t.Fatalf("err = %v", err)
	}
	recs := col.Records()
	if len(recs) != 2 || recs[1].Outcome != llm.OutcomeAPIError || recs[1].Attempt != 2 || recs[1].Error == "" {
		t.Errorf("records = %+v", recs)
	}
}
