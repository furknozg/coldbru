// Package dsl parses the Coldbru audience-filter DSL (docs/coldbru-gqdsl.md)
// into a typed AST. It is a hand-written lexer plus a Pratt parser (D-038);
// name resolution and type checking are a separate pass (T-019).
//
// The parser stops at the first error and reports it with a code, a message
// that tells the LLM how to fix the expression, a line/rune-column position and
// a short excerpt (D-065). It uses only the standard library.
package dsl

import (
	"fmt"
	"strings"
	"time"
)

// Version is the DSL version implemented here (D-045). Prompts declare
// requires_dsl against it (D-059).
const Version = "0.1"

// Limits (D-065). They mirror the Customer service's limits (D-062), which
// has the final word; keep both in sync.
//
// Counting, identical to the Customer filter compiler (D-062, "Uygulama
// notu"): the root expression is depth 1. Each And/Or term, each Not operand
// and each relation body is one level deeper than its parent. Every Compare and
// In counts as one node (an In counts 1 whatever its list length), every
// And/Or/Not counts as one node, and every relation quantifier counts as one
// node. Parentheses are not nodes and add no depth.
const (
	MaxSourceBytes = 4096
	MaxDepth       = 8
	MaxNodes       = 100
	MaxInValues    = 100

	// MaxAgoDays and MaxAgoHours bound ago("..."): 100 years (D-062, D-058).
	MaxAgoDays  = 36500
	MaxAgoHours = 876000
)

// Parse parses src into an AST. On failure it returns a nil Expr and an
// *Error (use errors.As to read the code and position).
func Parse(src string) (Expr, error) {
	if len(src) > MaxSourceBytes {
		start := Pos{Offset: 0, Line: 1, Col: 1}
		return nil, newError(src, CodeComplexityLimit,
			fmt.Sprintf("expression is %d bytes; the maximum is %d. Shorten it, for example with fewer conditions or shorter lists", len(src), MaxSourceBytes),
			start, start)
	}
	p := &parser{src: src, toks: lex(src)}
	e, perr := p.parseExpr(bpOr)
	if perr != nil {
		return nil, perr
	}
	if t := p.cur(); t.kind == tkRParen {
		return nil, p.at(t, CodeSyntaxError, "unmatched `)`: there is no `(` to close; remove it")
	}
	if p.cur().kind != tkEOF {
		return nil, p.unexpected("`&&`, `||` or the end of the expression")
	}
	if lerr := p.checkLimits(e); lerr != nil {
		return nil, lerr
	}
	return e, nil
}

// Binding powers: higher binds tighter. Comparisons, in and ! are handled in
// parseUnary because they only apply to the operands they syntactically own.
const (
	bpOr  = 1
	bpAnd = 2
)

type parser struct {
	src    string
	toks   []token
	i      int
	scopes []string // enclosing lambda variables, innermost last
}

func (p *parser) cur() token { return p.toks[p.i] }

func (p *parser) peek(n int) token {
	return p.toks[min(p.i+n, len(p.toks)-1)]
}

func (p *parser) advance() token {
	t := p.toks[p.i]
	if p.i < len(p.toks)-1 {
		p.i++
	}
	return t
}

// at builds an error on a token; a lexical error token reports itself.
func (p *parser) at(t token, code Code, format string, args ...any) *Error {
	if t.kind == tkError {
		return t.err
	}
	return newError(p.src, code, fmt.Sprintf(format, args...), t.start, t.end)
}

func (p *parser) span(code Code, start, end Pos, format string, args ...any) *Error {
	return newError(p.src, code, fmt.Sprintf(format, args...), start, end)
}

func describe(t token) string {
	switch t.kind {
	case tkEOF:
		return "the end of the expression"
	case tkIdent:
		return "identifier `" + t.text + "`"
	case tkString:
		return "a string"
	case tkNumber:
		return "number `" + t.text + "`"
	}
	return "`" + t.text + "`"
}

func isComparison(k tokKind) bool {
	switch k {
	case tkEq, tkNe, tkGt, tkGte, tkLt, tkLte:
		return true
	}
	return false
}

func isLiteralStart(k tokKind) bool {
	switch k {
	case tkString, tkNumber, tkTrue, tkFalse, tkAgo, tkDate:
		return true
	}
	return false
}

func quantifierOf(k tokKind) (Quantifier, bool) {
	switch k {
	case tkSome:
		return Some, true
	case tkNone:
		return None, true
	case tkEvery:
		return Every, true
	}
	return "", false
}

// unexpected reports the current token where `expected` was required.
func (p *parser) unexpected(expected string) *Error {
	t := p.cur()
	switch {
	case t.kind == tkEOF:
		return p.at(t, CodeSyntaxError, "unexpected end of the expression; expected %s", expected)
	case isComparison(t.kind) || t.kind == tkIn:
		return p.at(t, CodeSyntaxError, "unexpected %s: a comparison needs a field on its left, and comparisons cannot be chained or applied to a group, `!` or a relation; expected %s", describe(t), expected)
	}
	return p.at(t, CodeSyntaxError, "unexpected %s; expected %s", describe(t), expected)
}

func (p *parser) expect(kind tokKind, what string) (token, *Error) {
	if p.cur().kind != kind {
		return token{}, p.unexpected(what)
	}
	return p.advance(), nil
}

// parseExpr is the Pratt loop for || (bpOr) and && (bpAnd); both are left
// associative and chains are flattened into one node.
func (p *parser) parseExpr(minBP int) (Expr, *Error) {
	left, err := p.parseUnary(nil)
	if err != nil {
		return nil, err
	}
	for {
		var bp int
		isAnd := false
		switch p.cur().kind {
		case tkOr:
			bp = bpOr
		case tkAnd:
			bp, isAnd = bpAnd, true
		default:
			return left, nil
		}
		if bp < minBP {
			return left, nil
		}
		p.advance()
		right, err := p.parseExpr(bp + 1)
		if err != nil {
			return nil, err
		}
		terms := collect(isAnd, left, right)
		if isAnd {
			left = &And{Span: spanOf(terms), Terms: terms}
		} else {
			left = &Or{Span: spanOf(terms), Terms: terms}
		}
	}
}

// collect returns the operands of an And (isAnd) or Or, splicing in the terms
// of a same-kind child so that a && b && c, also written with parentheses, is
// one node with three terms.
func collect(isAnd bool, operands ...Expr) []Expr {
	var terms []Expr
	for _, e := range operands {
		switch x := e.(type) {
		case *And:
			if isAnd {
				terms = append(terms, x.Terms...)
				continue
			}
		case *Or:
			if !isAnd {
				terms = append(terms, x.Terms...)
				continue
			}
		}
		terms = append(terms, e)
	}
	return terms
}

func spanOf(terms []Expr) Span {
	return Span{Start: terms[0].Extent().Start, End: terms[len(terms)-1].Extent().End}
}

// parseUnary parses a parenthesized group, a ! expression, a relation, a
// comparison or an in-test. neg is the `!` token when the caller is parsing
// the operand of a !, for better messages.
func (p *parser) parseUnary(neg *token) (Expr, *Error) {
	t := p.cur()
	switch {
	case t.kind == tkLParen:
		p.advance()
		e, err := p.parseExpr(bpOr)
		if err != nil {
			return nil, err
		}
		rp, err := p.expect(tkRParen, "`)` to close the group")
		if err != nil {
			return nil, err
		}
		e.setSpan(Span{Start: t.start, End: rp.end})
		return e, nil
	case t.kind == tkNot:
		p.advance()
		x, err := p.parseUnary(&t)
		if err != nil {
			return nil, err
		}
		return &Not{Span: Span{Start: t.start, End: x.Extent().End}, X: x}, nil
	case t.kind == tkIdent:
		return p.parsePathLed(neg)
	case isLiteralStart(t.kind):
		return nil, p.leadingLiteral()
	case t.kind == tkSome || t.kind == tkNone || t.kind == tkEvery || t.kind == tkIn:
		return nil, p.at(t, CodeSyntaxError, "`%s` is a reserved word and cannot be used as a field name; a condition starts with a field, `!` or `(`", t.text)
	case t.kind == tkAssign:
		return nil, p.at(t, CodeUnsupportedExpression, "assignment is not supported; the DSL only filters. Use === to compare")
	}
	return nil, p.unexpected("a condition (a field comparison, `!`, a relation or a parenthesized group)")
}

// leadingLiteral reports a literal where a field was expected (D-065).
func (p *parser) leadingLiteral() *Error {
	start := p.cur()
	lit, err := p.parseLiteral()
	if err != nil {
		return err
	}
	next := p.cur()
	if isComparison(next.kind) {
		hint := "put the field on the left, for example `field " + string(flipOp(Op(next.text))) + " " + printLiteral(lit) + "`"
		if path, ok := p.simplePathAfterOp(); ok {
			hint = "put the field on the left: write `" + path + " " + string(flipOp(Op(next.text))) + " " + printLiteral(lit) + "`"
		}
		return p.span(CodeSyntaxError, start.start, lit.Extent().End, "a literal cannot be on the left of a comparison; %s", hint)
	}
	if next.kind == tkIn {
		return p.span(CodeSyntaxError, start.start, lit.Extent().End, "a literal cannot be on the left of `in`; put the field on the left, for example `field in [%s]`", printLiteral(lit))
	}
	return p.span(CodeSyntaxError, start.start, lit.Extent().End, "a literal is not a condition; write a comparison such as `field === %s`", printLiteral(lit))
}

// simplePathAfterOp returns the dotted path written after the comparison
// operator at the cursor, if it is one plain path.
func (p *parser) simplePathAfterOp() (string, bool) {
	var segs []string
	for n := 1; ; n += 2 {
		t := p.peek(n)
		if t.kind != tkIdent {
			return "", false
		}
		segs = append(segs, t.text)
		switch p.peek(n + 1).kind {
		case tkDot:
			continue
		case tkLParen:
			return "", false
		}
		return strings.Join(segs, "."), true
	}
}

func flipOp(op Op) Op {
	switch op {
	case OpGt:
		return OpLt
	case OpGte:
		return OpLte
	case OpLt:
		return OpGt
	case OpLte:
		return OpGte
	}
	return op
}

// readPath reads ident(.ident)*. It stops before `.some(`, `.none(` or
// `.every(` and rejects every other call or method (D-065).
func (p *parser) readPath() (Path, *Error) {
	first := p.advance()
	path := Path{Span: Span{Start: first.start, End: first.end}, Segments: []string{first.text}}
	for p.cur().kind == tkDot {
		next := p.peek(1)
		if _, isQuant := quantifierOf(next.kind); isQuant {
			if p.peek(2).kind == tkLParen {
				break
			}
			return path, p.at(next, CodeSyntaxError, "`.%s` must be called as .%s(v => condition), for example `orders.some(o => o.totalGbp > 100)`", next.text, next.text)
		}
		if next.kind != tkIdent {
			if _, reserved := keywords[next.text]; reserved {
				return path, p.at(next, CodeSyntaxError, "`%s` is a reserved word and cannot be used as a field name", next.text)
			}
			return path, p.at(next, CodeSyntaxError, "expected a field name after `.`, found %s", describe(next))
		}
		p.advance()
		p.advance()
		path.Segments = append(path.Segments, next.text)
		path.End = next.end
	}
	if p.cur().kind == tkLParen {
		name := path.Segments[len(path.Segments)-1]
		what := "function call `" + name + "(...)`"
		if len(path.Segments) > 1 {
			what = "method call `." + name + "(...)`"
		}
		return path, p.span(CodeUnsupportedExpression, path.Start, p.cur().end,
			"%s is not supported; the only calls are ago(\"30d\"), date(\"2026-01-01\") and the relation quantifiers .some/.none/.every(v => condition)", what)
	}
	return path, nil
}

// checkScope enforces D-065 lambda scoping on a path found inside a body.
func (p *parser) checkScope(path Path) *Error {
	if len(p.scopes) == 0 {
		return nil
	}
	inner := p.scopes[len(p.scopes)-1]
	first := path.Segments[0]
	if first != inner {
		for _, outer := range p.scopes[:len(p.scopes)-1] {
			if first == outer {
				return p.span(CodeInvalidRelationScope, path.Start, path.End,
					"`%s` is the variable of an enclosing relation; inside this body only `%s` is visible. Move the condition out of the inner lambda", first, inner)
			}
		}
		return p.span(CodeInvalidRelationScope, path.Start, path.End,
			"inside `%s => ...` every field must be qualified with the variable: write `%s.%s`", inner, inner, strings.Join(path.Segments, "."))
	}
	if len(path.Segments) == 1 {
		return p.span(CodeInvalidRelationScope, path.Start, path.End,
			"`%s` is a relation variable, not a field; write `%s.<field>`", inner, inner)
	}
	return nil
}

func (p *parser) parsePathLed(neg *token) (Expr, *Error) {
	path, err := p.readPath()
	if err != nil {
		return nil, err
	}
	if err := p.checkScope(path); err != nil {
		return nil, err
	}
	t := p.cur()
	switch {
	case t.kind == tkDot:
		return p.parseRelation(path)
	case neg != nil && (isComparison(t.kind) || t.kind == tkIn):
		return nil, p.span(CodeSyntaxError, neg.start, t.end,
			"`!` binds tighter than a comparison, so it cannot negate `%s %s ...`; wrap the comparison in parentheses: !(%s %s ...)",
			strings.Join(path.Segments, "."), t.text, strings.Join(path.Segments, "."), t.text)
	case isComparison(t.kind):
		p.advance()
		val, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		return &Compare{Span: Span{Start: path.Start, End: val.Extent().End}, Field: path, Op: Op(t.text), Value: val}, nil
	case t.kind == tkIn:
		return p.parseIn(path)
	case t.kind == tkTilde:
		return nil, p.at(t, CodeUnsupportedOperator, "`~=` (text search) is reserved and not available; use === or `in [...]` for exact matches")
	case t.kind == tkLooseEq:
		return nil, p.at(t, CodeUnsupportedOperator, "`==` is not an operator of this language; use === for equality")
	case t.kind == tkLooseNe:
		return nil, p.at(t, CodeUnsupportedOperator, "`!=` is not an operator of this language; use !== for inequality")
	case t.kind == tkAssign:
		return nil, p.at(t, CodeUnsupportedExpression, "assignment is not supported; the DSL only filters. Use === to compare")
	case t.kind == tkArrow:
		return nil, p.at(t, CodeUnsupportedExpression, "arrow functions are only allowed inside .some(), .none() and .every(), for example orders.some(o => o.totalGbp > 100)")
	}
	name := strings.Join(path.Segments, ".")
	switch t.kind {
	case tkEOF, tkAnd, tkOr, tkRParen:
		if neg != nil {
			return nil, p.span(CodeSyntaxError, neg.start, path.End,
				"`!%s` is not supported: a field alone is not a condition. Compare it explicitly: `%s === false`", name, name)
		}
		return nil, p.span(CodeSyntaxError, path.Start, path.End,
			"`%s` alone is not a condition (bare boolean fields are not supported). Compare it explicitly: `%s === true`", name, name)
	}
	return nil, p.unexpected("a comparison operator (===, !==, >, >=, <, <=) or `in` after `" + name + "`")
}

// parseRelation parses `.quantifier(var => body)` after path.
func (p *parser) parseRelation(path Path) (Expr, *Error) {
	p.advance() // .
	qTok := p.advance()
	quant, _ := quantifierOf(qTok.kind)
	p.advance() // (
	v := p.cur()
	switch v.kind {
	case tkIdent:
	case tkLParen:
		return nil, p.at(v, CodeSyntaxError, "write the lambda variable without parentheses: %s(v => condition)", qTok.text)
	case tkRParen:
		return nil, p.at(v, CodeSyntaxError, ".%s() needs a lambda: %s(v => condition)", qTok.text, qTok.text)
	case tkError:
		return nil, v.err
	default:
		if _, reserved := keywords[v.text]; reserved {
			return nil, p.at(v, CodeSyntaxError, "`%s` is a reserved word and cannot be a lambda variable; pick a short name such as `o` or `c`", v.text)
		}
		return nil, p.at(v, CodeSyntaxError, "expected a lambda variable such as `o` after .%s(, found %s", qTok.text, describe(v))
	}
	for _, outer := range p.scopes {
		if outer == v.text {
			return nil, p.at(v, CodeInvalidRelationScope, "lambda variable `%s` is already used by an enclosing relation; choose a different name for the nested one", v.text)
		}
	}
	p.advance()
	switch a := p.cur(); a.kind {
	case tkArrow:
		p.advance()
	case tkComma:
		return nil, p.at(a, CodeSyntaxError, "a relation lambda takes exactly one variable: %s(%s => condition)", qTok.text, v.text)
	default:
		return nil, p.unexpected(fmt.Sprintf("`=>` after the lambda variable `%s`", v.text))
	}
	p.scopes = append(p.scopes, v.text)
	body, err := p.parseExpr(bpOr)
	p.scopes = p.scopes[:len(p.scopes)-1]
	if err != nil {
		return nil, err
	}
	rp, err := p.expect(tkRParen, "`)` to close ."+qTok.text+"(")
	if err != nil {
		return nil, err
	}
	return &Relation{
		Span:       Span{Start: path.Start, End: rp.end},
		Path:       path,
		Quantifier: quant,
		Var:        v.text,
		VarSpan:    Span{Start: v.start, End: v.end},
		Body:       body,
	}, nil
}

// parseValue parses the right side of a comparison: a literal only (D-045).
func (p *parser) parseValue() (Literal, *Error) {
	t := p.cur()
	switch {
	case isLiteralStart(t.kind):
		lit, err := p.parseLiteral()
		if err != nil {
			return nil, err
		}
		if p.cur().kind == tkDot {
			return nil, p.at(p.cur(), CodeUnsupportedExpression, "methods on values are not supported; write the literal alone")
		}
		return lit, nil
	case t.kind == tkLBracket:
		return nil, p.at(t, CodeSyntaxError, "a list literal is only allowed after `in`: field in [a, b]; comparisons take a single literal")
	case t.kind == tkIdent:
		path, err := p.readPath()
		if err != nil {
			return nil, err
		}
		if p.cur().kind == tkDot {
			return nil, p.span(CodeUnsupportedExpression, path.Start, p.cur().end, "a relation predicate cannot be used as a value; use it as a condition on its own")
		}
		name := strings.Join(path.Segments, ".")
		switch name {
		case "null", "undefined", "NaN":
			return nil, p.span(CodeSyntaxError, path.Start, path.End, "there is no `%s` literal in this language; compare with a string, number, true/false, ago(...) or date(...)", name)
		}
		return nil, p.span(CodeUnsupportedOperator, path.Start, path.End,
			"the right side of a comparison must be a literal, but `%s` is a field; comparing two fields is reserved. Use a literal value", name)
	}
	return nil, p.unexpected("a literal value (string, number, true, false, ago(\"30d\") or date(\"2026-01-01\"))")
}

// parseLiteral parses one literal at the cursor (isLiteralStart holds).
func (p *parser) parseLiteral() (Literal, *Error) {
	t := p.cur()
	sp := Span{Start: t.start, End: t.end}
	switch t.kind {
	case tkString:
		p.advance()
		return String{Span: sp, Value: t.val}, nil
	case tkNumber:
		p.advance()
		return Number{Span: sp, Text: t.text}, nil
	case tkTrue, tkFalse:
		p.advance()
		return Bool{Span: sp, Value: t.kind == tkTrue}, nil
	case tkAgo, tkDate:
		return p.parseTimeCall()
	}
	return nil, p.unexpected("a literal value")
}

// parseTimeCall parses ago("30d") or date("2026-01-01") and validates the
// argument (D-039, D-058, D-062).
func (p *parser) parseTimeCall() (Literal, *Error) {
	kw := p.advance()
	example := `ago("30d")`
	if kw.kind == tkDate {
		example = `date("2026-01-01")`
	}
	if p.cur().kind != tkLParen {
		return nil, p.at(kw, CodeSyntaxError, "`%s` must be called with one string argument, for example %s", kw.text, example)
	}
	p.advance()
	arg := p.cur()
	if arg.kind != tkString {
		return nil, p.at(arg, CodeSyntaxError, "`%s()` takes exactly one string argument, for example %s", kw.text, example)
	}
	if msg := checkTimeArg(kw.kind, arg.val); msg != "" {
		return nil, p.at(arg, CodeSyntaxError, "%s", msg)
	}
	p.advance()
	if p.cur().kind == tkComma {
		return nil, p.at(p.cur(), CodeSyntaxError, "`%s()` takes exactly one argument, for example %s", kw.text, example)
	}
	rp, err := p.expect(tkRParen, "`)` to close "+kw.text+"(")
	if err != nil {
		return nil, err
	}
	sp := Span{Start: kw.start, End: rp.end}
	if kw.kind == tkAgo {
		return Ago{Span: sp, Text: arg.val}, nil
	}
	return Date{Span: sp, Text: arg.val}, nil
}

// checkTimeArg returns "" when the argument is valid, or the error message.
func checkTimeArg(kind tokKind, arg string) string {
	if kind == tkAgo {
		return checkAgo(arg)
	}
	return checkDate(arg)
}

// checkAgo accepts ^[1-9][0-9]*(d|h)$ up to 36500d / 876000h.
func checkAgo(s string) string {
	digits, unit := "", byte(0)
	if len(s) > 0 {
		digits, unit = s[:len(s)-1], s[len(s)-1]
	}
	valid := digits != "" && digits[0] >= '1' && digits[0] <= '9' && (unit == 'd' || unit == 'h')
	for i := 0; valid && i < len(digits); i++ {
		valid = digits[i] >= '0' && digits[i] <= '9'
	}
	if !valid {
		return fmt.Sprintf("ago(%q) is invalid: use a positive whole number followed by d (days) or h (hours) with no spaces, for example \"30d\" or \"12h\"", s)
	}
	limit := MaxAgoDays
	if unit == 'h' {
		limit = MaxAgoHours
	}
	n := 0
	for i := 0; i < len(digits); i++ {
		n = n*10 + int(digits[i]-'0')
		if n > limit {
			return fmt.Sprintf("ago(%q) reaches too far back: the maximum is %dd or %dh", s, MaxAgoDays, MaxAgoHours)
		}
	}
	return ""
}

// checkDate accepts a valid YYYY-MM-DD calendar date.
func checkDate(s string) string {
	bad := fmt.Sprintf("date(%q) is invalid: use a real calendar date as YYYY-MM-DD, for example \"2026-01-31\"", s)
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return bad
	}
	for i := range len(s) {
		if i != 4 && i != 7 && (s[i] < '0' || s[i] > '9') {
			return bad
		}
	}
	y := int(s[0]-'0')*1000 + int(s[1]-'0')*100 + int(s[2]-'0')*10 + int(s[3]-'0')
	m := int(s[5]-'0')*10 + int(s[6]-'0')
	d := int(s[8]-'0')*10 + int(s[9]-'0')
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	if m < 1 || m > 12 || d < 1 || t.Year() != y || int(t.Month()) != m || t.Day() != d {
		return bad
	}
	return ""
}

// parseIn parses `in [literal, ...]` after path (D-046, D-062).
func (p *parser) parseIn(path Path) (Expr, *Error) {
	p.advance() // in
	lb := p.cur()
	if lb.kind != tkLBracket {
		return nil, p.unexpected("a list after `in`, for example " + strings.Join(path.Segments, ".") + ` in ["a", "b"]`)
	}
	p.advance()
	if rb := p.cur(); rb.kind == tkRBracket {
		return nil, p.span(CodeSyntaxError, lb.start, rb.end, "`in []` is empty; an in-list needs at least one literal. An empty list would silently match nobody")
	}
	var values []Literal
	for {
		t := p.cur()
		switch {
		case isLiteralStart(t.kind):
		case t.kind == tkLBracket:
			return nil, p.at(t, CodeSyntaxError, "nested lists are not supported; the list holds literals only")
		case t.kind == tkIdent:
			return nil, p.at(t, CodeSyntaxError, "list elements must be literals (string, number, true, false, ago(...), date(...)), but `%s` is a field", t.text)
		default:
			return nil, p.unexpected("a literal list element")
		}
		lit, err := p.parseLiteral()
		if err != nil {
			return nil, err
		}
		values = append(values, lit)
		if len(values) > MaxInValues {
			return nil, p.span(CodeComplexityLimit, lit.Extent().Start, lit.Extent().End,
				"in-list has more than %d values (the maximum is %d); use fewer values or a broader condition", MaxInValues, MaxInValues)
		}
		switch p.cur().kind {
		case tkComma:
			p.advance()
			if p.cur().kind == tkRBracket {
				return nil, p.at(p.cur(), CodeSyntaxError, "trailing comma: remove the comma before `]`")
			}
		case tkRBracket:
			rb := p.advance()
			return &In{Span: Span{Start: path.Start, End: rb.end}, Field: path, Values: values}, nil
		default:
			return nil, p.unexpected("`,` or `]` in the list")
		}
	}
}

// checkLimits walks the tree once and reports the first node that breaks the
// depth or node limit, counted as documented on MaxDepth (D-062, D-065).
func (p *parser) checkLimits(e Expr) *Error {
	nodes := 0
	var walk func(e Expr, depth int) *Error
	walk = func(e Expr, depth int) *Error {
		nodes++
		sp := e.Extent()
		if depth > MaxDepth {
			return p.span(CodeComplexityLimit, sp.Start, sp.End,
				"expression nesting is deeper than %d levels; flatten the conditions or split the rule", MaxDepth)
		}
		if nodes > MaxNodes {
			return p.span(CodeComplexityLimit, sp.Start, sp.End,
				"expression has more than %d condition nodes; simplify the rule", MaxNodes)
		}
		var children []Expr
		switch x := e.(type) {
		case *And:
			children = x.Terms
		case *Or:
			children = x.Terms
		case *Not:
			children = []Expr{x.X}
		case *Relation:
			children = []Expr{x.Body}
		}
		for _, c := range children {
			if err := walk(c, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(e, 1)
}
