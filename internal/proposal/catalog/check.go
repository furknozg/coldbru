package catalog

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/agnivade/levenshtein"

	"github.com/furknozg/coldbru/internal/proposal/dsl"
)

// MaxErrors caps the errors Check returns, so a badly broken expression does not flood the LLM
// repair prompt (D-066).
const MaxErrors = 10

const (
	maxSuggestions = 3
	maxEditDist    = 2
	// listFieldsMax is the largest entity whose field names are listed in an unknown-field message.
	listFieldsMax = 12
)

// decimalText is Customer's Decimal pattern (D-058): at most two fraction digits.
var decimalText = regexp.MustCompile(`^-?[0-9]+(\.[0-9]{1,2})?$`)

// Check is the semantic pass of the DSL (D-066): it resolves every name against Root and checks
// operators and literals against the field types and closed lists. Unlike the parser it does not
// stop at the first problem: it returns every error in source order, at most MaxErrors, and nil
// when the expression is valid. e must come from dsl.Parse(src); the errors carry positions in src.
func Check(src string, e dsl.Expr) []*dsl.Error {
	return checkAgainst(Root, src, e)
}

// checkAgainst is Check over an arbitrary root, so tests can use a small catalog.
func checkAgainst(root *Entity, src string, e dsl.Expr) []*dsl.Error {
	c := &checker{src: src}
	c.expr(scope{entity: root}, e)
	return c.errs
}

// scope is the entity a path resolves against. Inside a relation lambda variable is the lambda
// variable, which the parser guarantees prefixes every path (D-065); at the root it is empty.
type scope struct {
	entity   *Entity
	variable string
}

type checker struct {
	src  string
	errs []*dsl.Error
}

func (c *checker) add(code dsl.Code, msg string, start, end dsl.Pos, suggestions []string) {
	if len(c.errs) >= MaxErrors {
		return
	}
	err := dsl.NewError(c.src, code, msg, start, end)
	err.Suggestions = suggestions
	c.errs = append(c.errs, err)
}

func (c *checker) expr(s scope, e dsl.Expr) {
	switch x := e.(type) {
	case *dsl.And:
		if x != nil {
			for _, t := range x.Terms {
				c.expr(s, t)
			}
		}
	case *dsl.Or:
		if x != nil {
			for _, t := range x.Terms {
				c.expr(s, t)
			}
		}
	case *dsl.Not:
		if x != nil {
			c.expr(s, x.X)
		}
	case *dsl.Compare:
		if x != nil {
			c.compare(s, x)
		}
	case *dsl.In:
		if x != nil {
			c.in(s, x)
		}
	case *dsl.Relation:
		if x != nil {
			c.relation(s, x)
		}
	}
}

func (c *checker) compare(s scope, x *dsl.Compare) {
	f := c.field(s, x.Field)
	if f == nil {
		return
	}
	if !f.Allows(x.Op) {
		if start, end, ok := c.locate(x.Field.End, x.Value.Extent().Start.Offset, string(x.Op)); ok {
			c.opError(f, x.Op, start, end)
		} else {
			c.opError(f, x.Op, x.Start, x.End)
		}
		return
	}
	c.value(f, x.Op, x.Value)
}

func (c *checker) in(s scope, x *dsl.In) {
	f := c.field(s, x.Field)
	if f == nil {
		return
	}
	if !f.Allows(OpIn) {
		start, end := x.Start, x.End
		if len(x.Values) > 0 {
			if st, en, ok := c.locate(x.Field.End, x.Values[0].Extent().Start.Offset, "in"); ok {
				start, end = st, en
			}
		}
		c.opError(f, OpIn, start, end)
		return
	}
	for _, v := range x.Values {
		c.value(f, OpIn, v)
	}
}

func (c *checker) opError(f *Field, op dsl.Op, start, end dsl.Pos) {
	allowed := make([]string, len(f.Ops))
	for i, o := range f.Ops {
		allowed[i] = string(o)
	}
	c.add(dsl.CodeTypeMismatch,
		fmt.Sprintf("operator %s cannot be used on %s (%s); allowed operators: %s", op, f.Name, f.Kind, strings.Join(allowed, ", ")),
		start, end, nil)
}

// segments returns the path without the lambda variable, or false when the path does not start
// with it (the parser prevents that; the check keeps Check safe on hand-built trees).
func (s scope) segments(p dsl.Path) ([]string, bool) {
	segs := p.Segments
	if s.variable == "" {
		return segs, true
	}
	if len(segs) == 0 || segs[0] != s.variable {
		return segs, false
	}
	return segs[1:], true
}

func dotted(p dsl.Path) string { return strings.Join(p.Segments, ".") }

// field resolves the path of a comparison or `in` to a field, or reports why it is not one.
func (c *checker) field(s scope, p dsl.Path) *Field {
	segs, ok := s.segments(p)
	if !ok || len(segs) != 1 {
		msg := fmt.Sprintf("%q is not a field of %s; use a plain field name", dotted(p), s.entity.Name)
		if s.variable != "" {
			msg = fmt.Sprintf("%q is not a field of %s; write %s.<field>", dotted(p), s.entity.Name, s.variable)
		} else if len(p.Segments) > 1 {
			if r := s.entity.Relation(p.Segments[0]); r != nil {
				msg = fmt.Sprintf("%q is not a field: %s is a relation; use %s", dotted(p), r.Name, relationHint(r, p.Segments[1:]))
			} else {
				msg = fmt.Sprintf("%q is not a field of %s: fields have no dotted paths at the top level", dotted(p), s.entity.Name)
			}
		}
		c.add(dsl.CodeUnknownField, msg, p.Start, p.End, nil)
		return nil
	}
	name := segs[0]
	if f := s.entity.Field(name); f != nil {
		return f
	}
	if r := s.entity.Relation(name); r != nil {
		c.add(dsl.CodeTypeMismatch,
			fmt.Sprintf("%s is a relation, not a field; use %s", r.Name, relationHint(r, nil)),
			p.Start, p.End, nil)
		return nil
	}
	names := make([]string, len(s.entity.Fields))
	for i := range s.entity.Fields {
		names[i] = s.entity.Fields[i].Name
	}
	sugg := suggest(name, names)
	msg := fmt.Sprintf("unknown field %q on %s", name, s.entity.Name)
	switch {
	case len(sugg) > 0:
		msg += "; " + didYouMean(sugg)
	case len(names) <= listFieldsMax:
		msg += "; fields: " + strings.Join(names, ", ")
	}
	c.add(dsl.CodeUnknownField, msg, p.Start, p.End, sugg)
	return nil
}

// relationHint shows the quantifier form of a relation, optionally for the field the caller tried
// to reach through it, for example `orders.some(o => o.totalGbp ...)`.
func relationHint(r *Relation, rest []string) string {
	v := strings.ToLower(r.Name[:1])
	body := "..."
	if len(rest) > 0 {
		body = v + "." + strings.Join(rest, ".") + " ..."
	}
	return fmt.Sprintf("%s.some(%s => %s)", r.Name, v, body)
}

func (c *checker) relation(s scope, x *dsl.Relation) {
	segs, ok := s.segments(x.Path)
	if !ok || len(segs) != 1 {
		c.add(dsl.CodeUnknownRelation, fmt.Sprintf("%q is not a relation of %s", dotted(x.Path), s.entity.Name), x.Path.Start, x.Path.End, nil)
		return
	}
	name := segs[0]
	r := s.entity.Relation(name)
	if r == nil {
		c.unknownRelation(s, x.Path, name)
		return
	}
	if !r.Allows(x.Quantifier) {
		qs := make([]string, len(r.Quantifiers))
		for i, q := range r.Quantifiers {
			qs[i] = string(q)
		}
		start, end := x.Path.Start, x.Path.End
		if st, en, ok := c.locate(x.Path.End, x.VarSpan.Start.Offset, string(x.Quantifier)); ok {
			start, end = st, en
		}
		c.add(dsl.CodeUnknownRelation,
			fmt.Sprintf("%s does not support .%s(); use one of: %s", r.Name, x.Quantifier, strings.Join(qs, ", ")),
			start, end, nil)
	}
	c.expr(scope{entity: r.Entity, variable: x.Var}, x.Body)
}

func (c *checker) unknownRelation(s scope, p dsl.Path, name string) {
	if s.entity.Field(name) != nil {
		c.add(dsl.CodeUnknownRelation,
			fmt.Sprintf("%s is a field, not a relation; compare it directly, for example %s === ...", name, p.Segments[len(p.Segments)-1]),
			p.Start, p.End, nil)
		return
	}
	names := make([]string, len(s.entity.Relations))
	for i := range s.entity.Relations {
		names[i] = s.entity.Relations[i].Name
	}
	sugg := suggest(name, names)
	msg := fmt.Sprintf("unknown relation %q on %s", name, s.entity.Name)
	switch {
	case len(sugg) > 0:
		msg += "; " + didYouMean(sugg)
	case len(names) > 0:
		msg += "; relations: " + strings.Join(names, ", ")
	case s.variable != "":
		msg += fmt.Sprintf("; %s has no relations (relations do not nest)", s.entity.Name)
	default:
		msg += fmt.Sprintf("; %s has no relations", s.entity.Name)
	}
	c.add(dsl.CodeUnknownRelation, msg, p.Start, p.End, sugg)
}

// value checks one literal against the field kind and, for closed lists, its value.
func (c *checker) value(f *Field, op dsl.Op, lit dsl.Literal) {
	if lit == nil {
		return
	}
	sp := lit.Extent()
	mismatch := func(want string) {
		c.add(dsl.CodeTypeMismatch,
			fmt.Sprintf("%s is of type %s; %s, not %s", f.Name, f.Kind, want, describe(lit)),
			sp.Start, sp.End, nil)
	}
	switch f.Kind {
	case KindString:
		s, ok := lit.(dsl.String)
		if !ok {
			mismatch(`compare it with a string literal such as "text"`)
			return
		}
		c.closed(f, s)
	case KindBool:
		if _, ok := lit.(dsl.Bool); !ok {
			mismatch("compare it with true or false")
		}
	case KindInt:
		n, ok := lit.(dsl.Number)
		if !ok {
			mismatch("compare it with a whole number such as 25")
			return
		}
		if _, err := strconv.ParseInt(n.Text, 10, 32); err != nil {
			c.add(dsl.CodeTypeMismatch,
				fmt.Sprintf("%s is of type %s; use a whole number between -2147483648 and 2147483647, not %s", f.Name, f.Kind, n.Text),
				sp.Start, sp.End, nil)
		}
	case KindDecimal:
		n, ok := lit.(dsl.Number)
		if !ok {
			mismatch("compare it with a number such as 99.50")
			return
		}
		if !decimalText.MatchString(n.Text) {
			c.add(dsl.CodeTypeMismatch,
				fmt.Sprintf("%s is of type %s; use a number with at most 2 decimal places, not %s", f.Name, f.Kind, n.Text),
				sp.Start, sp.End, nil)
		}
	case KindDate, KindDateTime:
		switch lit.(type) {
		case dsl.Date:
		case dsl.Ago:
			if !isOrdering(op) {
				c.add(dsl.CodeTypeMismatch,
					fmt.Sprintf("ago() can only be used with >, >=, < or <=, not %s; use date(\"YYYY-MM-DD\") to pick a day", op),
					sp.Start, sp.End, nil)
			}
		default:
			mismatch(`compare it with date("YYYY-MM-DD") or ago("30d")`)
		}
	}
}

func isOrdering(op dsl.Op) bool {
	return op == dsl.OpGt || op == dsl.OpGte || op == dsl.OpLt || op == dsl.OpLte
}

// closed checks a string against the field's closed list: exact, case-sensitive membership.
func (c *checker) closed(f *Field, s dsl.String) {
	if len(f.Closed) == 0 || slices.Contains(f.Closed, s.Value) {
		return
	}
	sugg := suggest(s.Value, f.Closed)
	msg := fmt.Sprintf("%q is not a value of %s", s.Value, f.Name)
	if len(sugg) > 0 {
		msg += "; " + didYouMean(sugg)
	} else {
		msg += "; allowed values: " + strings.Join(f.Closed, ", ")
	}
	c.add(dsl.CodeUnknownValue, msg, s.Start, s.End, sugg)
}

func describe(lit dsl.Literal) string {
	switch x := lit.(type) {
	case dsl.String:
		return "a string"
	case dsl.Number:
		return "the number " + x.Text
	case dsl.Bool:
		return "true/false"
	case dsl.Ago:
		return "ago()"
	case dsl.Date:
		return "date()"
	}
	return "this value"
}

// locate finds word in the source between from and the byte offset to, and returns its range.
// The gap between a path and the next literal holds only whitespace and the operator, so the
// first occurrence is the operator itself.
func (c *checker) locate(from dsl.Pos, to int, word string) (start, end dsl.Pos, ok bool) {
	if from.Offset < 0 || to > len(c.src) || from.Offset > to {
		return start, end, false
	}
	gap := c.src[from.Offset:to]
	i := strings.Index(gap, word)
	if i < 0 {
		return start, end, false
	}
	start = advance(from, gap[:i])
	return start, advance(start, word), true
}

// advance moves a position over text the way the lexer counts: lines by \n, columns in runes.
func advance(p dsl.Pos, text string) dsl.Pos {
	for _, r := range text {
		p.Offset += len(string(r))
		if r == '\n' {
			p.Line++
			p.Col = 1
			continue
		}
		p.Col++
	}
	return p
}

func didYouMean(sugg []string) string {
	q := make([]string, len(sugg))
	for i, s := range sugg {
		q[i] = fmt.Sprintf("%q", s)
	}
	return "did you mean " + strings.Join(q, " or ") + "?"
}

// suggest returns up to maxSuggestions candidates close to input: those that start with it, then
// those within maxEditDist edits, compared case-insensitively. Ties keep the candidate order.
func suggest(input string, candidates []string) []string {
	in := strings.ToLower(input)
	type scored struct {
		name  string
		score int
	}
	var hits []scored
	for _, cand := range candidates {
		lc := strings.ToLower(cand)
		switch d := levenshtein.ComputeDistance(in, lc); {
		case in != "" && strings.HasPrefix(lc, in):
			hits = append(hits, scored{cand, 0})
		case d <= maxEditDist:
			hits = append(hits, scored{cand, d})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score < hits[j].score })
	var out []string
	for _, h := range hits[:min(len(hits), maxSuggestions)] {
		out = append(out, h.name)
	}
	return out
}
