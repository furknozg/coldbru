package contentgen_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/proposal/contentgen"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/llmtest"
	"github.com/furknozg/coldbru/internal/proposal/prompts"
)

// userOf decodes the user message of a recorded call.
func userOf(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	decode(t, raw, &m)
	return m
}

func TestGenerateSMSStoresMeasurements(t *testing.T) {
	t.Parallel()
	// "Sepetin" has no Turkish-only letter, "alışveriş" does: UCS-2.
	text := "Merhaba {{first_name}}, sepetindeki ürünlerde %30 indirim seni bekliyor, alışverişe dön!"
	s := llmtest.NewScripted().StreamJSON(smsDoc(text))
	raw, err := newGenerator(s).Generate(t.Context(), request("tr", "sms", 33), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got content.SMSContent
	decode(t, raw.Content, &got)
	rendered := []rune(strings.ReplaceAll(text, "{{first_name}}", rep("x", 9)))
	want := content.SMSContent{
		Text: text, Encoding: content.UCS2, Segments: 2, RenderedLength: len(rendered), EstimatedCostUSD: "2.6400", // 2 x 33 x 0.04
	}
	if len(rendered) <= 70 {
		want.Segments, want.EstimatedCostUSD = 1, "1.3200"
	}
	if got != want {
		t.Errorf("content = %+v, want %+v", got, want)
	}
	// Only the stored fields are present.
	var keys map[string]json.RawMessage
	decode(t, raw.Content, &keys)
	if len(keys) != 5 {
		t.Errorf("keys = %v", keys)
	}
}

func TestGenerateEmailStoresBlocksAndCost(t *testing.T) {
	t.Parallel()
	s := llmtest.NewScripted().StreamJSON(emailDoc("Sepetin seni bekliyor, {{first_name}}"))
	raw, err := newGenerator(s).Generate(t.Context(), request("tr", "email", 33), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got content.EmailContent
	decode(t, raw.Content, &got)
	if got.Subject != "Sepetin seni bekliyor, {{first_name}}" || len(got.Paragraphs) != 2 || got.CTALabel != "Sepete dön" {
		t.Errorf("blocks = %+v", got)
	}
	if got.EstimatedCostUSD != "0.0330" || got.MaxRenderedLengths.Subject != 23+9 || got.MaxRenderedLengths.Headline != 8+9+1 {
		t.Errorf("measurements = %+v / %s", got.MaxRenderedLengths, got.EstimatedCostUSD)
	}
	// The % of "%30" and the umlauts are stored as written, not escaped.
	if !strings.Contains(string(raw.Content), `"%30 indirim fırsatını kaçırma."`) {
		t.Errorf("content = %s", raw.Content)
	}
}

func TestGenerateMeasuresWithTheLanguagesLongestName(t *testing.T) {
	t.Parallel()
	// 152 characters plus {{first_name}}: 159 with the English audience's longest name (7), 161 with
	// the Turkish one's (9), which is also the overall maximum used for a language without an entry.
	text := smsDoc(rep("a", 152) + "{{first_name}}")
	en := llmtest.NewScripted().StreamJSON(text)
	if _, err := newGenerator(en).Generate(t.Context(), request("en", "sms", 1), nil); err != nil {
		t.Errorf("en: %v", err)
	}
	for _, lang := range []string{"tr", "de"} {
		s := llmtest.NewScripted().StreamJSON(text).StreamJSON(text).StreamJSON(text)
		_, err := newGenerator(s).Generate(t.Context(), request(lang, "sms", 1), nil)
		var ex *llm.RepairExhaustedError
		if !errors.As(err, &ex) || len(ex.Problems) != 1 || !strings.Contains(ex.Problems[0], "161 characters") {
			t.Errorf("%s: err = %v, want 161 characters over the limit", lang, err)
		}
	}
}

func TestGenerateUserMessage(t *testing.T) {
	t.Parallel()
	s := llmtest.NewScripted().StreamJSON(smsDoc("Merhaba {{first_name}}"))
	req := request("tr", "sms", 33)
	req.RejectionReason = "too pushy"
	if _, err := newGenerator(s).Generate(t.Context(), req, nil); err != nil {
		t.Fatal(err)
	}
	calls := s.StreamCalls()
	if len(calls) != 1 || calls[0].Package != prompts.Content() {
		t.Fatalf("calls = %d, package = %v: the content package must be used", len(calls), calls[0].Package)
	}
	m := userOf(t, calls[0].UserJSON)
	checks := map[string]any{
		"language": "tr", "channel": "sms", "brief": sampleBrief, "recipients": float64(33),
		"max_first_name_length": float64(9), "rejection_reason": "too pushy",
	}
	for k, want := range checks {
		if m[k] != want {
			t.Errorf("%s = %v, want %v", k, m[k], want)
		}
	}
	if _, ok := m["repair"]; ok {
		t.Error("the first attempt carries a repair object")
	}
	if notes, _ := m["context_notes"].([]any); len(notes) != 1 || notes[0] != "Mention the weekend." {
		t.Errorf("context_notes = %v", m["context_notes"])
	}
	if aud, _ := m["audience"].(map[string]any); aud["count"] != float64(46) || aud["language_count"] != float64(30) {
		t.Errorf("audience = %v", m["audience"])
	}
	if plan, _ := m["plan"].(map[string]any); plan["mode"] != "primary_fallback" || plan["primary"] != "email" || len(plan) != 2 {
		t.Errorf("plan = %v", m["plan"])
	}
	var limits content.Limits
	b, _ := json.Marshal(m["limits"])
	decode(t, b, &limits)
	if limits != content.DefaultLimits() || limits.SMSMaxChars != 160 || limits.SubjectMax != 80 {
		t.Errorf("limits = %+v", limits)
	}
	// No customer data and no filter reach the model (research 03 §7).
	if strings.Contains(string(calls[0].UserJSON), "dsl") || strings.Contains(string(calls[0].UserJSON), "İstanbul\"") {
		t.Errorf("the audience filter leaked into the user message: %s", calls[0].UserJSON)
	}
	// Turkish text is not escaped.
	if !strings.Contains(string(calls[0].UserJSON), "İstanbul'daki") || strings.Contains(string(calls[0].UserJSON), `\u`) {
		t.Errorf("user JSON is escaped: %s", calls[0].UserJSON)
	}
}

func TestGenerateRepairsAnOverLongAnswer(t *testing.T) {
	t.Parallel()
	bad := smsDoc(rep("a", 170))
	good := smsDoc("Merhaba {{first_name}}, sepetin seni bekliyor!")
	s := llmtest.NewScripted().StreamJSON(bad).StreamJSON(good)
	raw, err := newGenerator(s).Generate(t.Context(), request("en", "sms", 10), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got content.SMSContent
	decode(t, raw.Content, &got)
	if got.Text != "Merhaba {{first_name}}, sepetin seni bekliyor!" || got.Encoding != content.GSM7 || got.Segments != 1 || got.EstimatedCostUSD != "0.4000" {
		t.Errorf("content = %+v", got)
	}
	calls := s.StreamCalls()
	if len(calls) != 2 {
		t.Fatalf("%d model calls, want 2", len(calls))
	}
	if string(calls[0].UserJSON) == string(calls[1].UserJSON) {
		t.Fatal("the repair call is the same as the first call")
	}
	m := userOf(t, calls[1].UserJSON)
	rp, _ := m["repair"].(map[string]any)
	if rp == nil || rp["attempt"] != float64(2) {
		t.Fatalf("repair = %v", m["repair"])
	}
	// The previous output, as the model wrote it.
	var prev map[string]any
	b, _ := json.Marshal(rp["previous_output"])
	decode(t, b, &prev)
	if sms, _ := prev["sms"].(map[string]any); sms["text"] != rep("a", 170) {
		t.Errorf("previous_output = %v", rp["previous_output"])
	}
	// The structured problem list.
	problems, _ := rp["problems"].([]any)
	if len(problems) != 1 {
		t.Fatalf("problems = %v", problems)
	}
	p, _ := problems[0].(map[string]any)
	if p["field"] != "sms.text" || p["rule"] != "max_length" || p["got"] != float64(170) || p["limit"] != float64(160) {
		t.Errorf("problem = %v", p)
	}
	if msg, _ := p["message"].(string); !strings.Contains(msg, "170") || !strings.Contains(msg, "160") {
		t.Errorf("message = %q", msg)
	}
	// Everything else of the user message is unchanged (same brief, limits, name length).
	first := userOf(t, calls[0].UserJSON)
	delete(m, "repair")
	if !jsonEqual(t, first, m) {
		t.Errorf("the repair changed more than the repair object:\n%v\n%v", first, m)
	}
}

func jsonEqual(t *testing.T, a, b any) bool {
	t.Helper()
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

func TestGenerateRepairExhausted(t *testing.T) {
	t.Parallel()
	s := llmtest.NewScripted().
		StreamJSON(smsDoc(rep("a", 170))).
		StreamJSON(smsDoc("see https://x.example")).
		StreamJSON(smsDoc("Hi {{city}}"))
	var deltas []string
	_, err := newGenerator(s).Generate(t.Context(), request("en", "sms", 10), func(_ int, d string) {
		if d != "" {
			deltas = append(deltas, d)
		}
	})
	var ex *llm.RepairExhaustedError
	if !errors.As(err, &ex) || !errors.Is(err, llm.ErrRepairExhausted) {
		t.Fatalf("err = %v, want *RepairExhaustedError", err)
	}
	if ex.Purpose != llm.PurposeContent || ex.Attempts != 3 || len(ex.Problems) != 1 || !strings.Contains(ex.Problems[0], "{{city}}") {
		t.Errorf("exhausted = %+v: want the problems of the last attempt", ex)
	}
	calls := s.StreamCalls()
	if len(calls) != 3 {
		t.Fatalf("%d model calls, want 3 (one plus two repairs)", len(calls))
	}
	// The third call carries the second answer and its problem, not the first.
	rp, _ := userOf(t, calls[2].UserJSON)["repair"].(map[string]any)
	prev, _ := json.Marshal(rp["previous_output"])
	if rp["attempt"] != float64(3) || !strings.Contains(string(prev), "https://x.example") {
		t.Errorf("third call repair = %v", rp)
	}
	problems, _ := rp["problems"].([]any)
	if p, _ := problems[0].(map[string]any); len(problems) != 1 || p["rule"] != "url" {
		t.Errorf("problems of the second answer = %v", problems)
	}
	if len(deltas) != 3 {
		t.Errorf("deltas = %d, want one per attempt from the scripted stream", len(deltas))
	}
}

func TestGenerateWrongChannelIsRepaired(t *testing.T) {
	t.Parallel()
	// An email was requested and the model filled the sms slot (and left email null).
	s := llmtest.NewScripted().StreamJSON(smsDoc("Hi {{first_name}}")).StreamJSON(emailDoc("Selam {{first_name}}"))
	raw, err := newGenerator(s).Generate(t.Context(), request("en", "email", 5), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got content.EmailContent
	decode(t, raw.Content, &got)
	if got.EstimatedCostUSD != "0.0050" {
		t.Errorf("content = %+v", got)
	}
	rp, _ := userOf(t, s.StreamCalls()[1].UserJSON)["repair"].(map[string]any)
	problems, _ := rp["problems"].([]any)
	if len(problems) != 2 {
		t.Fatalf("problems = %v", problems)
	}
	var fields []string
	for _, p := range problems {
		pm := p.(map[string]any)
		if pm["rule"] != "wrong_channel" {
			t.Errorf("problem = %v", pm)
		}
		fields = append(fields, pm["field"].(string))
	}
	if strings.Join(fields, ",") != "email,sms" {
		t.Errorf("fields = %v, want email (empty) and sms (filled)", fields)
	}
}

func TestGenerateDocumentShapeProblems(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		doc   string
		rule  string
		field string
	}{
		{"both channels filled", `{"sms":{"text":"a"},"email":{"subject":"s","preheader":"p","headline":"h","paragraphs":["x"],"cta_label":"c"}}`, "wrong_channel", "email"},
		{"neither filled", `{"sms":null,"email":null}`, "wrong_channel", "sms"},
		{"email key missing", `{"sms":{"text":"a"}}`, "required", "email"},
		{"unknown top-level key", `{"sms":{"text":"a"},"email":null,"note":"x"}`, "unknown_field", "note"},
		{"sms without text", `{"sms":{},"email":null}`, "required", "sms.text"},
		{"sms with an extra field", `{"sms":{"text":"a","emoji":"x"},"email":null}`, "invalid_json", "sms"},
		{"sms with computed fields", `{"sms":{"text":"a","segments":1},"email":null}`, "unknown_field", "sms"},
		{"not an object", `[1]`, "invalid_json", "$"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Three identical failures: the problems of the last attempt come back.
			s := llmtest.NewScripted().StreamJSON(tt.doc).StreamJSON(tt.doc).StreamJSON(tt.doc)
			_, err := newGenerator(s).Generate(t.Context(), request("en", "sms", 1), nil)
			var ex *llm.RepairExhaustedError
			if !errors.As(err, &ex) || len(ex.Problems) == 0 {
				t.Fatalf("err = %v", err)
			}
			rp, _ := userOf(t, s.StreamCalls()[2].UserJSON)["repair"].(map[string]any)
			var found bool
			for _, p := range rp["problems"].([]any) {
				pm := p.(map[string]any)
				found = found || (pm["rule"] == tt.rule && pm["field"] == tt.field)
			}
			if !found {
				t.Errorf("problems = %v, want rule %s on %s", rp["problems"], tt.rule, tt.field)
			}
		})
	}
}

func TestGenerateEmailProblemsAreAllListed(t *testing.T) {
	t.Parallel()
	doc := `{"sms":null,"email":{"subject":"` + rep("s", 90) + `","preheader":"p","headline":"<b>h</b>","paragraphs":["a","b","c","d","e"],"cta_label":"go {{city}}"}}`
	s := llmtest.NewScripted().StreamJSON(doc).StreamJSON(emailDoc("ok"))
	if _, err := newGenerator(s).Generate(t.Context(), request("tr", "email", 3), nil); err != nil {
		t.Fatal(err)
	}
	rp, _ := userOf(t, s.StreamCalls()[1].UserJSON)["repair"].(map[string]any)
	got := map[string]bool{}
	for _, p := range rp["problems"].([]any) {
		pm := p.(map[string]any)
		got[pm["field"].(string)+":"+pm["rule"].(string)] = true
	}
	for _, want := range []string{"email.subject:max_length", "email.headline:html", "email.paragraphs:max_items", "email.cta_label:placeholder"} {
		if !got[want] {
			t.Errorf("missing problem %s in %v", want, got)
		}
	}
}

func TestGenerateStopsOnErrorsThatAreNotContentProblems(t *testing.T) {
	t.Parallel()
	for name, err := range map[string]error{
		"refusal":   &llm.RefusalError{Purpose: llm.PurposeContent, Category: "general_harms"},
		"api error": &llm.APIError{Purpose: llm.PurposeContent, Status: 529, Type: "overloaded_error"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			s := llmtest.NewScripted().FailStream(err).StreamJSON(smsDoc("never used"))
			_, got := newGenerator(s).Generate(t.Context(), request("en", "sms", 1), nil)
			if !errors.Is(got, err) {
				t.Fatalf("err = %v, want %v", got, err)
			}
			if n := len(s.StreamCalls()); n != 1 {
				t.Errorf("%d model calls, want 1: no repair for this failure", n)
			}
		})
	}
}

func TestGenerateRejectsAnUnusableRequest(t *testing.T) {
	t.Parallel()
	s := llmtest.NewScripted()
	for name, mutate := range map[string]func(*string, *string){
		"no language":     func(l, _ *string) { *l = "" },
		"unknown channel": func(_, c *string) { *c = "push" },
	} {
		req := request("tr", "sms", 1)
		mutate(&req.Language, &req.Channel)
		if _, err := newGenerator(s).Generate(t.Context(), req, nil); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	if n := len(s.StreamCalls()); n != 0 {
		t.Errorf("%d model calls for unusable requests", n)
	}
}

func TestGenerateForwardsDeltasOfEveryAttempt(t *testing.T) {
	t.Parallel()
	bad, good := smsDoc(rep("a", 170)), smsDoc("Hello {{first_name}}")
	s := llmtest.NewScripted().StreamJSON(bad[:10], bad[10:]).StreamJSON(good[:7], good[7:])
	type delta struct {
		attempt int
		text    string
	}
	var got []delta
	if _, err := newGenerator(s).Generate(t.Context(), request("en", "sms", 1), func(a int, d string) { got = append(got, delta{a, d}) }); err != nil {
		t.Fatal(err)
	}
	// Every attempt is announced by an empty fragment, then its text follows (D-072).
	want := []delta{{1, ""}, {1, bad[:10]}, {1, bad[10:]}, {2, ""}, {2, good[:7]}, {2, good[7:]}}
	if len(got) != len(want) {
		t.Fatalf("deltas = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("delta %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// ---- against the real adapter and the Messages API server ----

func TestGenerateRecordsOneCallPerAttempt(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t,
		sse(smsDoc(rep("a", 170))),
		sse(smsDoc("see www.example.com")),
		sse(smsDoc("Hello {{first_name}}!")),
	)
	col := &llm.Collector{}
	ctx := llm.WithRecorder(context.Background(), col)
	raw, err := serverGenerator(t, srv).Generate(ctx, request("en", "sms", 10), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got content.SMSContent
	decode(t, raw.Content, &got)
	if got.Text != "Hello {{first_name}}!" {
		t.Errorf("content = %+v", got)
	}
	recs := col.Records()
	if len(recs) != 3 {
		t.Fatalf("%d records, want 3", len(recs))
	}
	wantOutcome := []llm.Outcome{llm.OutcomeValidationFailed, llm.OutcomeValidationFailed, llm.OutcomeAccepted}
	for i, r := range recs {
		pkg := prompts.Content()
		if r.Attempt != i+1 || r.Outcome != wantOutcome[i] || r.Purpose != llm.PurposeContent {
			t.Errorf("record %d = attempt %d, %s, %s", i, r.Attempt, r.Purpose, r.Outcome)
		}
		if r.Prompt != (llm.PromptRef{Name: "content", Version: pkg.Version, SHA256: pkg.SHA256}) || r.Model != "claude-sonnet-5-5" {
			t.Errorf("record %d prompt = %+v model %s", i, r.Prompt, r.Model)
		}
		if r.Usage.InputTokens != 900 || r.Usage.OutputTokens != 120 || r.StopReason != "end_turn" || len(r.Response) == 0 {
			t.Errorf("record %d usage/stop/response = %+v %s %s", i, r.Usage, r.StopReason, r.Response)
		}
	}
	if !strings.Contains(recs[0].Error, "170") || !strings.Contains(recs[1].Error, "web address") || recs[2].Error != "" {
		t.Errorf("record errors = %q / %q / %q", recs[0].Error, recs[1].Error, recs[2].Error)
	}
	// The second request is a new, stateless call: one user message, not a longer conversation.
	for i, r := range srv.Requests() {
		msgs, _ := r.JSON()["messages"].([]any)
		if len(msgs) != 1 {
			t.Errorf("request %d has %d messages, want 1", i, len(msgs))
		}
	}
}

func TestGenerateRequestUsesTheContentPackage(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, sse(emailDoc("Selam")))
	if _, err := serverGenerator(t, srv).Generate(t.Context(), request("en", "email", 1), nil); err != nil {
		t.Fatal(err)
	}
	r := srv.Requests()[0]
	pkg := prompts.Content()
	var body struct {
		Model        string           `json:"model"`
		MaxTokens    int              `json:"max_tokens"`
		Stream       bool             `json:"stream"`
		Thinking     map[string]any   `json:"thinking"`
		System       []map[string]any `json:"system"`
		Tools        []any            `json:"tools"`
		OutputConfig struct {
			Effort string `json:"effort"`
			Format struct {
				Type   string          `json:"type"`
				Schema json.RawMessage `json:"schema"`
			} `json:"format"`
		} `json:"output_config"`
		Messages []struct {
			Role    string `json:"role"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	decode(t, r.Body, &body)
	if body.Model != "claude-sonnet-5-5" || body.MaxTokens != 2048 || !body.Stream || body.Thinking["type"] != "between_tools" || body.OutputConfig.Effort != "low" {
		t.Errorf("parameters = %s %d stream %v thinking %v effort %s (D-069)", body.Model, body.MaxTokens, body.Stream, body.Thinking, body.OutputConfig.Effort)
	}
	// D-061: the system text and the schema are the bytes of the package.
	if len(body.System) != 1 || body.System[0]["text"] != pkg.System {
		t.Errorf("system text differs from the content package")
	}
	canon, err := prompts.Canonical(body.OutputConfig.Format.Schema)
	if err != nil || body.OutputConfig.Format.Type != "json_schema" || string(canon) != string(pkg.OutputSchema) {
		t.Errorf("output schema differs from the content package:\n%s\nwant\n%s", canon, pkg.OutputSchema)
	}
	if len(body.Tools) != 0 {
		t.Errorf("tools = %v", body.Tools)
	}
	// The call-specific data is the one user message and nowhere else.
	if len(body.Messages) != 1 || body.Messages[0].Role != "user" {
		t.Fatalf("messages = %+v", body.Messages)
	}
	if n := strings.Count(string(r.Body), "hatırlatması gönder"); n != 1 {
		t.Errorf("the brief appears %d times in the request, want 1", n)
	}
	if strings.Contains(pkg.System, "hatırlatması") {
		t.Error("the system text contains the brief")
	}
	var user map[string]any
	decode(t, []byte(body.Messages[0].Content[0].Text), &user)
	if user["language"] != "en" || user["channel"] != "email" || user["max_first_name_length"] != float64(7) {
		t.Errorf("user message = %v", user)
	}
}

func TestGenerateRefusalStopsAndIsRecorded(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, llmtest.SSE(llmtest.Message{Refusal: &llmtest.Refusal{Category: "general_harms"}}))
	col := &llm.Collector{}
	_, err := serverGenerator(t, srv).Generate(llm.WithRecorder(t.Context(), col), request("tr", "sms", 1), nil)
	var re *llm.RefusalError
	if !errors.As(err, &re) || re.Category != "general_harms" {
		t.Fatalf("err = %v, want *RefusalError", err)
	}
	if n := len(srv.Requests()); n != 1 {
		t.Errorf("%d requests, want 1: no repair after a refusal", n)
	}
	recs := col.Records()
	if len(recs) != 1 || recs[0].Outcome != llm.OutcomeRefusal || recs[0].Attempt != 1 || recs[0].RefusalCategory != "general_harms" {
		t.Errorf("records = %+v", recs)
	}
}

func TestGenerateRepairAfterARefusalFreeFailureKeepsNumbering(t *testing.T) {
	t.Parallel()
	// A valid second answer after an invalid first: the accepted record is the last and has attempt 2.
	srv := llmtest.NewMessagesServer(t, sse(smsDoc("{{city}}")), sse(smsDoc("Hi {{first_name}}")))
	col := &llm.Collector{}
	if _, err := serverGenerator(t, srv).Generate(llm.WithRecorder(t.Context(), col), request("en", "sms", 1), nil); err != nil {
		t.Fatal(err)
	}
	recs := col.Records()
	if len(recs) != 2 || recs[1].Outcome != llm.OutcomeAccepted || recs[1].Attempt != 2 || recs[0].Attempt != 1 {
		t.Errorf("records = %+v", recs)
	}
}

func TestGenerateAPIFailureIsRecordedOnceAndNotRepaired(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, llmtest.HTTPError(500, "api_error", "boom"))
	col := &llm.Collector{}
	_, err := serverGenerator(t, srv).Generate(llm.WithRecorder(t.Context(), col), request("tr", "sms", 1), nil)
	if !errors.Is(err, llm.ErrAPI) {
		t.Fatalf("err = %v", err)
	}
	if recs := col.Records(); len(recs) != 1 || recs[0].Outcome != llm.OutcomeAPIError {
		t.Errorf("records = %+v", recs)
	}
}

func TestGenerateWithoutARecorderIsFine(t *testing.T) {
	t.Parallel()
	srv := llmtest.NewMessagesServer(t, sse(smsDoc("Hi {{first_name}}")))
	if _, err := serverGenerator(t, srv).Generate(context.Background(), request("en", "sms", 1), nil); err != nil {
		t.Fatal(err)
	}
}

func cutOffErr() error {
	return &llm.InvalidOutputError{Purpose: llm.PurposeContent, StopReason: "max_tokens", Reason: "stopped with max_tokens"}
}

func TestGenerateRepairsACutOffAnswer(t *testing.T) {
	t.Parallel()
	s := llmtest.NewScripted().FailStream(cutOffErr(), `{"sms":{"text":"Merhaba`).StreamJSON(smsDoc("Hello {{first_name}}"))
	raw, err := newGenerator(s).Generate(t.Context(), request("en", "sms", 10), nil)
	if err != nil {
		t.Fatal(err)
	}
	var got content.SMSContent
	decode(t, raw.Content, &got)
	if got.Text != "Hello {{first_name}}" {
		t.Errorf("content = %+v", got)
	}
	calls := s.StreamCalls()
	if len(calls) != 2 {
		t.Fatalf("%d model calls, want 2", len(calls))
	}
	rp, _ := userOf(t, calls[1].UserJSON)["repair"].(map[string]any)
	if rp["attempt"] != float64(2) {
		t.Fatalf("repair = %v", rp)
	}
	if _, ok := rp["previous_output"]; ok {
		t.Errorf("previous_output = %v: the scripted failure has no record to take it from", rp["previous_output"])
	}
	problems, _ := rp["problems"].([]any)
	p, _ := problems[0].(map[string]any)
	if len(problems) != 1 || p["rule"] != "invalid_output" || p["field"] != "" ||
		p["message"] != "the previous answer was cut off or was not valid JSON; answer again with the complete, shorter JSON object" {
		t.Errorf("problems = %v", problems)
	}
}

func TestGenerateThreeCutOffsExhaustTheRepairs(t *testing.T) {
	t.Parallel()
	s := llmtest.NewScripted().FailStream(cutOffErr()).FailStream(cutOffErr()).FailStream(cutOffErr()).StreamJSON(smsDoc("never used"))
	_, err := newGenerator(s).Generate(t.Context(), request("en", "sms", 10), nil)
	var ex *llm.RepairExhaustedError
	if !errors.As(err, &ex) || ex.Purpose != llm.PurposeContent || ex.Attempts != 3 || len(ex.Problems) != 1 || !strings.Contains(ex.Problems[0], "cut off") {
		t.Fatalf("err = %v (%+v)", err, ex)
	}
	if n := len(s.StreamCalls()); n != 3 {
		t.Errorf("%d model calls, want 3", n)
	}
}

func TestGenerateMixedFailuresShareTheThreeAttempts(t *testing.T) {
	t.Parallel()
	s := llmtest.NewScripted().FailStream(cutOffErr()).StreamJSON(smsDoc(rep("a", 170))).StreamJSON(smsDoc("Hi {{first_name}}"))
	if _, err := newGenerator(s).Generate(t.Context(), request("en", "sms", 1), nil); err != nil {
		t.Fatal(err)
	}
	rp, _ := userOf(t, s.StreamCalls()[2].UserJSON)["repair"].(map[string]any)
	problems, _ := rp["problems"].([]any)
	if p, _ := problems[0].(map[string]any); rp["attempt"] != float64(3) || len(problems) != 1 || p["rule"] != "max_length" {
		t.Errorf("third call repair = %v", rp)
	}
}

func TestGenerateCutOffAndNonJSONRecordsPerAttempt(t *testing.T) {
	t.Parallel()
	cut := llmtest.SSE(llmtest.Message{Blocks: []llmtest.Block{llmtest.Text(`{"sms":{"text":"Merhaba {{first_n`)}, StopReason: "max_tokens", Usage: llmtest.Usage{Input: 900, Output: 2048}})
	srv := llmtest.NewMessagesServer(t, cut, sse("Sure! Here is your SMS."), sse(smsDoc("Hello {{first_name}}")))
	col := &llm.Collector{}
	if _, err := serverGenerator(t, srv).Generate(llm.WithRecorder(t.Context(), col), request("en", "sms", 1), nil); err != nil {
		t.Fatal(err)
	}
	recs := col.Records()
	if len(recs) != 3 {
		t.Fatalf("%d records, want 3", len(recs))
	}
	wantOutcome := []llm.Outcome{llm.OutcomeValidationFailed, llm.OutcomeValidationFailed, llm.OutcomeAccepted}
	for i, r := range recs {
		if r.Attempt != i+1 || r.Outcome != wantOutcome[i] {
			t.Errorf("record %d = attempt %d, %s", i, r.Attempt, r.Outcome)
		}
	}
	if recs[0].StopReason != "max_tokens" || recs[0].Usage.OutputTokens != 2048 {
		t.Errorf("first record = %+v", recs[0])
	}
	// The next call carries what was received, as a JSON string, and the invalid_output problem.
	reqs := srv.Requests()
	for i, wantPrev := range []string{`{"sms":{"text":"Merhaba {{first_n`, `Sure! Here is your SMS.`} {
		var body struct {
			Messages []struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"messages"`
		}
		decode(t, reqs[i+1].Body, &body)
		var user struct {
			Repair struct {
				Attempt  int              `json:"attempt"`
				Previous string           `json:"previous_output"`
				Problems []map[string]any `json:"problems"`
			} `json:"repair"`
		}
		decode(t, []byte(body.Messages[0].Content[0].Text), &user)
		r := user.Repair
		if r.Attempt != i+2 || r.Previous != wantPrev || len(r.Problems) != 1 || r.Problems[0]["rule"] != "invalid_output" {
			t.Errorf("request %d repair = %+v, want attempt %d with previous %q", i+1, r, i+2, wantPrev)
		}
	}
}

// content@2 (D-088): the summary of the answer is returned next to the content and never stored
// in it; a summary that breaks the rules, or a missing one, is dropped without a repair; an
// unknown key is still a problem.
func TestGenerateReturnsTheSummary(t *testing.T) {
	t.Parallel()
	withSummary := func(summary any) string {
		b, _ := json.Marshal(map[string]any{"sms": map[string]any{"text": "Merhaba {{first_name}}, %30 indirim seni bekliyor."}, "email": nil, "summary": summary})
		return string(b)
	}
	tests := []struct {
		name string
		doc  string
		want string
	}{
		{"summary", withSummary("  **Samimi** ton, %30 indirim. "), "**Samimi** ton, %30 indirim."},
		{"missing summary", smsDoc("Merhaba {{first_name}}, %30 indirim seni bekliyor."), ""},
		{"null summary", withSummary(nil), ""},
		{"summary with a link", withSummary("see https://x.y"), ""},
		{"summary with a heading", withSummary("# Ton"), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := llmtest.NewScripted().StreamJSON(tt.doc)
			gen, err := newGenerator(s).Generate(t.Context(), request("tr", "sms", 33), nil)
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if gen.Summary != tt.want {
				t.Errorf("summary = %q, want %q", gen.Summary, tt.want)
			}
			if strings.Contains(string(gen.Content), "summary") {
				t.Errorf("the stored content carries the summary: %s", gen.Content)
			}
			if n := len(s.StreamCalls()); n != 1 {
				t.Errorf("%d calls: a summary problem is never repaired", n)
			}
		})
	}
	// The request carries no clarifications key: the content step asks no questions.
	s := llmtest.NewScripted().StreamJSON(withSummary("ok"))
	if _, err := newGenerator(s).Generate(t.Context(), request("tr", "sms", 33), nil); err != nil {
		t.Fatal(err)
	}
	if u := userOf(t, s.StreamCalls()[0].UserJSON); u["clarifications"] != nil || u["questions_left"] != nil {
		t.Errorf("user message = %v", u)
	}
}

// With the tracking link, the model is told a smaller SMS limit and the validator enforces it
// (D-109); the email limits do not change.
func TestGenerateUsesTheSMSBudget(t *testing.T) {
	t.Parallel()
	budget := content.SMSBudget(content.DefaultTrackingBaseURL) // 125
	generator := func(l llm.LLM) *contentgen.Generator {
		return contentgen.New(l, demoPrices, contentgen.WithSMSBudget(budget))
	}

	t.Run("the limit in the user message", func(t *testing.T) {
		t.Parallel()
		s := llmtest.NewScripted().StreamJSON(smsDoc("Merhaba {{first_name}}!"))
		if _, err := generator(s).Generate(t.Context(), request("tr", "sms", 10), nil); err != nil {
			t.Fatal(err)
		}
		var limits content.Limits
		b, _ := json.Marshal(userOf(t, s.StreamCalls()[0].UserJSON)["limits"])
		decode(t, b, &limits)
		if limits != content.NewLimits(budget) || limits.SMSMaxChars != 125 || limits.SubjectMax != 80 {
			t.Errorf("limits = %+v, want an SMS limit of 125 and the email limits unchanged", limits)
		}
	})

	t.Run("a text over the budget is repaired, one at the budget is accepted", func(t *testing.T) {
		t.Parallel()
		// 126 characters fit the old 160 limit but not the budget; the name (max 9) counts too.
		over := smsDoc(rep("a", 126))
		atBudget := smsDoc("{{first_name}} " + rep("a", budget-10))
		s := llmtest.NewScripted().StreamJSON(over).StreamJSON(atBudget)
		got, err := generator(s).Generate(t.Context(), request("tr", "sms", 10), nil)
		if err != nil {
			t.Fatal(err)
		}
		var c content.SMSContent
		decode(t, got.Content, &c)
		if c.RenderedLength != budget {
			t.Errorf("rendered_length = %d, want %d", c.RenderedLength, budget)
		}
		calls := s.StreamCalls()
		if len(calls) != 2 {
			t.Fatalf("%d model calls, want 2: one repair", len(calls))
		}
		rp, _ := userOf(t, calls[1].UserJSON)["repair"].(map[string]any)
		problems, _ := rp["problems"].([]any)
		if len(problems) != 1 {
			t.Fatalf("repair problems = %v", rp["problems"])
		}
		p, _ := problems[0].(map[string]any)
		if p["rule"] != content.RuleMaxLength || p["limit"] != float64(budget) || p["got"] != float64(126) {
			t.Errorf("problem = %v, want max_length 126 over a limit of %d", p, budget)
		}
	})

	t.Run("an edit is held to the budget as well", func(t *testing.T) {
		t.Parallel()
		g := generator(llmtest.NewScripted())
		over := json.RawMessage(`{"text":"` + rep("a", 130) + `"}`)
		if _, err := g.Validate(request("tr", "sms", 10), over); err == nil {
			t.Error("Validate accepted a 130 character text under a budget of 125")
		}
		fits := json.RawMessage(`{"text":"` + rep("a", 125) + `"}`)
		if _, err := g.Validate(request("tr", "sms", 10), fits); err != nil {
			t.Errorf("Validate rejected a 125 character text: %v", err)
		}
	})

	t.Run("without the option nothing is reserved", func(t *testing.T) {
		t.Parallel()
		g := newGenerator(llmtest.NewScripted())
		if _, err := g.Validate(request("tr", "sms", 10), json.RawMessage(`{"text":"`+rep("a", 160)+`"}`)); err != nil {
			t.Errorf("a 160 character text without a budget option: %v", err)
		}
	})
}

// The stored segments and cost are those of the SMS with its tracking link (D-109): a UCS-2 text
// of 40 characters is one segment alone and two with the 34 character link and its space.
func TestGenerateMeasuresTheSMSWithTheTrackingLink(t *testing.T) {
	t.Parallel()
	base := content.DefaultTrackingBaseURL
	text := "Şimdi sepete dön, " + rep("b", 22)
	build := func(opts ...contentgen.Option) content.SMSContent {
		t.Helper()
		s := llmtest.NewScripted().StreamJSON(smsDoc(text))
		opts = append([]contentgen.Option{contentgen.WithSMSBudget(content.SMSBudget(base))}, opts...)
		got, err := contentgen.New(s, demoPrices, opts...).Generate(t.Context(), request("tr", "sms", 10), nil)
		if err != nil {
			t.Fatal(err)
		}
		var c content.SMSContent
		decode(t, got.Content, &c)
		return c
	}
	alone := build()
	with := build(contentgen.WithSMSLink(content.PlaceholderLink(base)))
	if alone.Segments != 1 || alone.EstimatedCostUSD != "0.4000" || with.Segments != 2 || with.EstimatedCostUSD != "0.8000" || with.RenderedLength != alone.RenderedLength {
		t.Errorf("alone %+v, with the link %+v: want 1 then 2 segments, the cost doubling, the text length unchanged", alone, with)
	}

	// An edit is measured the same way.
	g := contentgen.New(llmtest.NewScripted(), demoPrices, contentgen.WithSMSBudget(content.SMSBudget(base)), contentgen.WithSMSLink(content.PlaceholderLink(base)))
	raw, err := g.Validate(request("tr", "sms", 10), json.RawMessage(`{"text":"`+text+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	var edited content.SMSContent
	decode(t, raw, &edited)
	if edited.Segments != 2 {
		t.Errorf("edited segments = %d, want 2", edited.Segments)
	}
}
