//go:build eval

// Prompt evaluation runner (D-115). It is a test file behind the eval build tag rather than a
// cmd/eval binary for one reason: the only scripted Messages API server in the repo,
// llmtest.MessagesServer, is built on testing.TB (it closes itself with t.Cleanup and fails the test
// when a case runs out of recorded turns), and the real adapter can only be driven without a key
// through that server. Reusing it unchanged keeps the whole production path under evaluation:
// anthropic.Client parses stop reasons and tool calls, llm.CustomerTools compiles the DSL
// (dsl.Parse, then catalog.Check), contentgen.Generator runs the validators of package content, and
// the repair messages that reach the "model" are the ones the server recorded. go test also gives
// subtests, -run filtering and a non-zero exit on any mismatch for free. The tag keeps it out of
// `go test ./...`, so make test does not run it (see the Makefile's eval target).
package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/proposal/compile"
	"github.com/furknozg/coldbru/internal/proposal/contentgen"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/flow"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/anthropic"
	"github.com/furknozg/coldbru/internal/proposal/llm/llmtest"
)

// casesDir holds one JSON file per case; the file name without .json is the case name.
const casesDir = "cases"

// Prompts a case may name: the prompt packages of internal/proposal/prompts that the pipeline
// validates in code. channel_plan and schedule are not evaluated here (their validators need the
// coverage of a real audience).
const (
	promptScreen  = "screen"
	promptSegment = "segment"
	promptContent = "content"
)

// Verdicts of the runner and of the expectation of a case.
const (
	verdictAccepted        = "accepted"         // the first recorded turn passed every check
	verdictRepairRequested = "repair_requested" // the first turn was rejected, the pipeline asked for a repair and the follow-up turn was accepted
	verdictRejected        = "rejected"         // the call ended in a terminal error (refusal, cut off, repair exhausted, ...)
	verdictFlagged         = "flagged"          // the pre-screen blocked the brief
)

// defaultAudience is the audience count the stub Customer reports for a filter that compiles.
const defaultAudience = 46

// testCase is one file of casesDir.
type testCase struct {
	name        string
	Description string      `json:"description"`
	Prompt      string      `json:"prompt"`
	Input       caseInput   `json:"input"`
	Turns       []turn      `json:"turns"`
	Expect      expectation `json:"expect"`
}

// caseInput is the call-specific data of the case: the user message of the prompt and, for the
// segment prompt, what the stub Customer answers.
type caseInput struct {
	Brief        string   `json:"brief"`
	ContextNotes []string `json:"context_notes"`
	// Segment: the audience count of every filter that compiles (defaultAudience when absent) and
	// per-filter overrides keyed by the exact DSL text. The count comes from a stub, not from
	// Customer/Postgres; see README.md.
	AudienceCount *int           `json:"audience_count"`
	AudienceByDSL map[string]int `json:"audience_by_dsl"`
	// Content: the slot.
	Language           string `json:"language"`
	Channel            string `json:"channel"`
	Recipients         int    `json:"recipients"`
	MaxFirstNameLength int    `json:"max_first_name_length"`
}

// turn is one recorded assistant message, in the shape the validators consume: the structured
// output as JSON (screen, content) or as raw text when it is cut off, and the tool calls of a
// segment turn with their inputs.
type turn struct {
	JSON       json.RawMessage `json:"json"`
	Text       string          `json:"text"`
	ToolCalls  []toolCall      `json:"tool_calls"`
	StopReason string          `json:"stop_reason"`
	Refusal    *refusal        `json:"refusal"`
}

type toolCall struct {
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type refusal struct {
	Category    string `json:"category"`
	Explanation string `json:"explanation"`
}

// expectation is the outcome the case expects. Error is the DSL error code (segment), the content
// rule (content) or the terminal error kind (rejected); Field, when given, is the content field.
type expectation struct {
	Verdict string `json:"verdict"`
	Error   string `json:"error"`
	Field   string `json:"field"`
}

func (e expectation) String() string {
	s := e.Verdict
	if e.Error != "" {
		s += "/" + e.Error
	}
	if e.Field != "" {
		s += "@" + e.Field
	}
	return s
}

// outcome is what the pipeline did with the case.
type outcome struct {
	expectation
	// Detail is free text for the report line.
	Detail string
}

// matches compares the outcome with the expectation. Field is compared only when the case names one.
func (o outcome) matches(e expectation) bool {
	return o.Verdict == e.Verdict && o.Error == e.Error && (e.Field == "" || o.Field == e.Field)
}

func TestEval(t *testing.T) {
	cases := loadCases(t)
	if len(cases) == 0 {
		t.Fatalf("no case in %s", casesDir)
	}
	passed, failed := 0, 0
	for _, c := range cases {
		ok := t.Run(c.name, func(t *testing.T) {
			got := run(t, c)
			status := "PASS"
			if !got.matches(c.Expect) {
				status = "FAIL"
				t.Errorf("%s: expected %s, got %s", c.name, c.Expect, got.expectation)
			}
			detail := ""
			if got.Detail != "" {
				detail = "  (" + got.Detail + ")"
			}
			fmt.Printf("%s  %-36s  %-8s  expected=%-34s  actual=%s%s\n", status, c.name, c.Prompt, c.Expect, got.expectation, detail)
		})
		if ok {
			passed++
		} else {
			failed++
		}
	}
	fmt.Printf("eval: %d cases, %d passed, %d failed\n", len(cases), passed, failed)
}

// loadCases reads every cases/*.json, strictly (an unknown key is a mistake in the case), and orders
// them by pipeline step (screen, segment, content), then by name.
func loadCases(t *testing.T) []testCase {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(casesDir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	order := map[string]int{promptScreen: 0, promptSegment: 1, promptContent: 2}
	var out []testCase
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var c testCase
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&c); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		c.name = strings.TrimSuffix(filepath.Base(f), ".json")
		if _, ok := order[c.Prompt]; !ok {
			t.Fatalf("%s: prompt %q is not one of screen, segment, content", f, c.Prompt)
		}
		if len(c.Turns) == 0 {
			t.Fatalf("%s: no turns", f)
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if order[out[i].Prompt] != order[out[j].Prompt] {
			return order[out[i].Prompt] < order[out[j].Prompt]
		}
		return out[i].name < out[j].name
	})
	return out
}

// run plays the case: the recorded turns are queued on a scripted Messages API server, the real
// adapter is pointed at it with a dummy key (the SDK reads no environment), and the production
// entry point of the prompt is called.
func run(t *testing.T, c testCase) outcome {
	t.Helper()
	responses := make([]llmtest.Response, len(c.Turns))
	for i, tn := range c.Turns {
		m, err := tn.message(i)
		if err != nil {
			t.Fatalf("turn %d: %v", i+1, err)
		}
		if c.Prompt == promptContent {
			responses[i] = llmtest.SSE(m) // StreamStructured streams
		} else {
			responses[i] = llmtest.JSON(m)
		}
	}
	srv := llmtest.NewMessagesServer(t, responses...)
	client, err := anthropic.New(anthropic.Config{APIKey: "eval-no-key", BaseURL: srv.URL}, option.WithMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}
	col := &llm.Collector{}
	ctx := llm.WithRecorder(t.Context(), col)
	switch c.Prompt {
	case promptScreen:
		return runScreen(ctx, client, c.Input)
	case promptSegment:
		return runSegment(ctx, client, srv, col, c.Input)
	default:
		return runContent(ctx, client, srv, c.Input)
	}
}

// message turns a recorded turn into the scripted server's message. n numbers the tool_use ids.
func (tn turn) message(n int) (llmtest.Message, error) {
	var m llmtest.Message
	switch {
	case len(tn.JSON) > 0 && tn.Text != "":
		return m, errors.New(`give "json" or "text", not both`)
	case len(tn.JSON) > 0:
		var buf bytes.Buffer
		if err := json.Compact(&buf, tn.JSON); err != nil {
			return m, fmt.Errorf("json: %w", err)
		}
		m.Blocks = append(m.Blocks, llmtest.Text(buf.String()))
	case tn.Text != "":
		m.Blocks = append(m.Blocks, llmtest.Text(tn.Text))
	}
	for i, tc := range tn.ToolCalls {
		var in any
		if err := json.Unmarshal(tc.Input, &in); err != nil {
			return m, fmt.Errorf("tool_calls[%d].input: %w", i, err)
		}
		m.Blocks = append(m.Blocks, llmtest.ToolUse(fmt.Sprintf("toolu_eval_%d_%d", n+1, i+1), tc.Name, in))
	}
	m.StopReason = tn.StopReason
	if tn.Refusal != nil {
		m.Refusal = &llmtest.Refusal{Category: tn.Refusal.Category, Explanation: tn.Refusal.Explanation}
	}
	return m, nil
}

// runScreen calls the pre-screen: allow is accepted, a block verdict is flagged, anything else is
// rejected (fail-closed, D-069).
func runScreen(ctx context.Context, client llm.LLM, in caseInput) outcome {
	res, err := client.Screen(ctx, in.Brief)
	var blocked *llm.ScreenBlockedError
	switch {
	case err == nil:
		return outcome{expectation: expectation{Verdict: verdictAccepted}, Detail: "title " + strconv(res.Title)}
	case errors.As(err, &blocked):
		return outcome{expectation: expectation{Verdict: verdictFlagged}, Detail: "reason " + strconv(blocked.Reason)}
	default:
		return outcome{expectation: expectation{Verdict: verdictRejected, Error: rejectedKind(err)}, Detail: err.Error()}
	}
}

// stubPreviewer stands in for the Customer client behind llm.CustomerTools: the compile step is the
// real one, the audience count is the case's. It is the only part of the segment path that is not
// production code (README.md).
type stubPreviewer struct {
	count int
	byDSL map[string]int
}

func (p stubPreviewer) Preview(_ context.Context, f *compile.Filter) (*customer.Preview, error) {
	n := p.count
	if m, ok := p.byDSL[f.Source()]; ok {
		n = m
	}
	both := n / 2
	email := (n - both) / 2
	sms := n - both - email
	return &customer.Preview{
		Count: n, Coverage: customer.Coverage{Both: both, EmailOnly: email, SmsOnly: sms}, MaxFirstNameLength: 9,
		ByLanguage: []customer.LanguageCoverage{{Language: "tr", Count: n, Coverage: customer.Coverage{Both: both, EmailOnly: email, SmsOnly: sms}, MaxFirstNameLength: 9}},
	}, nil
}

// runSegment runs the segment tool loop over llm.CustomerTools (compile + preview). The verdict on
// the first turn is read from what the loop sent back to the model: a repair is the is_error
// tool_result of the second request, whose JSON carries the DSL error codes (llm.FormatErrors); a
// nudge after a text-only or cut-off turn is classified from the first call record.
func runSegment(ctx context.Context, client llm.LLM, srv *llmtest.MessagesServer, col *llm.Collector, in caseInput) outcome {
	count := defaultAudience
	if in.AudienceCount != nil {
		count = *in.AudienceCount
	}
	tools := llm.NewCustomerTools(stubPreviewer{count: count, byDSL: in.AudienceByDSL})
	res, err := client.ProposeSegment(ctx, llm.SegmentInput{
		Brief: in.Brief, ContextNotes: in.ContextNotes, QuestionsLeft: llm.MaxQuestionsPerStep,
	}, tools)
	reqs := srv.Requests()
	repair := segmentRepair(reqs, col.Records())
	switch {
	case err != nil:
		return outcome{expectation: expectation{Verdict: verdictRejected, Error: rejectedKind(err)}, Detail: err.Error()}
	case res.Question != nil:
		return outcome{expectation: expectation{Verdict: verdictRejected, Error: "question"}, Detail: "asked: " + res.Question.Text}
	case repair != "":
		return outcome{expectation: expectation{Verdict: verdictRepairRequested, Error: repair},
			Detail: fmt.Sprintf("accepted after %d turns: %s, audience %d", res.Turns, res.DSL, res.Preview.Count)}
	default:
		return outcome{expectation: expectation{Verdict: verdictAccepted},
			Detail: fmt.Sprintf("%s, audience %d, summary %d chars", res.DSL, res.Preview.Count, len([]rune(res.Summary)))}
	}
}

// segmentRepair returns the kind of the first repair the loop asked for, "" when the first turn
// was accepted or the loop never got to a second request.
func segmentRepair(reqs []llmtest.Request, recs []llm.CallRecord) string {
	if len(reqs) < 2 {
		return ""
	}
	// The last message of the second request is what the model got back for its first turn.
	msgs, _ := reqs[1].JSON()["messages"].([]any)
	if len(msgs) == 0 {
		return ""
	}
	last, _ := msgs[len(msgs)-1].(map[string]any)
	blocks, _ := last["content"].([]any)
	for _, b := range blocks {
		blk, _ := b.(map[string]any)
		if blk["type"] != "tool_result" || blk["is_error"] != true {
			continue
		}
		var body struct {
			Errors []struct {
				Code string `json:"code"`
			} `json:"errors"`
		}
		if json.Unmarshal([]byte(toolResultText(blk)), &body) == nil && len(body.Errors) > 0 {
			return body.Errors[0].Code
		}
		return "tool_input" // an is_error result without the DSL error list: the tool input itself was bad
	}
	// No tool result: the loop nudged the model after a turn without a propose_segment.
	for _, r := range recs {
		if r.Outcome != llm.OutcomeValidationFailed {
			continue
		}
		if r.StopReason == "max_tokens" {
			return "max_tokens"
		}
		return "no_tool_call"
	}
	return ""
}

// toolResultText reads the text of a tool_result block, sent as a string or as text blocks.
func toolResultText(blk map[string]any) string {
	switch c := blk["content"].(type) {
	case string:
		return c
	case []any:
		var b strings.Builder
		for _, item := range c {
			if m, ok := item.(map[string]any); ok {
				s, _ := m["text"].(string)
				b.WriteString(s)
			}
		}
		return b.String()
	}
	return ""
}

// runContent runs contentgen.Generator for one slot with the SMS budget of the default
// TRACKING_BASE_URL (D-109) and the demo prices. The verdict on the first turn is read from the
// repair the generator sent in its second request: the user message carries repair.problems with
// the rule and the field of every violation (content.Problem).
func runContent(ctx context.Context, client llm.LLM, srv *llmtest.MessagesServer, in caseInput) outcome {
	prices, err := demoPrices()
	if err != nil {
		return outcome{expectation: expectation{Verdict: verdictRejected, Error: "setup"}, Detail: err.Error()}
	}
	base := content.DefaultTrackingBaseURL
	gen := contentgen.New(client, prices,
		contentgen.WithSMSBudget(content.SMSBudget(base)), contentgen.WithSMSLink(content.PlaceholderLink(base)))
	recipients := in.Recipients
	if recipients == 0 {
		recipients = defaultAudience
	}
	req := flow.ContentRequest{
		Language: in.Language, Channel: in.Channel, Brief: in.Brief, ContextNotes: in.ContextNotes,
		Audience: flow.AudienceContent{
			Count: recipients, MaxFirstNameLength: in.MaxFirstNameLength,
			ByLanguage: []flow.LanguageAudience{{Language: in.Language, Count: recipients, MaxFirstNameLength: in.MaxFirstNameLength}},
		},
		Plan:       flow.PlanContent{Mode: llm.ModeSingle, Primary: llm.Channel(in.Channel)},
		Recipients: recipients,
	}
	gen1, err := gen.Generate(ctx, req, nil)
	reqs := srv.Requests()
	rule, field := contentRepair(reqs)
	switch {
	case err != nil:
		return outcome{expectation: expectation{Verdict: verdictRejected, Error: rejectedKind(err)}, Detail: err.Error()}
	case rule != "":
		return outcome{expectation: expectation{Verdict: verdictRepairRequested, Error: rule, Field: field},
			Detail: fmt.Sprintf("accepted after %d attempts", len(reqs))}
	default:
		return outcome{expectation: expectation{Verdict: verdictAccepted}, Detail: storedSummary(gen1.Content)}
	}
}

// contentRepair returns the rule and the field of the first problem of the first repair request,
// "" when there was none.
func contentRepair(reqs []llmtest.Request) (rule, field string) {
	if len(reqs) < 2 {
		return "", ""
	}
	msgs, _ := reqs[1].JSON()["messages"].([]any)
	if len(msgs) == 0 {
		return "", ""
	}
	first, _ := msgs[0].(map[string]any)
	blocks, _ := first["content"].([]any)
	if len(blocks) == 0 {
		return "", ""
	}
	blk, _ := blocks[0].(map[string]any)
	text, _ := blk["text"].(string)
	var user struct {
		Repair *struct {
			Problems []content.Problem `json:"problems"`
		} `json:"repair"`
	}
	if json.Unmarshal([]byte(text), &user) != nil || user.Repair == nil || len(user.Repair.Problems) == 0 {
		return "", ""
	}
	p := user.Repair.Problems[0]
	return p.Rule, p.Field
}

// storedSummary describes the stored content in one phrase.
func storedSummary(raw json.RawMessage) string {
	var sms content.SMSContent
	if json.Unmarshal(raw, &sms) == nil && sms.Text != "" {
		return fmt.Sprintf("sms %d chars, %s, %d segment(s)", sms.RenderedLength, sms.Encoding, sms.Segments)
	}
	var email content.EmailContent
	if json.Unmarshal(raw, &email) == nil && email.Subject != "" {
		return fmt.Sprintf("email subject %d chars, %d paragraph(s)", email.MaxRenderedLengths.Subject, len(email.Paragraphs))
	}
	return "stored"
}

func demoPrices() (content.Prices, error) {
	sms, err := content.ParseMoney(content.DefaultSMSSegmentPrice)
	if err != nil {
		return content.Prices{}, err
	}
	email, err := content.ParseMoney(content.DefaultEmailPrice)
	if err != nil {
		return content.Prices{}, err
	}
	return content.Prices{SMSSegment: sms, Email: email}, nil
}

// rejectedKind names the terminal error of a call.
func rejectedKind(err error) string {
	var (
		re *llm.RefusalError
		ie *llm.InvalidOutputError
		rx *llm.RepairExhaustedError
		tl *llm.TurnLimitError
		te *llm.ToolsError
		ae *llm.APIError
	)
	switch {
	case errors.As(err, &re):
		return "refusal"
	case errors.As(err, &ie):
		if ie.StopReason == "max_tokens" {
			return "max_tokens"
		}
		return "invalid_output"
	case errors.As(err, &rx):
		return "repair_exhausted"
	case errors.As(err, &tl):
		return "turn_limit"
	case errors.As(err, &te):
		return "tools_error"
	case errors.As(err, &ae):
		return "api_error"
	}
	return "error"
}

// strconv quotes a short text for the report line.
func strconv(s string) string { return fmt.Sprintf("%q", s) }
