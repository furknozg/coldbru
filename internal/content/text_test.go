package content

import (
	"strings"
	"testing"
)

func TestExpand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, text, want string
	}{
		{"plain", "Hi {{first_name}}!", "Hi Ayşe!"},
		{"spaces", "Hi {{ first_name }}!", "Hi Ayşe!"},
		{"twice", "{{first_name}}, {{first_name}}", "Ayşe, Ayşe"},
		{"other variable left alone", "{{city}} {{first_name}}", "{{city}} Ayşe"},
		{"no placeholder", "Hello", "Hello"},
		{"name is not re-expanded", "{{first_name}}", "Ayşe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Expand(tt.text, "Ayşe"); got != tt.want {
				t.Errorf("Expand = %q, want %q", got, tt.want)
			}
		})
	}
	// A name that itself looks like a placeholder is inserted as is, not expanded again.
	if got := Expand("Hi {{first_name}}", "{{first_name}}"); got != "Hi {{first_name}}" {
		t.Errorf("Expand with a placeholder-like name = %q", got)
	}
}

func TestExpandMax(t *testing.T) {
	t.Parallel()
	if got := ExpandMax("a{{first_name}}b", 3); got != "axxxb" {
		t.Errorf("ExpandMax = %q", got)
	}
	if got := ExpandMax("a{{first_name}}b", 0); got != "ab" {
		t.Errorf("ExpandMax with 0 = %q", got)
	}
	if got := ExpandMax("a{{first_name}}b", -4); got != "ab" {
		t.Errorf("ExpandMax with a negative length = %q", got)
	}
}

func TestPlaceholderWhitelist(t *testing.T) {
	t.Parallel()
	tests := []struct {
		text string
		want []Problem
	}{
		{"{{first_name}}", nil},
		{"{{  first_name\t}}", nil},
		{"{{last_name}}", []Problem{{Field: "f", Rule: RulePlaceholder, Got: "{{last_name}}"}}},
		{"{{first_name}} {{city}} {{email}}", []Problem{
			{Field: "f", Rule: RulePlaceholder, Got: "{{city}}"}, {Field: "f", Rule: RulePlaceholder, Got: "{{email}}"}}},
		{"{{{first_name}}}", []Problem{{Field: "f", Rule: RulePlaceholderSyntax, Got: "{{{"}, {Field: "f", Rule: RulePlaceholderSyntax, Got: "}}}"}}},
		{"{{first_name", []Problem{{Field: "f", Rule: RulePlaceholderSyntax, Got: "{{"}}},
		{"first_name}}", []Problem{{Field: "f", Rule: RulePlaceholderSyntax, Got: "}}"}}},
		{"{first_name}", nil},
		{"{{ first name }}", []Problem{{Field: "f", Rule: RulePlaceholder, Got: "{{ first name }}"}}},
		{"{{FIRST_NAME}}", []Problem{{Field: "f", Rule: RulePlaceholder, Got: "{{FIRST_NAME}}"}}},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()
			got := checkPlaceholders("f", tt.text)
			if len(got) != len(tt.want) {
				t.Fatalf("problems = %+v, want %+v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("problem %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestURLAndHTMLRejection(t *testing.T) {
	t.Parallel()
	bad := []struct{ text, rule string }{
		{"see http://a.example/x", RuleURL},
		{"see HTTPS://A.EXAMPLE", RuleURL},
		{"see ftp://files.example", RuleURL},
		{"see www.example", RuleURL},
		{"see WWW.Example.com", RuleURL},
		{"write to mailto:a@b.co", RuleURL},
		{"click javascript:alert(1)", RuleURL},
		{"shop at indirim.com", RuleURL},
		{"shop at magaza.shop/sepet", RuleURL},
		{"hello <b>x</b>", RuleHTML},
		{"hello <script>alert(1)</script>", RuleHTML},
		{"hello <!-- c -->", RuleHTML},
		{"hello </p>", RuleHTML},
		{"hello < br >", RuleHTML},
		{"<img src=x onerror=y>", RuleHTML},
	}
	for _, tt := range bad {
		t.Run(tt.text, func(t *testing.T) {
			t.Parallel()
			found := false
			for _, p := range checkPlainText("f", tt.text, true) {
				found = found || p.Rule == tt.rule
			}
			if !found {
				t.Errorf("%q was not rejected with %s", tt.text, tt.rule)
			}
		})
	}
	good := []string{
		"Sepetindeki ürünler seni bekliyor.", "3 < 5 ve 5 > 3", "I <3 sales", "Save 30% today, e.g. on shoes.",
		"Saat 10.30'da başlıyor", "Fiyat 1.299,90 TL", "{{first_name}}, hoş geldin!", "A&B Ltd.", "R&D günü",
	}
	for _, text := range good {
		t.Run(text, func(t *testing.T) {
			t.Parallel()
			if p := checkPlainText("f", text, true); len(p) > 0 {
				t.Errorf("%q rejected: %+v", text, p)
			}
		})
	}
}

func TestSingleLineAndControlChars(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"a\nb", "a\rb", "a b"} {
		p := checkPlainText("f", text, false)
		if len(p) == 0 || p[0].Rule != RuleSingleLine {
			t.Errorf("%q: problems = %+v, want single_line", text, p)
		}
	}
	if p := checkPlainText("f", "a\nb", true); len(p) != 0 {
		t.Errorf("a line feed in a multiline field was rejected: %+v", p)
	}
	if p := checkPlainText("f", "a\tb", true); len(p) != 1 || p[0].Rule != RuleControlChar || p[0].Got != "U+0009" {
		t.Errorf("tab: problems = %+v", p)
	}
	if p := checkPlainText("f", "bad\xffbyte", true); len(p) != 1 || p[0].Rule != RuleEncoding {
		t.Errorf("invalid utf-8: problems = %+v", p)
	}
}

func TestProblemMessagesAndJSON(t *testing.T) {
	t.Parallel()
	p := Problem{Field: "sms.text", Rule: RuleMaxLength, Got: 187, Limit: 160}
	if m := p.Message(); !strings.Contains(m, "sms.text") || !strings.Contains(m, "187") || !strings.Contains(m, "160") {
		t.Errorf("message = %q", m)
	}
	b, err := p.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"field":"sms.text"`, `"rule":"max_length"`, `"got":187`, `"limit":160`, `"message":`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("json %s lacks %s", b, want)
		}
	}
	rules := []string{RuleEmpty, RuleMinItems, RuleMaxItems, RulePlaceholder, RulePlaceholderSyntax, RuleURL, RuleHTML,
		RuleSingleLine, RuleControlChar, RuleEncoding, "other"}
	for _, r := range rules {
		if (Problem{Field: "f", Rule: r, Got: 1, Limit: 2}).Message() == "" {
			t.Errorf("rule %s has no message", r)
		}
	}
	if err := AsError(nil); err != nil {
		t.Errorf("AsError(nil) = %v", err)
	}
	err = AsError([]Problem{p, {Field: "email.subject", Rule: RuleEmpty}})
	var ve *ValidationError
	if e, ok := err.(*ValidationError); !ok || len(e.Problems) != 2 || !strings.Contains(err.Error(), "; ") {
		t.Errorf("AsError = %v (%T)", err, ve)
	}
}
