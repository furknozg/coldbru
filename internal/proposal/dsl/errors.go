package dsl

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Code is the stable, machine-readable error code of a DSL error
// (docs/coldbru-gqdsl.md §12.1, D-065). The string values are part of the
// contract with the LLM repair loop (D-027) and the semantic pass.
type Code string

// Codes produced by this package (the parser, D-065).
const (
	// CodeSyntaxError is an invalid expression grammar.
	CodeSyntaxError Code = "SYNTAX_ERROR"
	// CodeUnsupportedOperator is a reserved or rejected operator (~=, ==, !=)
	// or a field-to-field comparison (D-045).
	CodeUnsupportedOperator Code = "UNSUPPORTED_OPERATOR"
	// CodeUnsupportedExpression is a JavaScript construct outside the DSL:
	// calls other than ago/date/some/none/every, assignment, arrow functions.
	CodeUnsupportedExpression Code = "UNSUPPORTED_EXPRESSION"
	// CodeInvalidRelationScope is a lambda scoping violation.
	CodeInvalidRelationScope Code = "INVALID_RELATION_SCOPE"
	// CodeComplexityLimit is a source, depth, node or in-list limit (D-062, D-065).
	CodeComplexityLimit Code = "COMPLEXITY_LIMIT"
)

// Codes of the semantic pass (T-019). Declared here so that both passes share
// one vocabulary; the parser never produces them.
const (
	CodeUnknownField    Code = "UNKNOWN_FIELD"
	CodeUnknownRelation Code = "UNKNOWN_RELATION"
	CodeUnknownValue    Code = "UNKNOWN_VALUE"
	CodeTypeMismatch    Code = "TYPE_MISMATCH"
)

// Pos is a position in the source. Offset is a 0-based byte offset, Line is
// 1-based and Col is the 1-based column counted in runes (so "İ" is one column).
type Pos struct {
	Offset int
	Line   int
	Col    int
}

// Error is the only error type Parse returns. Start and End delimit the
// offending source range (End is exclusive; for an error at the end of the
// input both equal the end position).
type Error struct {
	Code    Code
	Message string
	Start   Pos
	End     Pos
	// Near is a short, single-line excerpt of the source around the error,
	// meant for the LLM repair prompt (D-027).
	Near string
	// Suggestions are close names or values the semantic pass offers for an
	// unknown field, relation or value (D-066). Error() does not print them;
	// the message already carries a `did you mean` hint.
	Suggestions []string
}

// Error formats as `SYNTAX_ERROR at 1:15: <message> near "<excerpt>"`.
func (e *Error) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s at %d:%d: %s", e.Code, e.Start.Line, e.Start.Col, e.Message)
	if e.Near != "" {
		b.WriteString(` near "`)
		b.WriteString(e.Near)
		b.WriteString(`"`)
	}
	return b.String()
}

const (
	maxNearRunes = 40
	eofNearRunes = 20
)

// NewError builds an Error for the semantic pass with the same Near excerpt as
// the parser's errors (D-065, D-066). start and end are positions in src.
func NewError(src string, code Code, msg string, start, end Pos) *Error {
	return newError(src, code, msg, start, end)
}

func newError(src string, code Code, msg string, start, end Pos) *Error {
	return &Error{Code: code, Message: msg, Start: start, End: end, Near: nearExcerpt(src, start, end)}
}

// nearExcerpt returns the source text of the range, flattened to one line and
// truncated. A range at the end of the input shows the tail of the source.
func nearExcerpt(src string, start, end Pos) string {
	s, e := start.Offset, end.Offset
	if s < 0 || s > len(src) {
		return ""
	}
	if e < s || e > len(src) {
		e = len(src)
	}
	text := src[s:e]
	if text == "" && s == len(src) {
		from := max(len(src)-eofNearRunes, 0)
		for from < len(src) && !utf8.RuneStart(src[from]) {
			from++
		}
		text = src[from:]
	}
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, text)
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) > maxNearRunes {
		rs := []rune(text)
		text = string(rs[:maxNearRunes]) + "…"
	}
	return text
}
