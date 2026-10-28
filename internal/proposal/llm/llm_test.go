package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/furknozg/coldbru/internal/proposal/compile"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/dsl"
	"github.com/furknozg/coldbru/internal/proposal/llm"
)

func TestErrorsMatchTheirSentinels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err      error
		sentinel error
		text     string
	}{
		{&llm.ScreenBlockedError{Reason: "rule 2"}, llm.ErrScreenBlocked, "rule 2"},
		{&llm.RefusalError{Purpose: llm.PurposeSegment, Category: "cyber"}, llm.ErrRefusal, "segment call refused by the model (category cyber)"},
		{&llm.RefusalError{Purpose: llm.PurposeScreen}, llm.ErrRefusal, "screen call refused by the model"},
		{&llm.RepairExhaustedError{Attempts: 3, Reason: "cut off", Errors: []*dsl.Error{{Code: dsl.CodeSyntaxError, Message: "bad", Start: dsl.Pos{Line: 1, Col: 2}}}}, llm.ErrRepairExhausted, "SYNTAX_ERROR at 1:2: bad"},
		{&llm.TurnLimitError{Turns: 8}, llm.ErrTurnLimit, "all 8 model turns"},
		{&llm.APIError{Purpose: llm.PurposeContent, Status: 529, Type: "overloaded_error", Err: errors.New("boom")}, llm.ErrAPI, "HTTP 529 overloaded_error): boom"},
		{&llm.InvalidOutputError{Purpose: llm.PurposeContent, StopReason: "max_tokens", Reason: "cut"}, llm.ErrInvalidOutput, "max_tokens"},
		{&llm.ToolsError{Err: errors.New("down")}, llm.ErrTools, "down"},
		{&llm.ContentInvalidError{Err: errors.New("sms too long")}, llm.ErrContentInvalid, "sms too long"},
	}
	for _, tt := range tests {
		if !errors.Is(tt.err, tt.sentinel) {
			t.Errorf("%T is not %v", tt.err, tt.sentinel)
		}
		if !strings.Contains(tt.err.Error(), tt.text) {
			t.Errorf("%T = %q, want it to contain %q", tt.err, tt.err.Error(), tt.text)
		}
		for _, other := range []error{llm.ErrScreenBlocked, llm.ErrRefusal, llm.ErrRepairExhausted, llm.ErrTurnLimit, llm.ErrAPI, llm.ErrInvalidOutput, llm.ErrTools, llm.ErrContentInvalid} {
			if other != tt.sentinel && errors.Is(tt.err, other) {
				t.Errorf("%T also matches %v", tt.err, other)
			}
		}
	}
}

func TestAPIErrorRetryable(t *testing.T) {
	t.Parallel()
	for status, want := range map[int]bool{0: true, 400: false, 401: false, 408: true, 409: true, 429: true, 500: true, 529: true} {
		if got := (&llm.APIError{Status: status}).Retryable(); got != want {
			t.Errorf("status %d: retryable = %v, want %v", status, got, want)
		}
	}
	cause := context.Canceled
	if err := error(&llm.APIError{Err: cause}); !errors.Is(err, cause) {
		t.Error("APIError does not unwrap")
	}
}

func TestUsageAdd(t *testing.T) {
	t.Parallel()
	got := llm.Usage{1, 2, 3, 4}.Add(llm.Usage{10, 20, 30, 40})
	if got != (llm.Usage{11, 22, 33, 44}) {
		t.Errorf("Add = %+v", got)
	}
}

func TestReportWithoutARecorderIsANoop(t *testing.T) {
	t.Parallel()
	llm.Report(t.Context(), llm.CallRecord{}) // must not panic
	llm.Report(llm.WithRecorder(t.Context(), nil), llm.CallRecord{})
}

func TestCollectorAndRecorderFunc(t *testing.T) {
	t.Parallel()
	col := &llm.Collector{}
	var seen []int
	ctx := llm.WithRecorder(t.Context(), llm.RecorderFunc(func(ctx context.Context, rec llm.CallRecord) {
		seen = append(seen, rec.Attempt)
		col.Record(ctx, rec)
	}))
	llm.Report(ctx, llm.CallRecord{Attempt: 1})
	llm.Report(ctx, llm.CallRecord{Attempt: 2})
	got := col.Records()
	if len(got) != 2 || got[0].Attempt != 1 || got[1].Attempt != 2 || len(seen) != 2 {
		t.Errorf("records = %+v, seen = %v", got, seen)
	}
	got[0].Attempt = 99
	if col.Records()[0].Attempt != 1 {
		t.Error("Records returned the internal slice")
	}
}

func TestCollectorIsSafeForConcurrentUse(t *testing.T) {
	t.Parallel()
	col := &llm.Collector{}
	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			col.Record(t.Context(), llm.CallRecord{Attempt: i + 1})
		}()
	}
	wg.Wait()
	if n := len(col.Records()); n != 50 {
		t.Errorf("records = %d", n)
	}
}

func TestStoreParams(t *testing.T) {
	t.Parallel()
	camp, step := uuid.New(), uuid.New()
	rec := llm.CallRecord{
		Purpose: llm.PurposeSegment, Model: "claude-sonnet-5-5", Attempt: 2,
		Usage:      llm.Usage{InputTokens: 10, OutputTokens: 20, CacheReadTokens: 30, CacheCreationTokens: 40},
		StopReason: "tool_use", LatencyMs: 1234,
		Prompt:   llm.PromptRef{Name: "segment", Version: "1.0.0", SHA256: strings.Repeat("a", 64)},
		Outcome:  llm.OutcomeValidationFailed,
		Response: json.RawMessage(`[{"name":"propose_segment"}]`),
		Error:    "propose_segment was rejected",
	}
	p := rec.StoreParams(camp, &step, nil)
	if p.CampaignID != camp || p.StepID == nil || *p.StepID != step || p.VariantID != nil ||
		p.Purpose != "segment" || p.Model != "claude-sonnet-5-5" || p.Attempt != 2 ||
		p.InputTokens != 10 || p.OutputTokens != 20 || p.CacheReadTokens != 30 || p.CacheCreationTokens != 40 ||
		!p.StopReason.Valid || p.StopReason.String != "tool_use" || p.LatencyMs != 1234 ||
		p.PromptName != "segment" || p.PromptVersion != "1.0.0" || p.PromptSha256 != strings.Repeat("a", 64) ||
		p.Outcome != "validation_failed" || string(p.Response) != `[{"name":"propose_segment"}]` ||
		!p.Error.Valid || p.Error.String != "propose_segment was rejected" {
		t.Errorf("params = %+v", p)
	}

	// Empty optional fields become NULLs.
	p = llm.CallRecord{Outcome: llm.OutcomeAccepted}.StoreParams(camp, nil, nil)
	if p.StopReason.Valid || p.Error.Valid || p.Response != nil || p.StepID != nil {
		t.Errorf("empty record = %+v", p)
	}

	// The refusal category has no column; it lands in the error text.
	p = llm.CallRecord{Outcome: llm.OutcomeRefusal, RefusalCategory: "bio"}.StoreParams(camp, nil, nil)
	if !p.Error.Valid || !strings.Contains(p.Error.String, "bio") {
		t.Errorf("refusal error = %+v", p.Error)
	}
}

func previewer(n int, err error) llm.Previewer {
	return stubPreviewer(func(context.Context, *compile.Filter) (*customer.Preview, error) {
		if err != nil {
			return nil, err
		}
		return &customer.Preview{Count: n, Coverage: customer.Coverage{Unreachable: 4}}, nil
	})
}

type stubPreviewer func(context.Context, *compile.Filter) (*customer.Preview, error)

func (f stubPreviewer) Preview(ctx context.Context, c *compile.Filter) (*customer.Preview, error) {
	return f(ctx, c)
}

func TestCustomerToolsValidate(t *testing.T) {
	t.Parallel()
	const good = `city === "Ankara"`
	boom := errors.New("customer: unauthorized")
	unknownCity := dsl.NewError(good, dsl.CodeUnknownValue, "unknown city", dsl.Pos{Offset: 9, Line: 1, Col: 10}, dsl.Pos{Offset: 17, Line: 1, Col: 18})
	tests := []struct {
		name     string
		src      string
		prev     llm.Previewer
		wantOK   bool
		wantCode dsl.Code
		wantErr  error
	}{
		{"valid", good, previewer(5, nil), true, "", nil},
		{"syntax error", `city ===`, previewer(5, nil), false, dsl.CodeSyntaxError, nil},
		{"unknown field", `citty === "x"`, previewer(5, nil), false, dsl.CodeUnknownField, nil},
		{"empty audience", good, previewer(0, nil), false, llm.CodeEmptyAudience, nil},
		{"customer repairable error", good, previewer(0, unknownCity), false, dsl.CodeUnknownValue, nil},
		{"customer repairable error, wrapped", good, previewer(0, errors.Join(errors.New("x"), unknownCity)), false, dsl.CodeUnknownValue, nil},
		{"backend failure", good, previewer(0, boom), false, "", boom},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			f, p, errs, err := llm.NewCustomerTools(tt.prev).Validate(t.Context(), tt.src)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			switch {
			case tt.wantOK:
				if f == nil || p == nil || len(errs) != 0 {
					t.Errorf("f = %v, p = %v, errs = %v", f, p, errs)
				}
			case tt.wantCode != "":
				if f != nil || p != nil || len(errs) == 0 || errs[0].Code != tt.wantCode {
					t.Errorf("f = %v, p = %v, errs = %v; want code %s", f, p, errs, tt.wantCode)
				}
			}
		})
	}
}

func TestEmptyAudienceErrorSpansTheSource(t *testing.T) {
	t.Parallel()
	src := "retired === true &&\ncity === \"İzmir\""
	none := stubPreviewer(func(context.Context, *compile.Filter) (*customer.Preview, error) { return &customer.Preview{}, nil })
	_, _, errs, err := llm.NewCustomerTools(none).Validate(t.Context(), src)
	if err != nil || len(errs) != 1 {
		t.Fatalf("errs = %v, err = %v", errs, err)
	}
	e := errs[0]
	if e.Code != llm.CodeEmptyAudience || e.Start.Line != 1 || e.Start.Col != 1 || e.End.Line != 2 || e.End.Col != 17 || e.Near == "" {
		t.Errorf("error = %+v", e)
	}
	if !strings.Contains(e.Message, "no reachable customer") {
		t.Errorf("message = %q", e.Message)
	}
	// With customers that match but cannot be reached the message says so (D-058).
	prev := stubPreviewer(func(context.Context, *compile.Filter) (*customer.Preview, error) {
		return &customer.Preview{Count: 0, Coverage: customer.Coverage{Unreachable: 9}}, nil
	})
	_, _, errs, _ = llm.NewCustomerTools(prev).Validate(t.Context(), `city === "Ankara"`)
	if len(errs) != 1 || !strings.Contains(errs[0].Message, "matches 9 customers but none can be reached") {
		t.Errorf("errs = %v", errs)
	}
}

func TestCustomerToolsCountAudience(t *testing.T) {
	t.Parallel()
	tools := llm.NewCustomerTools(previewer(0, nil))
	// A count of zero is information, not an error.
	p, err := tools.CountAudience(t.Context(), `city === "Ankara"`)
	if err != nil || p == nil || p.Count != 0 {
		t.Errorf("p = %+v, err = %v", p, err)
	}
	// A filter that does not compile is a repairable error that customer.DSLErrors can read.
	_, err = tools.CountAudience(t.Context(), `citty === 1`)
	var ve *llm.ValidationError
	if !errors.As(err, &ve) || len(llm.RepairableErrors(err)) != len(ve.Errors) || len(ve.Errors) == 0 {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "UNKNOWN_FIELD") {
		t.Errorf("error text = %q", err.Error())
	}
	// A backend failure is not repairable.
	_, err = llm.NewCustomerTools(previewer(0, errors.New("down"))).CountAudience(t.Context(), `city === "Ankara"`)
	if err == nil || len(llm.RepairableErrors(err)) != 0 {
		t.Errorf("err = %v", err)
	}
}

func TestFormatErrors(t *testing.T) {
	t.Parallel()
	e := dsl.NewError(`city === "Istnbul"`, dsl.CodeUnknownValue, `unknown city "Istnbul"`, dsl.Pos{Offset: 9, Line: 1, Col: 10}, dsl.Pos{Offset: 18, Line: 1, Col: 19})
	e.Suggestions = []string{"İstanbul"}
	bare := &dsl.Error{Code: dsl.CodeSyntaxError, Message: "m", Start: dsl.Pos{Line: 3, Col: 4}}
	out := llm.FormatErrors([]*dsl.Error{e, bare})
	var got struct {
		Error  string `json:"error"`
		Next   string `json:"next"`
		Errors []struct {
			Code        string   `json:"code"`
			Line        int      `json:"line"`
			Column      int      `json:"column"`
			Message     string   `json:"message"`
			Near        *string  `json:"near"`
			Suggestions []string `json:"suggestions"`
		} `json:"errors"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	if got.Error == "" || got.Next == "" || len(got.Errors) != 2 {
		t.Fatalf("got = %+v", got)
	}
	a, b := got.Errors[0], got.Errors[1]
	if a.Code != "UNKNOWN_VALUE" || a.Line != 1 || a.Column != 10 || a.Near == nil || *a.Near != `"Istnbul"` || len(a.Suggestions) != 1 || a.Suggestions[0] != "İstanbul" {
		t.Errorf("first = %+v", a)
	}
	if b.Code != "SYNTAX_ERROR" || b.Line != 3 || b.Column != 4 || b.Near != nil || b.Suggestions != nil {
		t.Errorf("second = %+v (empty near and suggestions are omitted)", b)
	}
	if strings.Contains(out, `\u0`) || strings.Contains(out, "\n") {
		t.Errorf("output is escaped or multi-line: %s", out)
	}
}

func TestFormatPreview(t *testing.T) {
	t.Parallel()
	out := llm.FormatPreview(&customer.Preview{Count: 46, Coverage: customer.Coverage{Both: 20, EmailOnly: 13, SmsOnly: 13, Unreachable: 3}})
	if want := `{"count":46,"coverage":{"both":20,"emailOnly":13,"smsOnly":13,"unreachable":3},"byLanguage":null}`; out != want {
		t.Errorf("FormatPreview = %s", out)
	}
}

func TestProblems(t *testing.T) {
	t.Parallel()
	if llm.NewProblemsError(nil) != nil || llm.Problems(nil) != nil {
		t.Error("no problems must be a nil error")
	}
	err := llm.NewProblemsError([]string{"a", "b"})
	if err.Error() != "a; b" {
		t.Errorf("error text = %q", err)
	}
	wrapped := fmt.Errorf("check: %w", err)
	if got := llm.Problems(wrapped); len(got) != 2 || got[1] != "b" {
		t.Errorf("Problems(wrapped) = %v", got)
	}
	if got := llm.Problems(errors.New("plain")); len(got) != 1 || got[0] != "plain" {
		t.Errorf("Problems(plain) = %v", got)
	}
}

func TestPlanTypes(t *testing.T) {
	t.Parallel()
	for _, m := range []llm.PlanMode{llm.ModeSingle, llm.ModePrimaryFallback, llm.ModeBoth} {
		if !m.Valid() {
			t.Errorf("%s is not valid", m)
		}
	}
	if llm.PlanMode("all").Valid() || llm.PlanMode("").Valid() || llm.Channel("push").Valid() {
		t.Error("an unknown mode or channel is valid")
	}
	if llm.ChannelEmail.Other() != llm.ChannelSMS || llm.ChannelSMS.Other() != llm.ChannelEmail {
		t.Error("Other()")
	}
	if llm.TimezoneOf("tr") != "Europe/Istanbul" || llm.TimezoneOf("en") != "Europe/London" || llm.TimezoneOf("de") != "UTC" {
		t.Error("TimezoneOf")
	}
}

func TestRepairExhaustedErrorText(t *testing.T) {
	t.Parallel()
	seg := (&llm.RepairExhaustedError{Attempts: 3}).Error()
	plan := (&llm.RepairExhaustedError{Purpose: llm.PurposeChannelPlan, Attempts: 3, Problems: []string{"x", "y"}}).Error()
	if !strings.Contains(seg, "no valid segment after 3") || !strings.Contains(plan, "no valid channel_plan after 3 attempts; x; y") {
		t.Errorf("texts = %q, %q", seg, plan)
	}
}
