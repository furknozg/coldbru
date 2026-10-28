package dsl

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func fmtPos(p Pos) string { return fmt.Sprintf("%d:%d", p.Line, p.Col) }

func fmtPosOff(p Pos) string { return fmt.Sprintf("%d:%d/%d", p.Line, p.Col, p.Offset) }

func contains(s, sub string) bool { return strings.Contains(s, sub) }

// mustParse parses src or fails the test.
func mustParse(t testing.TB, src string) Expr {
	t.Helper()
	e, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q) failed: %v", src, err)
	}
	return e
}

// parseErr parses src and returns the *Error, failing the test if Parse
// succeeds or returns another error type.
func parseErr(t testing.TB, src string) *Error {
	t.Helper()
	e, err := Parse(src)
	if err == nil {
		t.Fatalf("Parse(%q) = %s, want an error", src, Print(e))
	}
	var pe *Error
	if !errors.As(err, &pe) {
		t.Fatalf("Parse(%q) error is %T, want *Error", src, err)
	}
	return pe
}

// posAt recomputes the position of a byte offset from scratch, so the tests
// can verify the incremental line/column tracking of the lexer.
func posAt(src string, off int) Pos {
	line, col := 1, 1
	for i, r := range src {
		if i >= off {
			break
		}
		if r == '\n' {
			line++
			col = 1
			continue
		}
		col++
	}
	return Pos{Offset: off, Line: line, Col: col}
}

// checkSpans verifies, for every node, that its span lies in the source, that
// its positions are consistent with its offsets and that it contains the
// spans of its children.
func checkSpans(t testing.TB, src string, e Expr) {
	t.Helper()
	var visit func(e Expr, parent Span)
	inside := func(sp, outer Span) bool {
		return sp.Start.Offset >= outer.Start.Offset && sp.End.Offset <= outer.End.Offset && sp.Start.Offset <= sp.End.Offset
	}
	check := func(what string, sp, parent Span) {
		t.Helper()
		if sp.Start.Offset < 0 || sp.End.Offset > len(src) || !inside(sp, parent) {
			t.Errorf("%s span %v-%v outside its parent %v-%v (source %q)", what, sp.Start, sp.End, parent.Start, parent.End, src)
			return
		}
		if sp.Start != posAt(src, sp.Start.Offset) || sp.End != posAt(src, sp.End.Offset) {
			t.Errorf("%s span %v-%v has positions inconsistent with its offsets (source %q)", what, sp.Start, sp.End, src)
		}
	}
	whole := Span{Start: Pos{Line: 1, Col: 1}, End: posAt(src, len(src))}
	visit = func(e Expr, parent Span) {
		sp := e.Extent()
		check(fmt.Sprintf("%T", e), sp, parent)
		switch x := e.(type) {
		case *And:
			for _, c := range x.Terms {
				visit(c, sp)
			}
		case *Or:
			for _, c := range x.Terms {
				visit(c, sp)
			}
		case *Not:
			visit(x.X, sp)
		case *Compare:
			check("Compare.Field", x.Field.Extent(), sp)
			check("Compare.Value", x.Value.Extent(), sp)
		case *In:
			check("In.Field", x.Field.Extent(), sp)
			for _, v := range x.Values {
				check("In.Value", v.Extent(), sp)
			}
		case *Relation:
			check("Relation.Path", x.Path.Extent(), sp)
			check("Relation.VarSpan", x.VarSpan, sp)
			visit(x.Body, sp)
		}
	}
	visit(e, whole)
}

func validUTF8(s string) bool { return utf8.ValidString(s) }
