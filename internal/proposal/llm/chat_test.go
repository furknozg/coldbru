package llm_test

import (
	"strings"
	"testing"

	"github.com/furknozg/coldbru/internal/proposal/llm"
)

func TestCheckQuestion(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		q    llm.Question
		want string // substring of a problem; "" means accepted
	}{
		{"plain", llm.Question{Text: " Hangi şehir? "}, ""},
		{"with options", llm.Question{Text: "Hangi şehir?", Options: []string{" Ankara", "İzmir "}}, ""},
		{"four options", llm.Question{Text: "q?", Options: []string{"a", "b", "c", "d"}}, ""},
		{"blank", llm.Question{Text: " \n"}, "must not be blank"},
		{"too long", llm.Question{Text: strings.Repeat("s", llm.MaxQuestionChars+1)}, "characters"},
		{"html", llm.Question{Text: "Hangi <b>şehir</b>?"}, "HTML"},
		{"url", llm.Question{Text: "See www.example.com ?"}, "links"},
		{"one option", llm.Question{Text: "q?", Options: []string{"a"}}, `"options" has 1 items`},
		{"five options", llm.Question{Text: "q?", Options: []string{"a", "b", "c", "d", "e"}}, `"options" has 5 items`},
		{"empty option", llm.Question{Text: "q?", Options: []string{"a", " "}}, `"options[1]" is blank`},
		{"duplicate option", llm.Question{Text: "q?", Options: []string{"a", "a"}}, "repeats"},
		{"long option", llm.Question{Text: "q?", Options: []string{"a", strings.Repeat("b", llm.MaxOptionChars+1)}}, `"options[1]" is`},
		{"option with link", llm.Question{Text: "q?", Options: []string{"a", "[x](http://y)"}}, "links"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			q := tt.q
			problems := llm.CheckQuestion(&q)
			if tt.want == "" {
				if len(problems) != 0 {
					t.Fatalf("problems = %v, want none", problems)
				}
				if q.Text != strings.TrimSpace(tt.q.Text) {
					t.Errorf("text not trimmed: %q", q.Text)
				}
				for _, o := range q.Options {
					if o != strings.TrimSpace(o) {
						t.Errorf("option not trimmed: %q", o)
					}
				}
				return
			}
			if !strings.Contains(strings.Join(problems, "; "), tt.want) {
				t.Errorf("problems = %v, want one containing %q", problems, tt.want)
			}
		})
	}
}

func TestCheckSummary(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in, want string
	}{
		{"markdown list and bold", " - **Kitle**: 46 kişi\n- *İstanbul* ", ""},
		{"inline code", "Filtre: `city === \"İstanbul\"`.", ""},
		{"blank", "  ", "must not be blank"},
		{"too long", strings.Repeat("x", llm.MaxSummaryChars+1), "characters"},
		{"heading", "Özet\n## Kitle\nok", "headings"},
		{"heading first line", "# Kitle", "headings"},
		{"markdown link", "see [here](https://x.y)", "links"},
		{"bare url", "see https://x.y/z", "links"},
		{"html", "<p>ok</p>", "HTML"},
		{"html self-closing", "a<br/>b", "HTML"},
		{"less-than is fine", "a < b and 3 <5", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			md, problems := llm.CheckSummary(tt.in)
			if tt.want == "" {
				if len(problems) != 0 || md != strings.TrimSpace(tt.in) {
					t.Errorf("= %q, %v", md, problems)
				}
				return
			}
			if !strings.Contains(strings.Join(problems, "; "), tt.want) {
				t.Errorf("problems = %v, want one containing %q", problems, tt.want)
			}
		})
	}
}

func TestCheckTitle(t *testing.T) {
	t.Parallel()
	if got, err := llm.CheckTitle("  İstanbul sepet hatırlatması %30 "); err != nil || got != "İstanbul sepet hatırlatması %30" {
		t.Errorf("= %q, %v", got, err)
	}
	if got, err := llm.CheckTitle(strings.Repeat("ö", llm.MaxTitleChars)); err != nil || len([]rune(got)) != 60 {
		t.Errorf("60 characters: %q, %v", got, err)
	}
	for name, in := range map[string]string{"blank": " \t", "61 characters": strings.Repeat("ö", 61), "two lines": "a\nb"} {
		if _, err := llm.CheckTitle(in); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if llm.QuestionLimitProblem == "" || llm.MaxQuestionsPerStep != 2 {
		t.Error("limits changed")
	}
}
