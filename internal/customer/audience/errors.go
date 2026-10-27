package audience

import (
	"fmt"
	"strings"
)

// Error codes carried in the GraphQL extensions.code of audience errors.
const (
	// CodeBadUserInput is a malformed value: bad ago text, empty in list, bad
	// cursor or page size.
	CodeBadUserInput = "BAD_USER_INPUT"
	// CodeUnknownValue is a value outside a closed list (D-062); extensions
	// carry field, value and suggestions.
	CodeUnknownValue = "UNKNOWN_VALUE"
	// CodeComplexityLimit is a filter over the depth, node or in-list limits.
	CodeComplexityLimit = "COMPLEXITY_LIMIT"
)

// Error is a client-visible filter or argument error. The GraphQL layer turns
// it into a gqlerror with the same code and extensions.
type Error struct {
	Code    string
	Message string
	// FilterPath locates the offending value inside the filter argument, in
	// GraphQL path style: strings for fields, ints for list indexes, for example
	// ["filter", "and", 0, "city", "eq"].
	FilterPath []any
	// Extensions are extra machine-readable details (field, value, suggestions).
	Extensions map[string]any
}

// Error implements error.
func (e *Error) Error() string {
	if len(e.FilterPath) == 0 {
		return e.Code + ": " + e.Message
	}
	return e.Code + ": " + e.Message + " (at " + FormatPath(e.FilterPath) + ")"
}

// FormatPath renders a filter path as "filter.and[0].city.eq".
func FormatPath(path []any) string {
	var b strings.Builder
	for i, seg := range path {
		switch v := seg.(type) {
		case int:
			fmt.Fprintf(&b, "[%d]", v)
		default:
			if i > 0 {
				b.WriteByte('.')
			}
			fmt.Fprint(&b, v)
		}
	}
	return b.String()
}

// BadInput returns a BAD_USER_INPUT error for an argument that is not part of
// the filter (page size, cursor).
func BadInput(format string, args ...any) *Error {
	return &Error{Code: CodeBadUserInput, Message: fmt.Sprintf(format, args...)}
}

func badFilter(path []any, format string, args ...any) *Error {
	return &Error{Code: CodeBadUserInput, Message: fmt.Sprintf(format, args...), FilterPath: path}
}

func limitExceeded(path []any, format string, args ...any) *Error {
	return &Error{Code: CodeComplexityLimit, Message: fmt.Sprintf(format, args...), FilterPath: path}
}

func unknownValue(path []any, field, value string, suggestions []string) *Error {
	if suggestions == nil {
		suggestions = []string{}
	}
	msg := fmt.Sprintf("%q is not a known value of %s", value, field)
	if len(suggestions) > 0 {
		msg += "; did you mean " + strings.Join(quoteAll(suggestions), ", ") + "?"
	}
	return &Error{
		Code: CodeUnknownValue, Message: msg, FilterPath: path,
		Extensions: map[string]any{"field": field, "value": value, "suggestions": suggestions},
	}
}

func quoteAll(ss []string) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = fmt.Sprintf("%q", s)
	}
	return out
}

// withSeg returns path plus seg without aliasing path's backing array.
func withSeg(path []any, seg ...any) []any {
	out := make([]any, 0, len(path)+len(seg))
	out = append(out, path...)
	return append(out, seg...)
}
