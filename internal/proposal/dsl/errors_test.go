package dsl

import (
	"strings"
	"testing"
)

func TestErrorString(t *testing.T) {
	t.Parallel()
	e := parseErr(t, "city === \"İstanbul\" && x == 1")
	want := `UNSUPPORTED_OPERATOR at 1:26: ` + e.Message + ` near "=="`
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	e = parseErr(t, "a === 1 &&\n  b == 2")
	if got := e.Error(); !strings.HasPrefix(got, "UNSUPPORTED_OPERATOR at 2:5: ") || !strings.HasSuffix(got, ` near "=="`) {
		t.Errorf("Error() = %q", got)
	}
}

func TestErrorStringWithoutNear(t *testing.T) {
	t.Parallel()
	e := &Error{Code: CodeSyntaxError, Message: "boom", Start: Pos{Line: 3, Col: 4}}
	if got, want := e.Error(), "SYNTAX_ERROR at 3:4: boom"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestErrorCodeValues(t *testing.T) {
	t.Parallel()
	for code, want := range map[Code]string{
		CodeSyntaxError:           "SYNTAX_ERROR",
		CodeUnsupportedOperator:   "UNSUPPORTED_OPERATOR",
		CodeUnsupportedExpression: "UNSUPPORTED_EXPRESSION",
		CodeInvalidRelationScope:  "INVALID_RELATION_SCOPE",
		CodeComplexityLimit:       "COMPLEXITY_LIMIT",
		CodeUnknownField:          "UNKNOWN_FIELD",
		CodeUnknownRelation:       "UNKNOWN_RELATION",
		CodeUnknownValue:          "UNKNOWN_VALUE",
		CodeTypeMismatch:          "TYPE_MISMATCH",
	} {
		if string(code) != want {
			t.Errorf("code %q, want %q", code, want)
		}
	}
}

func TestErrorNear(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"token text", "a === 1 && b ~= 'x'", "~="},
		{"end of input shows the tail", "a === 1 &&", "a === 1 &&"},
		{"empty source", "", ""},
		{"line breaks are flattened", "a === 1 &&\n  (b ===", "a === 1 && (b ==="},
		{"long span is truncated", "x === " + strings.Repeat("y", 100), "yyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyyy…"},
		{"unterminated string", "x === 'abc", "'abc"},
		{"long tail at end of input is shortened", "a === 1 && " + strings.Repeat("(", 30), strings.Repeat("(", 20)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := parseErr(t, tt.src)
			if e.Near != tt.want {
				t.Errorf("near = %q, want %q (error: %v)", e.Near, tt.want, e)
			}
		})
	}
}

func TestNearExcerptEdges(t *testing.T) {
	t.Parallel()
	if got := nearExcerpt("abc", Pos{Offset: -1}, Pos{Offset: 2}); got != "" {
		t.Errorf("negative start: %q", got)
	}
	if got := nearExcerpt("abc", Pos{Offset: 9}, Pos{Offset: 9}); got != "" {
		t.Errorf("start past the end: %q", got)
	}
	if got := nearExcerpt("abc", Pos{Offset: 1}, Pos{Offset: 99}); got != "bc" {
		t.Errorf("end past the end: %q", got)
	}
	// A tail that starts inside a multi-byte rune is moved to the next boundary.
	src := strings.Repeat("İ", 15)
	got := nearExcerpt(src, Pos{Offset: len(src)}, Pos{Offset: len(src)})
	if !validUTF8(got) || got == "" {
		t.Errorf("tail = %q", got)
	}
}

// TestNewErrorAndSuggestions: the semantic pass builds errors like the parser does, and
// Suggestions stay out of the formatted text (D-066).
func TestNewErrorAndSuggestions(t *testing.T) {
	t.Parallel()
	src := "cty === 1"
	start := Pos{Offset: 0, Line: 1, Col: 1}
	end := Pos{Offset: 3, Line: 1, Col: 4}
	e := NewError(src, CodeUnknownField, `unknown field "cty"`, start, end)
	e.Suggestions = []string{"city"}
	if e.Code != CodeUnknownField || e.Near != "cty" || e.Start != start || e.End != end {
		t.Errorf("NewError = %+v", e)
	}
	if want := `UNKNOWN_FIELD at 1:1: unknown field "cty" near "cty"`; e.Error() != want {
		t.Errorf("Error() = %q, want %q", e.Error(), want)
	}
}
