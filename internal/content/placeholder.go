package content

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// FirstNameVar is the only variable of campaign text (D-071). Whitespace inside the braces is
// tolerated ("{{ first_name }}"), because models and hands both write it; the name is case sensitive.
const FirstNameVar = "first_name"

// FirstNameToken is the canonical spelling of the variable.
const FirstNameToken = "{{" + FirstNameVar + "}}"

// MeasureFiller is the character that stands in for a first name of a given length when a text is
// measured before the recipients' real names are known (preview, validation). It is plain ASCII
// (one GSM-7 septet, one UTF-16 unit), so the encoding and the segment count of a preview depend on
// the template text only; Render at send time uses the real name.
const MeasureFiller = "x"

// placeholderRE matches one {{...}} group without nested braces.
var placeholderRE = regexp.MustCompile(`\{\{([^{}]*)\}\}`)

// isFirstName reports whether the inside of a {{...}} group names the allowed variable.
func isFirstName(inner string) bool { return strings.TrimSpace(inner) == FirstNameVar }

// checkPlaceholders reports every {{...}} other than {{first_name}} and every stray "{{" or "}}".
func checkPlaceholders(field, text string) []Problem {
	var out []Problem
	for _, m := range placeholderRE.FindAllStringSubmatch(text, -1) {
		if !isFirstName(m[1]) {
			out = append(out, Problem{Field: field, Rule: RulePlaceholder, Got: m[0]})
		}
	}
	// Triple braces would leave single braces around an expanded name.
	for _, triple := range []string{"{{{", "}}}"} {
		if strings.Contains(text, triple) {
			out = append(out, Problem{Field: field, Rule: RulePlaceholderSyntax, Got: triple})
		}
	}
	rest := placeholderRE.ReplaceAllString(text, "")
	for _, stray := range []string{"{{", "}}"} {
		if strings.Contains(rest, stray) {
			out = append(out, Problem{Field: field, Rule: RulePlaceholderSyntax, Got: stray})
		}
	}
	return out
}

// Expand replaces every {{first_name}} of text by firstName. It is plain string replacement driven
// by a fixed pattern: text is never executed as a template. Any other {{...}} stays as written
// (validation rejects it before storage).
func Expand(text, firstName string) string {
	return placeholderRE.ReplaceAllStringFunc(text, func(m string) string {
		if isFirstName(m[2 : len(m)-2]) {
			return firstName
		}
		return m
	})
}

// ExpandMax expands text with a stand-in name of maxNameLength characters, the way the limits of
// D-028 are measured. A negative length is taken as zero.
func ExpandMax(text string, maxNameLength int) string {
	return Expand(text, strings.Repeat(MeasureFiller, max(maxNameLength, 0)))
}

// Patterns of text the model must not write (D-071).
var (
	// urlRE finds a scheme://, a www. address, a mailto/javascript/data scheme, or a bare domain with a
	// common top-level domain (indirim.com, shop.co/x).
	urlRE = regexp.MustCompile(`(?i)(?:[a-z][a-z0-9+.\-]*://\S*|\bwww\.\S*|\b(?:mailto|javascript|data|tel|sms):\S*|\b[a-z0-9][a-z0-9\-]*(?:\.[a-z0-9][a-z0-9\-]*)*\.(?:com|net|org|io|co|tr|uk|ly|me|app|shop|store|info|biz|xyz|link|click|site|online|dev)\b(?:/\S*)?)`)
	// tagRE finds the start of an HTML tag, comment or declaration. "<3" and "a < b" are not tags.
	tagRE = regexp.MustCompile(`<\s*[/!?a-zA-Z][^>]*>?`)
)

// checkPlainText applies the rules every text field shares: valid UTF-8, no control characters
// (a line feed is allowed when multiline is true), no URL, no HTML tag, only {{first_name}}.
func checkPlainText(field, text string, multiline bool) []Problem {
	if !utf8.ValidString(text) {
		return []Problem{{Field: field, Rule: RuleEncoding}}
	}
	var out []Problem
	if !multiline && strings.ContainsAny(text, "\r\n\u0085  ") {
		out = append(out, Problem{Field: field, Rule: RuleSingleLine})
	} else {
		for _, r := range text {
			if unicode.IsControl(r) && !(multiline && r == '\n') {
				out = append(out, Problem{Field: field, Rule: RuleControlChar, Got: runeName(r)})
				break
			}
		}
	}
	if m := urlRE.FindString(text); m != "" {
		out = append(out, Problem{Field: field, Rule: RuleURL, Got: m})
	}
	if m := tagRE.FindString(text); m != "" {
		out = append(out, Problem{Field: field, Rule: RuleHTML, Got: m})
	}
	return append(out, checkPlaceholders(field, text)...)
}

func runeName(r rune) string { return fmt.Sprintf("U+%04X", r) }
