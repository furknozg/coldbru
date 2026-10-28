package contentgen_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/furknozg/coldbru/internal/content"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/llmtest"
)

func TestValidateRecomputesMeasurements(t *testing.T) {
	t.Parallel()
	g := newGenerator(llmtest.NewScripted())
	// A stored variant echoed back by the UI with stale numbers: they are recomputed.
	stale := `{"text":"Hello {{first_name}}!","encoding":"ucs2","segments":9,"rendered_length":1,"estimated_cost_gbp":"99.0000"}`
	raw, err := g.Validate(request("en", "sms", 10), json.RawMessage(stale))
	if err != nil {
		t.Fatal(err)
	}
	var got content.SMSContent
	decode(t, raw, &got)
	want := content.SMSContent{Text: "Hello {{first_name}}!", Encoding: content.GSM7, Segments: 1, RenderedLength: len("Hello ") + 7 + len("!"), EstimatedCostGBP: "0.4000"}
	if got != want {
		t.Errorf("Validate = %+v, want %+v", got, want)
	}
	// Plain text from the editor.
	raw, err = g.Validate(request("tr", "sms", 33), json.RawMessage(`{"text":"Şimdi sepete dön!"}`))
	if err != nil {
		t.Fatal(err)
	}
	decode(t, raw, &got)
	if got.Encoding != content.UCS2 || got.Segments != 1 || got.EstimatedCostGBP != "1.3200" {
		t.Errorf("Validate = %+v", got)
	}
}

func TestValidateEmail(t *testing.T) {
	t.Parallel()
	g := newGenerator(llmtest.NewScripted())
	edit := emailMap("Selam {{first_name}}")
	edit["estimated_cost_gbp"] = "0.0000"
	edit["max_rendered_lengths"] = map[string]int{"subject": 1}
	b, _ := json.Marshal(edit)
	raw, err := g.Validate(request("tr", "email", 33), b)
	if err != nil {
		t.Fatal(err)
	}
	var got content.EmailContent
	decode(t, raw, &got)
	if got.EstimatedCostGBP != "0.0330" || got.MaxRenderedLengths.Subject != len("Selam ")+9 {
		t.Errorf("Validate = %+v", got)
	}
}

func TestValidateProblems(t *testing.T) {
	t.Parallel()
	g := newGenerator(llmtest.NewScripted())
	tests := []struct {
		name    string
		channel string
		in      string
		want    string // part of a problem message
	}{
		{"sms over 160 with the name", "sms", `{"text":"` + rep("a", 152) + `{{first_name}}"}`, "161 characters"},
		{"sms other variable", "sms", `{"text":"Hi {{city}}"}`, "{{city}}"},
		{"sms url", "sms", `{"text":"go to shop.example.com"}`, "web address"},
		{"sms html", "sms", `{"text":"<b>hi</b>"}`, "HTML tag"},
		{"sms empty", "sms", `{"text":"  "}`, "must not be empty"},
		{"sms missing text", "sms", `{}`, "sms.text"},
		{"sms unknown field", "sms", `{"text":"a","extra":1}`, "unknown field"},
		{"sms not an object", "sms", `"hello"`, "cannot unmarshal"},
		{"email missing blocks", "email", `{"subject":"s"}`, "email.preheader"},
		{"email subject 81", "email", `{"subject":"` + rep("s", 81) + `","preheader":"p","headline":"h","paragraphs":["x"],"cta_label":"c"}`, "81 characters"},
		{"email five paragraphs", "email", `{"subject":"s","preheader":"p","headline":"h","paragraphs":["a","b","c","d","e"],"cta_label":"c"}`, "5 items"},
		{"email script paragraph", "email", `{"subject":"s","preheader":"p","headline":"h","paragraphs":["<script>x</script>"],"cta_label":"c"}`, "HTML tag"},
		{"email line break in subject", "email", `{"subject":"a\nb","preheader":"p","headline":"h","paragraphs":["x"],"cta_label":"c"}`, "single line"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			raw, err := g.Validate(request("tr", tt.channel, 5), json.RawMessage(tt.in))
			if err == nil || raw != nil {
				t.Fatalf("Validate = %s, %v", raw, err)
			}
			problems := llm.Problems(err)
			if len(problems) == 0 || !strings.Contains(strings.Join(problems, " | "), tt.want) {
				t.Errorf("problems = %q, want one with %q", problems, tt.want)
			}
		})
	}
}

func TestValidateListsEveryProblem(t *testing.T) {
	t.Parallel()
	g := newGenerator(llmtest.NewScripted())
	_, err := g.Validate(request("tr", "email", 5), json.RawMessage(
		`{"subject":"`+rep("s", 90)+`","preheader":"","headline":"{{x}}","paragraphs":[],"cta_label":"ok"}`))
	var pe *llm.ProblemsError
	if !errors.As(err, &pe) || len(pe.Problems) != 4 {
		t.Fatalf("err = %v, want 4 problems (one per message)", err)
	}
}

func TestValidateRejectsAnUnusableRequest(t *testing.T) {
	t.Parallel()
	g := newGenerator(llmtest.NewScripted())
	if _, err := g.Validate(request("tr", "push", 1), json.RawMessage(`{"text":"a"}`)); err == nil {
		t.Error("an unknown channel was accepted")
	}
}
