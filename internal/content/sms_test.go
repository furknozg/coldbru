package content

import (
	"strings"
	"testing"
)

func TestMeasureSMSEncodings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		text  string
		enc   Encoding
		units int
	}{
		{"plain ascii", "Hello world", GSM7, 11},
		{"turkish gsm letters", "ö ü Ö Ü Ç", GSM7, 9},
		{"lowercase c cedilla forces ucs2", "ç", UCS2, 1},
		{"s cedilla", "ş", UCS2, 1},
		{"capital S cedilla", "Ş", UCS2, 1},
		{"g breve", "ğ", UCS2, 1},
		{"capital G breve", "Ğ", UCS2, 1},
		{"dotless i", "ı", UCS2, 1},
		{"capital I with dot", "İ", UCS2, 1},
		{"one turkish letter flips the whole text", "Merhaba sepetin seni bekliyor ş", UCS2, 31},
		{"euro is an extension char", "€", GSM7, 2},
		{"extension chars count two", "[ ] { } \\ ^ ~ |", GSM7, 8*2 + 7},
		{"each extension char", "€[]{}\\^~|", GSM7, 18},
		{"newline is gsm", "a\nb", GSM7, 3},
		{"emoji is two utf-16 units", "😀", UCS2, 2},
		{"emoji among ascii", "Hi 😀", UCS2, 5},
		{"empty", "", GSM7, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := MeasureSMS(tt.text)
			if got.Encoding != tt.enc || got.Units != tt.units {
				t.Errorf("MeasureSMS(%q) = %+v, want %s with %d units", tt.text, got, tt.enc, tt.units)
			}
		})
	}
}

func TestMeasureSMSSegmentBoundaries(t *testing.T) {
	t.Parallel()
	rep := func(s string, n int) string { return strings.Repeat(s, n) }
	tests := []struct {
		name string
		text string
		want int
	}{
		{"empty", "", 0},
		{"gsm 160", rep("a", 160), 1},
		{"gsm 161", rep("a", 161), 2},
		{"gsm 2x153", rep("a", 306), 2},
		{"gsm 2x153+1", rep("a", 307), 3},
		{"gsm extension straddling the boundary moves over", rep("a", 152) + "€" + rep("a", 150), 2},
		{"gsm extension pushes past 306", rep("a", 305) + "€", 3},
		{"ucs2 70", rep("ş", 70), 1},
		{"ucs2 71", rep("ş", 71), 2},
		{"ucs2 2x67", rep("ş", 134), 2},
		{"ucs2 2x67+1", rep("ş", 135), 3},
		{"emoji that would straddle needs a third segment", rep("ş", 66) + "😀" + rep("ş", 66), 3},
		{"emoji fits when it does not straddle", rep("ş", 66) + "😀" + rep("ş", 65), 2},
		{"emoji units count in the single limit", rep("ş", 69) + "😀", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := MeasureSMS(tt.text).Segments; got != tt.want {
				t.Errorf("segments = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestMeasureSMSLengthCountsCharactersNotUnits(t *testing.T) {
	t.Parallel()
	got := MeasureSMS("Hi 😀 €")
	if got.Length != 6 || got.Units != 7 || got.Encoding != UCS2 {
		t.Errorf("MeasureSMS = %+v, want 6 characters, 7 UTF-16 units, ucs2", got)
	}
}

func TestCheckSMS(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		text     string
		maxName  int
		wantRule string // "" for valid
		length   int
	}{
		{"valid with name", "Merhaba {{first_name}}!", 8, "", 17},
		{"spaces inside braces", "Hi {{ first_name }}", 5, "", 8},
		{"exactly 160 after expansion", strings.Repeat("a", 150) + "{{first_name}}", 10, "", 160},
		{"161 after expansion", strings.Repeat("a", 151) + "{{first_name}}", 10, RuleMaxLength, 161},
		{"long name pushes over", "{{first_name}} " + strings.Repeat("a", 140), 30, RuleMaxLength, 171},
		{"placeholder twice counted twice", strings.Repeat("{{first_name}}", 2), 80, "", 160},
		{"empty", "  ", 5, RuleEmpty, 0},
		{"other variable", "Hi {{city}}", 5, RulePlaceholder, 0},
		{"wrong case", "Hi {{First_Name}}", 5, RulePlaceholder, 0},
		{"empty braces", "Hi {{}}", 5, RulePlaceholder, 0},
		{"unclosed", "Hi {{first_name", 5, RulePlaceholderSyntax, 0},
		{"stray closing", "Hi first_name}}", 5, RulePlaceholderSyntax, 0},
		{"url https", "Gel https://shop.example.com/x", 5, RuleURL, 0},
		{"url www", "Gel www.shop.com", 5, RuleURL, 0},
		{"bare domain", "Gel indirim.com", 5, RuleURL, 0},
		{"html tag", "Hi <b>there</b>", 5, RuleHTML, 0},
		{"less than sign is fine", "Hi <3 you", 5, "", 9},
		{"control char", "Hi\x07there", 5, RuleControlChar, 0},
		{"newline allowed", "Hi\nthere", 5, "", 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info, problems := CheckSMS(tt.text, tt.maxName)
			if tt.wantRule == "" {
				if len(problems) > 0 {
					t.Fatalf("problems = %+v, want none", problems)
				}
				if info.Length != tt.length {
					t.Errorf("length = %d, want %d", info.Length, tt.length)
				}
				return
			}
			found := false
			for _, p := range problems {
				if p.Rule == tt.wantRule && p.Field == "sms.text" {
					found = true
				}
			}
			if !found {
				t.Fatalf("problems = %+v, want rule %s on sms.text", problems, tt.wantRule)
			}
			if tt.wantRule == RuleMaxLength {
				p := problems[0]
				if p.Got != tt.length || p.Limit != SMSMaxChars {
					t.Errorf("problem = %+v, want got %d limit 160", p, tt.length)
				}
			}
		})
	}
}

func TestCheckSMSExpandedLengthDecidesNotTemplateLength(t *testing.T) {
	t.Parallel()
	// 14 characters of template, but 154 with a 144-character name.
	_, p := CheckSMS("{{first_name}}", 144)
	if len(p) != 0 {
		t.Fatalf("problems = %+v", p)
	}
	_, p = CheckSMS("{{first_name}}a", 160)
	if len(p) != 1 || p[0].Rule != RuleMaxLength {
		t.Fatalf("problems = %+v, want one max_length", p)
	}
}

func TestCheckSMSMeasuresEncodingOfTheTemplate(t *testing.T) {
	t.Parallel()
	tr, _ := CheckSMS("Sepetin seni bekliyor, alışveriş yap {{first_name}}!", 8)
	if tr.Encoding != UCS2 {
		t.Errorf("a name placeholder must not decide the encoding; %q is UCS-2 because of ı", "bekliyor")
	}
	en, _ := CheckSMS("Your cart waits, {{first_name}}!", 8)
	if en.Encoding != GSM7 || en.Segments != 1 {
		t.Errorf("english text = %+v, want gsm7 in 1 segment", en)
	}
}
