package dsl

import (
	"strconv"
	"strings"
)

// Print renders e in the canonical form (D-065): double-quoted strings
// (escaping \\ and \"), single spaces around binary operators, no space after
// `!`, minimal parentheses by precedence, `[a, b]` lists, and relations as
// `orders.some(o => body)`. Parse(Print(e)) is Equal to e and Print is idempotent.
func Print(e Expr) string {
	var b strings.Builder
	printExpr(&b, e, precLowest)
	return b.String()
}

const (
	precLowest = iota
	precOr
	precAnd
	precUnary
)

func printExpr(b *strings.Builder, e Expr, parent int) {
	switch x := e.(type) {
	case *Or:
		printJoined(b, x.Terms, " || ", precOr, parent)
	case *And:
		printJoined(b, x.Terms, " && ", precAnd, parent)
	case *Not:
		b.WriteByte('!')
		switch x.X.(type) {
		case *Not, *Relation:
			printExpr(b, x.X, precUnary)
		default:
			// ! binds tighter than comparisons and &&, so a comparison,
			// in-test or logical group needs parentheses.
			b.WriteByte('(')
			printExpr(b, x.X, precLowest)
			b.WriteByte(')')
		}
	case *Compare:
		printPath(b, x.Field)
		b.WriteByte(' ')
		b.WriteString(string(x.Op))
		b.WriteByte(' ')
		b.WriteString(printLiteral(x.Value))
	case *In:
		printPath(b, x.Field)
		b.WriteString(" in [")
		for i, v := range x.Values {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(printLiteral(v))
		}
		b.WriteByte(']')
	case *Relation:
		printPath(b, x.Path)
		b.WriteByte('.')
		b.WriteString(string(x.Quantifier))
		b.WriteByte('(')
		b.WriteString(x.Var)
		b.WriteString(" => ")
		printExpr(b, x.Body, precLowest)
		b.WriteByte(')')
	}
}

func printJoined(b *strings.Builder, terms []Expr, sep string, prec, parent int) {
	paren := parent > prec
	if paren {
		b.WriteByte('(')
	}
	for i, t := range terms {
		if i > 0 {
			b.WriteString(sep)
		}
		printExpr(b, t, prec+1)
	}
	if paren {
		b.WriteByte(')')
	}
}

func printPath(b *strings.Builder, p Path) {
	b.WriteString(strings.Join(p.Segments, "."))
}

func printLiteral(l Literal) string {
	switch x := l.(type) {
	case String:
		return quote(x.Value)
	case Number:
		return x.Text
	case Bool:
		return strconv.FormatBool(x.Value)
	case Ago:
		return "ago(" + quote(x.Text) + ")"
	case Date:
		return "date(" + quote(x.Text) + ")"
	}
	return ""
}

// quote writes a double-quoted DSL string; only \\ and \" are escaped.
func quote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		if r == '\\' || r == '"' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}
