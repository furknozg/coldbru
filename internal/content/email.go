package content

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Email is the text of an email as the model writes it (D-071): blocks, no markup. Every text may
// contain {{first_name}}.
type Email struct {
	Subject    string   `json:"subject"`
	Preheader  string   `json:"preheader"`
	Headline   string   `json:"headline"`
	Paragraphs []string `json:"paragraphs"`
	CTALabel   string   `json:"cta_label"`
}

// EmailLengths are the lengths of the blocks in Unicode characters with {{first_name}} replaced by
// the longest first name; Paragraph is the longest paragraph.
type EmailLengths struct {
	Subject   int `json:"subject"`
	Preheader int `json:"preheader"`
	Headline  int `json:"headline"`
	Paragraph int `json:"paragraph"`
	CTALabel  int `json:"cta_label"`
}

// CheckEmail validates the blocks of an email and measures them with {{first_name}} replaced by a
// name of maxNameLength characters. Subject, preheader, headline and label are single lines, every
// block is non-empty after trimming. The lengths are returned even when problems are found.
func CheckEmail(e Email, maxNameLength int) (EmailLengths, []Problem) {
	var problems []Problem
	var lens EmailLengths
	one := func(field, text string, limit int, dst *int) {
		if strings.TrimSpace(text) == "" {
			problems = append(problems, Problem{Field: field, Rule: RuleEmpty})
			return
		}
		problems = append(problems, checkPlainText(field, text, false)...)
		if !utf8.ValidString(text) {
			return
		}
		n := utf8.RuneCountInString(ExpandMax(text, maxNameLength))
		*dst = max(*dst, n)
		if n > limit {
			problems = append(problems, Problem{Field: field, Rule: RuleMaxLength, Got: n, Limit: limit})
		}
	}
	one("email.subject", e.Subject, SubjectMax, &lens.Subject)
	one("email.preheader", e.Preheader, PreheaderMax, &lens.Preheader)
	one("email.headline", e.Headline, HeadlineMax, &lens.Headline)
	switch n := len(e.Paragraphs); {
	case n < ParagraphsMin:
		problems = append(problems, Problem{Field: "email.paragraphs", Rule: RuleMinItems, Got: n, Limit: ParagraphsMin})
	case n > ParagraphsMax:
		problems = append(problems, Problem{Field: "email.paragraphs", Rule: RuleMaxItems, Got: n, Limit: ParagraphsMax})
	}
	for i, p := range e.Paragraphs {
		one(fmt.Sprintf("email.paragraphs[%d]", i), p, ParagraphMax, &lens.Paragraph)
	}
	one("email.cta_label", e.CTALabel, CTALabelMax, &lens.CTALabel)
	return lens, problems
}
