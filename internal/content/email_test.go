package content

import (
	"strings"
	"testing"
)

func validEmail() Email {
	return Email{
		Subject:    "Sepetin seni bekliyor, {{first_name}}",
		Preheader:  "%30 indirim sadece bu hafta",
		Headline:   "Merhaba {{first_name}}!",
		Paragraphs: []string{"Sepetindeki ürünler hâlâ seni bekliyor.", "%30 indirim fırsatını kaçırma."},
		CTALabel:   "Sepete dön",
	}
}

func TestCheckEmailValid(t *testing.T) {
	t.Parallel()
	lens, problems := CheckEmail(validEmail(), 10)
	if len(problems) > 0 {
		t.Fatalf("problems = %+v", problems)
	}
	want := EmailLengths{
		Subject:   23 + 10, // "Sepetin seni bekliyor, " plus the name
		Preheader: len([]rune("%30 indirim sadece bu hafta")),
		Headline:  8 + 10 + 1, // "Merhaba ", the name, "!"
		Paragraph: len([]rune("Sepetindeki ürünler hâlâ seni bekliyor.")),
		CTALabel:  len([]rune("Sepete dön")),
	}
	if lens != want {
		t.Errorf("lengths = %+v, want %+v", lens, want)
	}
}

func TestCheckEmailLimits(t *testing.T) {
	t.Parallel()
	rep := func(n int) string { return strings.Repeat("a", n) }
	tests := []struct {
		name   string
		mutate func(e *Email)
		field  string
		rule   string
		got    any
		limit  any
	}{
		{"subject 80 ok", func(e *Email) { e.Subject = rep(80) }, "", "", nil, nil},
		{"subject 81", func(e *Email) { e.Subject = rep(81) }, "email.subject", RuleMaxLength, 81, 80},
		{"subject expanded over", func(e *Email) { e.Subject = rep(75) + "{{first_name}}" }, "email.subject", RuleMaxLength, 85, 80},
		{"preheader 100 ok", func(e *Email) { e.Preheader = rep(100) }, "", "", nil, nil},
		{"preheader 101", func(e *Email) { e.Preheader = rep(101) }, "email.preheader", RuleMaxLength, 101, 100},
		{"headline 80 ok", func(e *Email) { e.Headline = rep(80) }, "", "", nil, nil},
		{"headline 81", func(e *Email) { e.Headline = rep(81) }, "email.headline", RuleMaxLength, 81, 80},
		{"paragraph 600 ok", func(e *Email) { e.Paragraphs = []string{rep(600)} }, "", "", nil, nil},
		{"paragraph 601", func(e *Email) { e.Paragraphs = []string{"ok", rep(601)} }, "email.paragraphs[1]", RuleMaxLength, 601, 600},
		{"cta 30 ok", func(e *Email) { e.CTALabel = rep(30) }, "", "", nil, nil},
		{"cta 31", func(e *Email) { e.CTALabel = rep(31) }, "email.cta_label", RuleMaxLength, 31, 30},
		{"one paragraph ok", func(e *Email) { e.Paragraphs = []string{"x"} }, "", "", nil, nil},
		{"four paragraphs ok", func(e *Email) { e.Paragraphs = []string{"a", "b", "c", "d"} }, "", "", nil, nil},
		{"no paragraphs", func(e *Email) { e.Paragraphs = nil }, "email.paragraphs", RuleMinItems, 0, 1},
		{"five paragraphs", func(e *Email) { e.Paragraphs = []string{"a", "b", "c", "d", "e"} }, "email.paragraphs", RuleMaxItems, 5, 4},
		{"empty subject", func(e *Email) { e.Subject = "   " }, "email.subject", RuleEmpty, nil, nil},
		{"empty preheader", func(e *Email) { e.Preheader = "" }, "email.preheader", RuleEmpty, nil, nil},
		{"empty paragraph", func(e *Email) { e.Paragraphs = []string{"a", " "} }, "email.paragraphs[1]", RuleEmpty, nil, nil},
		{"empty cta", func(e *Email) { e.CTALabel = "" }, "email.cta_label", RuleEmpty, nil, nil},
		{"newline in subject", func(e *Email) { e.Subject = "a\nBcc: x@y.z" }, "email.subject", RuleSingleLine, nil, nil},
		{"url in paragraph", func(e *Email) { e.Paragraphs = []string{"see https://x.example"} }, "email.paragraphs[0]", RuleURL, "https://x.example", nil},
		{"html in headline", func(e *Email) { e.Headline = "<b>Hi</b>" }, "email.headline", RuleHTML, "<b>", nil},
		{"other variable in cta", func(e *Email) { e.CTALabel = "Go {{city}}" }, "email.cta_label", RulePlaceholder, "{{city}}", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := validEmail()
			tt.mutate(&e)
			_, problems := CheckEmail(e, 10)
			if tt.rule == "" {
				if len(problems) > 0 {
					t.Fatalf("problems = %+v, want none", problems)
				}
				return
			}
			want := Problem{Field: tt.field, Rule: tt.rule, Got: tt.got, Limit: tt.limit}
			for _, p := range problems {
				if p == want {
					return
				}
			}
			t.Fatalf("problems = %+v, want to contain %+v", problems, want)
		})
	}
}

func TestCheckEmailReportsEveryProblem(t *testing.T) {
	t.Parallel()
	e := Email{Subject: strings.Repeat("a", 90), Preheader: "", Headline: "{{x}}", Paragraphs: []string{"a", "b", "c", "d", "e"}, CTALabel: "ok"}
	_, problems := CheckEmail(e, 5)
	var fields []string
	for _, p := range problems {
		fields = append(fields, p.Field+":"+p.Rule)
	}
	want := []string{"email.subject:max_length", "email.preheader:empty", "email.headline:placeholder", "email.paragraphs:max_items"}
	for _, w := range want {
		found := false
		for _, f := range fields {
			found = found || f == w
		}
		if !found {
			t.Errorf("missing %s in %v", w, fields)
		}
	}
}

func TestBuildSMSAndEmail(t *testing.T) {
	t.Parallel()
	prices := Prices{SMSSegment: 40_000, Email: 1_000}
	sms, problems := BuildSMS("Merhaba {{first_name}}, sepetini unutma!", 8, 33, prices)
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	// "Merhaba " 8 + 8 + ", sepetini unutma!" 18 = 34 characters, GSM-7 in one segment.
	if sms.Encoding != GSM7 || sms.Segments != 1 || sms.RenderedLength != 34 || sms.EstimatedCostGBP != "1.3200" {
		t.Errorf("sms = %+v", sms)
	}
	tr, _ := BuildSMS("Şimdi sepete dön {{first_name}}", 8, 10, prices)
	if tr.Encoding != UCS2 || tr.Segments != 1 || tr.EstimatedCostGBP != "0.4000" {
		t.Errorf("tr = %+v", tr)
	}
	long, _ := BuildSMS(strings.Repeat("ş", 100), 0, 10, prices)
	if long.Segments != 2 || long.EstimatedCostGBP != "0.8000" {
		t.Errorf("long = %+v, want 2 segments x 10 x 0.04", long)
	}
	if got, p := BuildSMS(strings.Repeat("a", 161), 0, 10, prices); len(p) == 0 || got != (SMSContent{}) {
		t.Errorf("an over-long text was built: %+v %+v", got, p)
	}

	email, problems := BuildEmail(validEmail(), 10, 33, prices)
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	if email.EstimatedCostGBP != "0.0330" || email.MaxRenderedLengths.Subject != 33 || email.Email().Subject != validEmail().Subject {
		t.Errorf("email = %+v", email)
	}
	if got, p := BuildEmail(Email{}, 10, 33, prices); len(p) == 0 || got.Subject != "" {
		t.Errorf("an empty email was built: %+v", got)
	}
}
