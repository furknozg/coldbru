package content

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Rules of a Problem.
const (
	// RuleMaxLength: the text is longer than its limit (after expanding {{first_name}}).
	RuleMaxLength = "max_length"
	// RuleEmpty: the text is empty after trimming.
	RuleEmpty = "empty"
	// RuleMinItems and RuleMaxItems: a list has too few or too many items.
	RuleMinItems = "min_items"
	RuleMaxItems = "max_items"
	// RulePlaceholder: a {{...}} other than {{first_name}}.
	RulePlaceholder = "placeholder"
	// RulePlaceholderSyntax: a stray "{{" or "}}" that is not a whole placeholder.
	RulePlaceholderSyntax = "placeholder_syntax"
	// RuleURL: the text contains a URL or a web address.
	RuleURL = "url"
	// RuleHTML: the text contains an HTML tag.
	RuleHTML = "html"
	// RuleSingleLine: the field must be a single line.
	RuleSingleLine = "single_line"
	// RuleControlChar: the text contains a control character.
	RuleControlChar = "control_char"
	// RuleEncoding: the text is not valid UTF-8.
	RuleEncoding = "encoding"
	// RuleInvalidOutput: the model's answer was cut off (max_tokens) or was not valid JSON, so there
	// is nothing to validate. Field is empty.
	RuleInvalidOutput = "invalid_output"
)

// Problem is one rule violation of a message, structured so that the marketer's UI can point at the
// field and the model can be told exactly what to fix (D-071). Field is the dotted path in the
// stored shape, for example "sms.text", "email.subject" or "email.paragraphs[2]". Got and Limit are
// set when they make sense: a length and its limit, the offending variable or URL.
type Problem struct {
	Field string `json:"field"`
	Rule  string `json:"rule"`
	Got   any    `json:"got,omitempty"`
	Limit any    `json:"limit,omitempty"`
}

// Message is the problem as one English sentence, for the model and for logs.
func (p Problem) Message() string {
	switch p.Rule {
	case RuleMaxLength:
		return fmt.Sprintf("%s is %v characters long after {{first_name}} is replaced by the longest first name; the limit is %v", p.Field, p.Got, p.Limit)
	case RuleEmpty:
		return fmt.Sprintf("%s must not be empty", p.Field)
	case RuleMinItems:
		return fmt.Sprintf("%s has %v items; at least %v is required", p.Field, p.Got, p.Limit)
	case RuleMaxItems:
		return fmt.Sprintf("%s has %v items; at most %v are allowed", p.Field, p.Got, p.Limit)
	case RulePlaceholder:
		return fmt.Sprintf("%s uses the variable %v; only {{first_name}} may be used", p.Field, p.Got)
	case RulePlaceholderSyntax:
		return fmt.Sprintf("%s has a stray %v; a variable must be written exactly as {{first_name}}", p.Field, p.Got)
	case RuleURL:
		return fmt.Sprintf("%s contains a URL or web address (%v); links are not allowed", p.Field, p.Got)
	case RuleHTML:
		return fmt.Sprintf("%s contains an HTML tag (%v); write plain text only", p.Field, p.Got)
	case RuleSingleLine:
		return fmt.Sprintf("%s must be a single line without line breaks", p.Field)
	case RuleControlChar:
		return fmt.Sprintf("%s contains a control character (%v)", p.Field, p.Got)
	case RuleEncoding:
		return fmt.Sprintf("%s is not valid UTF-8", p.Field)
	case RuleInvalidOutput:
		return "the previous answer was cut off or was not valid JSON; answer again with the complete, shorter JSON object"
	}
	s := fmt.Sprintf("%s breaks the rule %s", p.Field, p.Rule)
	if p.Got != nil {
		s += fmt.Sprintf(" (got %v", p.Got)
		if p.Limit != nil {
			s += fmt.Sprintf(", limit %v", p.Limit)
		}
		s += ")"
	}
	return s
}

// MarshalJSON adds the English message to the structured fields.
func (p Problem) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Field   string `json:"field"`
		Rule    string `json:"rule"`
		Got     any    `json:"got,omitempty"`
		Limit   any    `json:"limit,omitempty"`
		Message string `json:"message"`
	}{p.Field, p.Rule, p.Got, p.Limit, p.Message()})
}

// Messages returns the messages of problems, one per problem.
func Messages(problems []Problem) []string {
	out := make([]string, len(problems))
	for i, p := range problems {
		out[i] = p.Message()
	}
	return out
}

// ValidationError is the error form of a non-empty problem list.
type ValidationError struct{ Problems []Problem }

func (e *ValidationError) Error() string { return strings.Join(Messages(e.Problems), "; ") }

// AsError returns nil for an empty list, otherwise a *ValidationError.
func AsError(problems []Problem) error {
	if len(problems) == 0 {
		return nil
	}
	return &ValidationError{Problems: problems}
}
