package anthropic_test

import (
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/llmtest"
	"github.com/furknozg/coldbru/internal/proposal/prompts"
)

// sampleCoverage is the coverage of the sample brief: 46 reachable, 3 unreachable (D-010).
func sampleCoverage() *customer.Preview {
	return &customer.Preview{
		Count:    46,
		Coverage: customer.Coverage{Both: 20, EmailOnly: 13, SmsOnly: 13, Unreachable: 3},
		ByLanguage: []customer.LanguageCoverage{
			{Language: "en", Count: 16, Coverage: customer.Coverage{Both: 7, EmailOnly: 5, SmsOnly: 4, Unreachable: 1}},
			{Language: "tr", Count: 30, Coverage: customer.Coverage{Both: 13, EmailOnly: 8, SmsOnly: 9, Unreachable: 2}},
		},
	}
}

// planSummary is the summary every scripted plan carries (D-088).
const planSummary = "**Plan**: e-posta birincil, SMS yedek."

// planReply is a channel_plan@2 answer with a plan and a summary and no question.
func planReply(mode, primary, rationale string) llmtest.Response {
	return planRaw(`{"question":null,"plan":{"mode":"` + mode + `","primary":"` + primary + `","rationale":"` + rationale + `"},"summary":"` + planSummary + `"}`)
}

// planRaw is a channel_plan answer with the given text.
func planRaw(text string) llmtest.Response {
	return llmtest.JSON(llmtest.Message{
		Model:  "claude-haiku-5-5",
		Blocks: []llmtest.Block{llmtest.Text(text)},
		Usage:  llmtest.Usage{Input: 400, Output: 30, CacheRead: 300, CacheCreation: 4},
	})
}

// planQuestion is a channel_plan@2 answer that asks the marketer.
func planQuestion(text string, options []string) llmtest.Response {
	q := map[string]any{"text": text, "options": options}
	b, _ := marshalJSON(map[string]any{"question": q, "plan": nil, "summary": nil})
	return planRaw(string(b))
}

var planInput = llm.ChannelPlanInput{Brief: brief.Brief, Coverage: sampleCoverage(), QuestionsLeft: 2}

func TestRequestBodyChannelPlan(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, planReply("primary_fallback", "email", "ok"))
	if _, err := newClient(t, srv).ProposeChannelPlan(t.Context(), planInput, nil); err != nil {
		t.Fatal(err)
	}
	body := requestShape(t, srv.Requests()[0], prompts.ChannelPlan(), false)
	if body["max_tokens"] != float64(512) {
		t.Errorf("max_tokens = %v", body["max_tokens"])
	}
	if !reflect.DeepEqual(body["thinking"], map[string]any{"type": "disabled"}) {
		t.Errorf("thinking = %v, want disabled (Haiku 5.5 accepts it at effort low)", body["thinking"])
	}
	if body["model"] != "claude-haiku-5-5" || field(t, body, "output_config", "effort") != "low" {
		t.Errorf("model/effort = %v/%v", body["model"], field(t, body, "output_config", "effort"))
	}
	if _, ok := body["stream"]; ok {
		t.Error("the plan call must not stream")
	}
	// The brief appears once, in the user message; the cached prefix holds no call data (D-060).
	if s := string(srv.Requests()[0].Body); countOccurrences(s, "hatırlatması gönder") != 1 {
		t.Errorf("the brief appears %d times, want once", countOccurrences(s, "hatırlatması gönder"))
	}
}

func TestProposeChannelPlanAccepted(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, planReply("primary_fallback", "email", "E-posta ve SMS 33 / 33: berabere, e-posta."))
	ctx, col := withRecorder(t)
	in := planInput
	in.ContextNotes = []string{"ton samimi olsun"}
	var seen []llm.ChannelPlan
	res, err := newClient(t, srv).ProposeChannelPlan(ctx, in, func(p llm.ChannelPlan) error {
		seen = append(seen, p)
		return nil
	})
	if err != nil {
		t.Fatalf("ProposeChannelPlan: %v", err)
	}
	want := llm.ChannelPlan{Mode: llm.ModePrimaryFallback, Primary: llm.ChannelEmail, Rationale: "E-posta ve SMS 33 / 33: berabere, e-posta."}
	if res.Plan != want || res.Attempts != 1 || len(seen) != 1 || seen[0] != want || res.Summary != planSummary || res.Question != nil {
		t.Errorf("result = %+v, validated %v", res, seen)
	}
	if res.Usage != (llm.Usage{InputTokens: 400, OutputTokens: 30, CacheReadTokens: 300, CacheCreationTokens: 4}) {
		t.Errorf("usage = %+v", res.Usage)
	}

	// The user message is one JSON object: brief, notes, the question budget and the coverage
	// table in snake_case.
	var user struct {
		Brief         string   `json:"brief"`
		ContextNotes  []string `json:"context_notes"`
		QuestionsLeft int      `json:"questions_left"`
		Coverage      struct {
			Count       int `json:"count"`
			Both        int `json:"both"`
			EmailOnly   int `json:"email_only"`
			SMSOnly     int `json:"sms_only"`
			Unreachable int `json:"unreachable"`
			ByLanguage  []struct {
				Language  string `json:"language"`
				Count     int    `json:"count"`
				Both      int    `json:"both"`
				EmailOnly int    `json:"email_only"`
				SMSOnly   int    `json:"sms_only"`
			} `json:"by_language"`
		} `json:"coverage"`
	}
	text, _ := field(t, srv.Requests()[0].JSON(), "messages", 0, "content", 0, "text").(string)
	decode(t, text, &user)
	if user.Brief != in.Brief || !reflect.DeepEqual(user.ContextNotes, in.ContextNotes) || user.QuestionsLeft != 2 {
		t.Errorf("user message = %+v", user)
	}
	c := user.Coverage
	if c.Count != 46 || c.Both != 20 || c.EmailOnly != 13 || c.SMSOnly != 13 || c.Unreachable != 3 ||
		len(c.ByLanguage) != 2 || c.ByLanguage[1].Language != "tr" || c.ByLanguage[1].Count != 30 || c.ByLanguage[1].EmailOnly != 8 || c.ByLanguage[1].SMSOnly != 9 {
		t.Errorf("coverage = %+v", c)
	}

	recs := col.Records()
	if len(recs) != 1 {
		t.Fatalf("records = %d, want 1", len(recs))
	}
	r := recs[0]
	if r.Purpose != llm.PurposeChannelPlan || r.Outcome != llm.OutcomeAccepted || r.Attempt != 1 || r.Model != "claude-haiku-5-5" ||
		r.StopReason != "end_turn" || r.Prompt.Name != "channel_plan" || r.Prompt.Version != "2.0.0" || len(r.Prompt.SHA256) != 64 || r.Error != "" ||
		!strings.Contains(string(r.Response), "primary_fallback") {
		t.Errorf("record = %+v", r)
	}
}

func TestProposeChannelPlanRepairsInTheSameConversation(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		planReply("single", "sms", "bad"),
		planReply("primary_fallback", "email", "good"),
	)
	ctx, col := withRecorder(t)
	validate := func(p llm.ChannelPlan) error {
		if p.Mode == llm.ModeSingle {
			return llm.NewProblemsError([]string{"single is only for an explicit request", "sms reaches 33 people, email 33"})
		}
		return nil
	}
	res, err := newClient(t, srv).ProposeChannelPlan(ctx, planInput, validate)
	if err != nil {
		t.Fatalf("ProposeChannelPlan: %v", err)
	}
	if res.Plan.Mode != llm.ModePrimaryFallback || res.Attempts != 2 {
		t.Errorf("result = %+v", res)
	}

	// Attempt 2 carries the whole conversation: the brief, the rejected answer, the problems.
	reqs := srv.Requests()
	if len(reqs) != 2 {
		t.Fatalf("requests = %d", len(reqs))
	}
	msgs := messagesOf(t, reqs[1])
	if len(msgs) != 3 || field(t, msgs[0], "role") != "user" || field(t, msgs[1], "role") != "assistant" || field(t, msgs[2], "role") != "user" {
		t.Fatalf("messages = %v, want user, assistant, user", msgs)
	}
	if asst, _ := field(t, msgs[1], "content", 0, "text").(string); !strings.Contains(asst, `"mode":"single"`) {
		t.Errorf("assistant turn = %q, want the rejected answer", asst)
	}
	fix, _ := field(t, msgs[2], "content", 0, "text").(string)
	for _, w := range []string{"rejected", "- single is only for an explicit request", "- sms reaches 33 people, email 33"} {
		if !strings.Contains(fix, w) {
			t.Errorf("repair message lacks %q:\n%s", w, fix)
		}
	}
	requestShape(t, reqs[1], prompts.ChannelPlan(), false)

	recs := col.Records()
	if len(recs) != 2 || recs[0].Attempt != 1 || recs[0].Outcome != llm.OutcomeValidationFailed || recs[1].Attempt != 2 || recs[1].Outcome != llm.OutcomeAccepted {
		t.Fatalf("records = %+v", recs)
	}
	if !strings.Contains(recs[0].Error, "single is only for an explicit request") || !strings.Contains(string(recs[0].Response), `"single"`) {
		t.Errorf("first record keeps the rejection: error %q response %s", recs[0].Error, recs[0].Response)
	}
}

func TestProposeChannelPlanRepairExhausted(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		planReply("both", "sms", "1"), planReply("both", "sms", "2"), planReply("both", "sms", "3"),
		planReply("both", "sms", "never asked"),
	)
	ctx, col := withRecorder(t)
	calls := 0
	_, err := newClient(t, srv).ProposeChannelPlan(ctx, planInput, func(llm.ChannelPlan) error {
		calls++
		return llm.NewProblemsError([]string{"both needs an explicit request", "attempt " + string(rune('0'+calls))})
	})
	var ex *llm.RepairExhaustedError
	if !errors.As(err, &ex) || !errors.Is(err, llm.ErrRepairExhausted) {
		t.Fatalf("err = %v, want *RepairExhaustedError", err)
	}
	// 1 + 2 repairs, no fourth call; the error carries the problems of the last attempt.
	if ex.Attempts != 3 || ex.Purpose != llm.PurposeChannelPlan || len(srv.Requests()) != 3 || calls != 3 {
		t.Errorf("attempts = %d, requests = %d, validate calls = %d", ex.Attempts, len(srv.Requests()), calls)
	}
	if want := []string{"both needs an explicit request", "attempt 3"}; !reflect.DeepEqual(ex.Problems, want) {
		t.Errorf("problems = %v, want %v", ex.Problems, want)
	}
	if !strings.Contains(err.Error(), "no valid channel_plan after 3 attempts") || !strings.Contains(err.Error(), "attempt 3") {
		t.Errorf("error text = %q", err)
	}
	recs := col.Records()
	if len(recs) != 3 {
		t.Fatalf("records = %d, want 3", len(recs))
	}
	for i, r := range recs {
		if r.Attempt != i+1 || r.Outcome != llm.OutcomeValidationFailed || r.Purpose != llm.PurposeChannelPlan {
			t.Errorf("record %d = %+v", i, r)
		}
	}
}

func TestProposeChannelPlanRefusalStops(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, llmtest.JSON(llmtest.Message{Refusal: &llmtest.Refusal{Category: "general_harms"}}))
	ctx, col := withRecorder(t)
	_, err := newClient(t, srv).ProposeChannelPlan(ctx, planInput, func(llm.ChannelPlan) error {
		t.Error("validate called for a refusal")
		return nil
	})
	var re *llm.RefusalError
	if !errors.As(err, &re) || re.Purpose != llm.PurposeChannelPlan || re.Category != "general_harms" {
		t.Fatalf("err = %v", err)
	}
	if len(srv.Requests()) != 1 {
		t.Errorf("requests = %d: a refusal must not be repaired", len(srv.Requests()))
	}
	recs := col.Records()
	if len(recs) != 1 || recs[0].Outcome != llm.OutcomeRefusal || recs[0].RefusalCategory != "general_harms" || recs[0].Purpose != llm.PurposeChannelPlan {
		t.Errorf("records = %+v", recs)
	}
}

func TestProposeChannelPlanAPIError(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, llmtest.HTTPError(http.StatusBadRequest, "invalid_request_error", "bad"))
	ctx, col := withRecorder(t)
	_, err := newClient(t, srv).ProposeChannelPlan(ctx, planInput, nil)
	var ae *llm.APIError
	if !errors.As(err, &ae) || ae.Status != 400 || ae.Purpose != llm.PurposeChannelPlan {
		t.Fatalf("err = %v", err)
	}
	if recs := col.Records(); len(recs) != 1 || recs[0].Outcome != llm.OutcomeAPIError || recs[0].Error == "" {
		t.Errorf("records = %+v", recs)
	}
}

// Answers the adapter cannot use are failed attempts that are repaired like a validation failure.
func TestProposeChannelPlanUnusableAnswersAreRepaired(t *testing.T) {
	t.Parallel()
	good := planReply("primary_fallback", "email", "ok")
	tests := []struct {
		name string
		bad  llmtest.Response
		want string
	}{
		{"not JSON", llmtest.JSON(msg(llmtest.Text("e-posta olsun"))), `not the object {"question","plan","summary"}`},
		{"old flat shape", planRaw(`{"mode":"single","primary":"sms","rationale":"x"}`), "unknown field"},
		{"unknown field", planRaw(`{"question":null,"plan":{"mode":"single","primary":"sms","rationale":"x","extra":1},"summary":"s"}`), "unknown field"},
		{"missing field", planRaw(`{"question":null,"plan":{"mode":"single","primary":"sms"},"summary":"s"}`), "all required"},
		{"unknown mode", planRaw(`{"question":null,"plan":{"mode":"all","primary":"sms","rationale":"x"},"summary":"s"}`), `unknown mode "all"`},
		{"unknown channel", planRaw(`{"question":null,"plan":{"mode":"both","primary":"push","rationale":"x"},"summary":"s"}`), `unknown primary "push"`},
		{"no plan, no question", planRaw(`{"question":null,"plan":null,"summary":"s"}`), `"plan" is required`},
		{"missing summary", planRaw(`{"question":null,"plan":{"mode":"single","primary":"sms","rationale":"x"},"summary":null}`), `"summary" is required`},
		{"summary with a link", planRaw(`{"question":null,"plan":{"mode":"single","primary":"sms","rationale":"x"},"summary":"see [here](https://x.y)"}`), "links"},
		{"summary with a heading", planRaw(`{"question":null,"plan":{"mode":"single","primary":"sms","rationale":"x"},"summary":"# Plan\nok"}`), "headings"},
		{"plan and question", planRaw(`{"question":{"text":"Which?","options":null},"plan":{"mode":"single","primary":"sms","rationale":"x"},"summary":null}`), "not both"},
		{"blank question", planRaw(`{"question":{"text":"  ","options":null},"plan":null,"summary":null}`), "must not be blank"},
		{"one option", planRaw(`{"question":{"text":"Which channel?","options":["sms"]},"plan":null,"summary":null}`), "options"},
		{"cut off", llmtest.JSON(llmtest.Message{Blocks: []llmtest.Block{llmtest.Text(`{"question":null,"plan":{"mode":"sin`)}, StopReason: "max_tokens"}), "cut off"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := llmtest.NewMessagesServer(t, tt.bad, good)
			ctx, col := withRecorder(t)
			res, err := newClient(t, srv).ProposeChannelPlan(ctx, planInput, func(llm.ChannelPlan) error {
				return nil
			})
			if err != nil || res.Plan.Mode != llm.ModePrimaryFallback || res.Attempts != 2 {
				t.Fatalf("res = %+v, err = %v", res, err)
			}
			fix, _ := field(t, messagesOf(t, srv.Requests()[1])[2], "content", 0, "text").(string)
			if !strings.Contains(fix, tt.want) {
				t.Errorf("repair message lacks %q:\n%s", tt.want, fix)
			}
			if recs := col.Records(); len(recs) != 2 || recs[0].Outcome != llm.OutcomeValidationFailed || recs[0].Error == "" {
				t.Errorf("records = %+v", recs)
			}
		})
	}
}

func TestProposeChannelPlanQuestionEndsTheCall(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, planQuestion("Hangi kanal?", []string{"SMS", "E-posta"}))
	ctx, col := withRecorder(t)
	res, err := newClient(t, srv).ProposeChannelPlan(ctx, planInput, func(llm.ChannelPlan) error {
		t.Error("validate called for a question")
		return nil
	})
	if err != nil {
		t.Fatalf("ProposeChannelPlan: %v", err)
	}
	if res.Question == nil || res.Question.Text != "Hangi kanal?" || !reflect.DeepEqual(res.Question.Options, []string{"SMS", "E-posta"}) ||
		res.Plan != (llm.ChannelPlan{}) || res.Summary != "" || res.Attempts != 1 {
		t.Errorf("res = %+v", res)
	}
	if len(srv.Requests()) != 1 {
		t.Errorf("requests = %d: a question ends the call", len(srv.Requests()))
	}
	if recs := col.Records(); len(recs) != 1 || recs[0].Outcome != llm.OutcomeAccepted || !strings.Contains(string(recs[0].Response), "Hangi kanal?") {
		t.Errorf("records = %+v", recs)
	}
}

// A question after the step used its questions is a validation failure fed back in the same
// conversation; it costs an attempt (D-069, D-088).
func TestProposeChannelPlanQuestionBeyondTheLimitIsRepaired(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, planQuestion("Hangi kanal?", nil), planReply("primary_fallback", "email", "ok"))
	in := planInput
	in.QuestionsLeft = 0
	in.Clarifications = []llm.Clarification{{Question: "Önceki soru", Answer: "Önceki cevap"}}
	ctx, col := withRecorder(t)
	res, err := newClient(t, srv).ProposeChannelPlan(ctx, in, nil)
	if err != nil || res.Question != nil || res.Plan.Mode != llm.ModePrimaryFallback || res.Attempts != 2 {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	reqs := srv.Requests()
	fix, _ := field(t, messagesOf(t, reqs[1])[2], "content", 0, "text").(string)
	if !strings.Contains(fix, llm.QuestionLimitProblem) {
		t.Errorf("repair message = %s", fix)
	}
	// The clarifications and the exhausted budget are in the user message.
	var user struct {
		Clarifications []struct{ Question, Answer string } `json:"clarifications"`
		QuestionsLeft  int                                 `json:"questions_left"`
	}
	text, _ := field(t, reqs[0].JSON(), "messages", 0, "content", 0, "text").(string)
	decode(t, text, &user)
	if user.QuestionsLeft != 0 || len(user.Clarifications) != 1 || user.Clarifications[0].Answer != "Önceki cevap" {
		t.Errorf("user message = %+v", user)
	}
	if recs := col.Records(); len(recs) != 2 || recs[0].Outcome != llm.OutcomeValidationFailed || !strings.Contains(recs[0].Error, "question limit") {
		t.Errorf("records = %+v", recs)
	}
}

func TestProposeChannelPlanNormalisesEnums(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, planReply(" Primary_Fallback ", " EMAIL", "  x "))
	res, err := newClient(t, srv).ProposeChannelPlan(t.Context(), planInput, nil)
	if err != nil || res.Plan != (llm.ChannelPlan{Mode: llm.ModePrimaryFallback, Primary: llm.ChannelEmail, Rationale: "x"}) {
		t.Errorf("res = %+v, err = %v", res, err)
	}
}

// ---- schedule ----

var scheduleNow = time.Date(2026, 10, 28, 22, 30, 0, 0, time.UTC)

func scheduleInput() llm.ScheduleInput {
	return llm.ScheduleInput{Brief: "Yarın sabah 10'da gönder.", ContextNotes: []string{"saat 10:00"}, Now: scheduleNow.In(time.FixedZone("x", 3*3600)), QuestionsLeft: 1,
		Locales: []llm.ScheduleLocale{
			{Language: "tr", Timezone: "Europe/Istanbul", Recipients: 36},
			{Language: "en", Timezone: "Europe/London", Recipients: 10},
		}}
}

// scheduleSummary is the summary every scripted schedule carries (D-088).
const scheduleSummary = "- tr: yarın 10:00\\n- en: hemen" // JSON-escaped: the summary holds a real newline

// scheduleReply is a schedule@3 answer with entries and a summary and no question.
func scheduleReply(entries string) llmtest.Response {
	return scheduleRaw(`{"question":null,"entries":` + entries + `,"summary":"` + scheduleSummary + `"}`)
}

func scheduleRaw(text string) llmtest.Response {
	return llmtest.JSON(llmtest.Message{
		Model:  "claude-haiku-5-5",
		Blocks: []llmtest.Block{llmtest.Text(text)},
		Usage:  llmtest.Usage{Input: 500, Output: 60},
	})
}

const goodEntries = `[{"language":"tr","timezone":"Europe/Istanbul","send_at":"2026-10-30T10:00:00+03:00","rationale":"Yarın 10:00 İstanbul."},{"language":"en","timezone":"Europe/London","send_at":null,"rationale":"No time for en."}]`

func TestRequestBodySchedule(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, scheduleReply(goodEntries))
	if _, err := newClient(t, srv).ProposeSchedule(t.Context(), scheduleInput(), nil); err != nil {
		t.Fatal(err)
	}
	body := requestShape(t, srv.Requests()[0], prompts.Schedule(), false)
	if body["max_tokens"] != float64(512) || body["model"] != "claude-haiku-5-5" || field(t, body, "output_config", "effort") != "low" {
		t.Errorf("max_tokens/model/effort = %v/%v/%v", body["max_tokens"], body["model"], field(t, body, "output_config", "effort"))
	}
	if !reflect.DeepEqual(body["thinking"], map[string]any{"type": "disabled"}) {
		t.Errorf("thinking = %v, want disabled", body["thinking"])
	}
	if _, ok := body["stream"]; ok {
		t.Error("the schedule call must not stream")
	}
}

func TestProposeScheduleAccepted(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, scheduleReply(goodEntries))
	ctx, col := withRecorder(t)
	res, err := newClient(t, srv).ProposeSchedule(ctx, scheduleInput(), func(e []llm.ScheduleEntry) error {
		if len(e) != 2 {
			t.Errorf("validate got %d entries", len(e))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("ProposeSchedule: %v", err)
	}
	if res.Attempts != 1 || len(res.Entries) != 2 || res.Summary != "- tr: yarın 10:00\n- en: hemen" || res.Question != nil {
		t.Fatalf("result = %+v", res)
	}
	tr, en := res.Entries[0], res.Entries[1]
	wantAt := time.Date(2026, 10, 30, 7, 0, 0, 0, time.UTC)
	if tr.Language != "tr" || tr.Timezone != "Europe/Istanbul" || tr.SendAt == nil || !tr.SendAt.Equal(wantAt) || tr.Rationale != "Yarın 10:00 İstanbul." {
		t.Errorf("tr entry = %+v", tr)
	}
	if _, off := tr.SendAt.Zone(); off != 3*3600 {
		t.Errorf("the offset the model wrote is lost: %d", off)
	}
	if en.Language != "en" || en.Timezone != "Europe/London" || en.SendAt != nil {
		t.Errorf("en entry = %+v, want send immediately", en)
	}

	// now in UTC, the question budget, and for each (language, timezone) pair the recipients and
	// the local time.
	var user struct {
		Brief         string   `json:"brief"`
		ContextNotes  []string `json:"context_notes"`
		QuestionsLeft int      `json:"questions_left"`
		Now           string   `json:"now"`
		Locales       []struct {
			Language   string `json:"language"`
			Timezone   string `json:"timezone"`
			Recipients int    `json:"recipients"`
			NowLocal   string `json:"now_local"`
		} `json:"locales"`
	}
	text, _ := field(t, srv.Requests()[0].JSON(), "messages", 0, "content", 0, "text").(string)
	decode(t, text, &user)
	if user.Brief != "Yarın sabah 10'da gönder." || !reflect.DeepEqual(user.ContextNotes, []string{"saat 10:00"}) || user.Now != "2026-10-28T22:30:00Z" ||
		len(user.Locales) != 2 || user.QuestionsLeft != 1 {
		t.Fatalf("user message = %+v", user)
	}
	if l := user.Locales[0]; l.Language != "tr" || l.Timezone != "Europe/Istanbul" || l.Recipients != 36 || l.NowLocal != "2026-10-29T01:30:00+03:00" {
		t.Errorf("tr = %+v", l)
	}
	if l := user.Locales[1]; l.Language != "en" || l.Timezone != "Europe/London" || l.Recipients != 10 || l.NowLocal != "2026-10-28T22:30:00Z" {
		t.Errorf("en = %+v", l)
	}

	recs := col.Records()
	if len(recs) != 1 || recs[0].Purpose != llm.PurposeSchedule || recs[0].Outcome != llm.OutcomeAccepted || recs[0].Prompt.Name != "schedule" {
		t.Errorf("records = %+v", recs)
	}
}

func TestProposeScheduleRepairsAndExhausts(t *testing.T) {
	t.Parallel()
	t.Run("repaired", func(t *testing.T) {
		t.Parallel()
		srv := llmtest.NewMessagesServer(t,
			scheduleReply(`[{"language":"tr","timezone":"Europe/Istanbul","send_at":"2026-10-28T22:31:00Z","rationale":"too soon"}]`),
			scheduleReply(goodEntries),
		)
		ctx, col := withRecorder(t)
		n := 0
		res, err := newClient(t, srv).ProposeSchedule(ctx, scheduleInput(), func(e []llm.ScheduleEntry) error {
			if n++; n == 1 {
				return llm.NewProblemsError([]string{"tr: send_at is earlier than now + 5 minutes", "en: missing"})
			}
			return nil
		})
		if err != nil || res.Attempts != 2 {
			t.Fatalf("res = %+v, err = %v", res, err)
		}
		fix, _ := field(t, messagesOf(t, srv.Requests()[1])[2], "content", 0, "text").(string)
		if !strings.Contains(fix, "- tr: send_at is earlier than now + 5 minutes") || !strings.Contains(fix, "- en: missing") {
			t.Errorf("repair message = %s", fix)
		}
		if recs := col.Records(); len(recs) != 2 || recs[0].Outcome != llm.OutcomeValidationFailed || recs[1].Outcome != llm.OutcomeAccepted {
			t.Errorf("records = %+v", recs)
		}
	})
	t.Run("exhausted", func(t *testing.T) {
		t.Parallel()
		srv := llmtest.NewMessagesServer(t, scheduleReply(goodEntries), scheduleReply(goodEntries), scheduleReply(goodEntries))
		ctx, col := withRecorder(t)
		_, err := newClient(t, srv).ProposeSchedule(ctx, scheduleInput(), func([]llm.ScheduleEntry) error {
			return errors.New("never good enough")
		})
		var ex *llm.RepairExhaustedError
		if !errors.As(err, &ex) || ex.Attempts != 3 || ex.Purpose != llm.PurposeSchedule || !reflect.DeepEqual(ex.Problems, []string{"never good enough"}) {
			t.Fatalf("err = %#v", err)
		}
		if len(srv.Requests()) != 3 || len(col.Records()) != 3 {
			t.Errorf("requests = %d, records = %d", len(srv.Requests()), len(col.Records()))
		}
	})
}

func TestProposeScheduleUnusableAnswersAreRepaired(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		bad  string
		want string
	}{
		{"no offset", `[{"language":"tr","timezone":"Europe/Istanbul","send_at":"2026-10-30T10:00:00","rationale":"x"}]`, "RFC 3339"},
		{"not a date", `[{"language":"tr","timezone":"Europe/Istanbul","send_at":"tomorrow","rationale":"x"}]`, `send_at "tomorrow"`},
		{"number", `[{"language":"tr","timezone":"Europe/Istanbul","send_at":5,"rationale":"x"}]`, "string or null"},
		{"missing send_at", `[{"language":"tr","timezone":"Europe/Istanbul","rationale":"x"}]`, `"send_at" is required`},
		{"empty language", `[{"language":" ","timezone":"Europe/Istanbul","send_at":null,"rationale":"x"}]`, `"language" is required`},
		{"missing rationale", `[{"language":"tr","timezone":"Europe/Istanbul","send_at":null}]`, `"rationale" is required`},
		{"missing timezone", `[{"language":"tr","send_at":null,"rationale":"x"}]`, `"timezone" is required`},
		{"empty timezone", `[{"language":"tr","timezone":" ","send_at":null,"rationale":"x"}]`, `"timezone" is required`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			srv := llmtest.NewMessagesServer(t, scheduleReply(tt.bad), scheduleReply(goodEntries))
			res, err := newClient(t, srv).ProposeSchedule(t.Context(), scheduleInput(), nil)
			if err != nil || res.Attempts != 2 {
				t.Fatalf("res = %+v, err = %v", res, err)
			}
			fix, _ := field(t, messagesOf(t, srv.Requests()[1])[2], "content", 0, "text").(string)
			if !strings.Contains(fix, tt.want) {
				t.Errorf("repair message lacks %q:\n%s", tt.want, fix)
			}
		})
	}
	t.Run("not an object", func(t *testing.T) {
		t.Parallel()
		srv := llmtest.NewMessagesServer(t, llmtest.JSON(msg(llmtest.Text(`[]`))), scheduleReply(goodEntries))
		if res, err := newClient(t, srv).ProposeSchedule(t.Context(), scheduleInput(), nil); err != nil || res.Attempts != 2 {
			t.Fatalf("res = %+v, err = %v", res, err)
		}
	})
	for name, tc := range map[string]struct{ raw, want string }{
		"old shape":       {`{"entries":` + goodEntries + `}`, `"summary" is required`},
		"missing summary": {`{"question":null,"entries":` + goodEntries + `,"summary":null}`, `"summary" is required`},
		"HTML summary":    {`{"question":null,"entries":` + goodEntries + `,"summary":"<p>x</p>"}`, "HTML"},
		"nothing":         {`{"question":null,"entries":null,"summary":null}`, `"entries" is required`},
		"both":            {`{"question":{"text":"Which Friday?","options":null},"entries":` + goodEntries + `,"summary":null}`, "not both"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv := llmtest.NewMessagesServer(t, scheduleRaw(tc.raw), scheduleReply(goodEntries))
			res, err := newClient(t, srv).ProposeSchedule(t.Context(), scheduleInput(), nil)
			if err != nil || res.Attempts != 2 {
				t.Fatalf("res = %+v, err = %v", res, err)
			}
			fix, _ := field(t, messagesOf(t, srv.Requests()[1])[2], "content", 0, "text").(string)
			if !strings.Contains(fix, tc.want) {
				t.Errorf("repair message lacks %q:\n%s", tc.want, fix)
			}
		})
	}
}

func TestProposeScheduleQuestion(t *testing.T) {
	t.Parallel()
	t.Run("asked", func(t *testing.T) {
		t.Parallel()
		srv := llmtest.NewMessagesServer(t, scheduleRaw(`{"question":{"text":"Hangi Cuma?","options":["Bu Cuma","Gelecek Cuma"]},"entries":null,"summary":null}`))
		res, err := newClient(t, srv).ProposeSchedule(t.Context(), scheduleInput(), func([]llm.ScheduleEntry) error {
			t.Error("validate called for a question")
			return nil
		})
		if err != nil || res.Question == nil || res.Question.Text != "Hangi Cuma?" || len(res.Question.Options) != 2 || len(res.Entries) != 0 || res.Attempts != 1 {
			t.Fatalf("res = %+v, err = %v", res, err)
		}
	})
	t.Run("beyond the limit", func(t *testing.T) {
		t.Parallel()
		srv := llmtest.NewMessagesServer(t, scheduleRaw(`{"question":{"text":"Hangi Cuma?","options":null},"entries":null,"summary":null}`), scheduleReply(goodEntries))
		in := scheduleInput()
		in.QuestionsLeft = 0
		res, err := newClient(t, srv).ProposeSchedule(t.Context(), in, nil)
		if err != nil || res.Question != nil || len(res.Entries) != 2 || res.Attempts != 2 {
			t.Fatalf("res = %+v, err = %v", res, err)
		}
		fix, _ := field(t, messagesOf(t, srv.Requests()[1])[2], "content", 0, "text").(string)
		if !strings.Contains(fix, llm.QuestionLimitProblem) {
			t.Errorf("repair message = %s", fix)
		}
	})
}

func TestProposeScheduleRefusalStops(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, llmtest.JSON(llmtest.Message{Refusal: &llmtest.Refusal{}}))
	ctx, col := withRecorder(t)
	_, err := newClient(t, srv).ProposeSchedule(ctx, scheduleInput(), nil)
	if !errors.Is(err, llm.ErrRefusal) || len(srv.Requests()) != 1 {
		t.Fatalf("err = %v, requests = %d", err, len(srv.Requests()))
	}
	if recs := col.Records(); len(recs) != 1 || recs[0].Outcome != llm.OutcomeRefusal || recs[0].Purpose != llm.PurposeSchedule {
		t.Errorf("records = %+v", recs)
	}
}
