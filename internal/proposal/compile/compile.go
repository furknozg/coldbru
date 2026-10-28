// Package compile turns a checked DSL expression into the JSON variable of the Customer GraphQL
// `CustomerFilter` input (D-067). The translation is one-to-one with the AST: no merging of equal
// fields and no logical simplification, so the filter shown in the preview is the one the LLM
// wrote. Alongside the variable it keeps an index from GraphQL filter paths to DSL source spans,
// so that an error Customer reports for a filter path can be shown at the DSL position (D-067).
package compile

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/furknozg/coldbru/internal/proposal/catalog"
	"github.com/furknozg/coldbru/internal/proposal/dsl"
)

// Root is the first segment of every filter path: the name of the GraphQL argument (D-062).
const Root = "filter"

// Filter is a compiled filter: the GraphQL variable and the path-to-span index.
type Filter struct {
	src   string
	value map[string]any
	spans map[string]dsl.Span
}

// Compile parses src, runs the semantic pass and builds the filter. A parse error is returned as
// a one-element slice; the semantic pass returns every error it finds (D-066). The filter is nil
// when there are errors.
func Compile(src string) (*Filter, []*dsl.Error) {
	e, err := dsl.Parse(src)
	if err != nil {
		var de *dsl.Error
		if !errors.As(err, &de) {
			// Parse documents *dsl.Error only; keep the contract if that ever changes.
			start := dsl.Pos{Line: 1, Col: 1}
			de = dsl.NewError(src, dsl.CodeSyntaxError, err.Error(), start, start)
		}
		return nil, []*dsl.Error{de}
	}
	if errs := catalog.Check(src, e); len(errs) > 0 {
		return nil, errs
	}
	b := &builder{src: src, spans: map[string]dsl.Span{}}
	v, err := b.expr(catalog.Root, e, []any{Root})
	if err != nil {
		// Unreachable after Check; reported instead of panicking.
		start := dsl.Pos{Line: 1, Col: 1}
		return nil, []*dsl.Error{dsl.NewError(src, dsl.CodeSyntaxError, err.Error(), start, start)}
	}
	return &Filter{src: src, value: v, spans: b.spans}, nil
}

// Source returns the DSL text the filter was compiled from.
func (f *Filter) Source() string { return f.src }

// Variable returns the value of the GraphQL `filter` variable. The tree is shared with the
// Filter; callers must not modify it.
func (f *Filter) Variable() map[string]any { return f.value }

// MarshalJSON encodes the variable. encoding/json sorts map keys, so the output is deterministic.
func (f *Filter) MarshalJSON() ([]byte, error) { return json.Marshal(f.value) }

// Span returns the DSL range for a GraphQL filter path in Customer's FilterPath style: strings
// for fields, integers for list indexes, starting with "filter" (D-062). When the exact path is
// not indexed it falls back to the longest indexed prefix. It reports false when not even the
// root is known, for example for a path that does not start with "filter".
func (f *Filter) Span(path []any) (dsl.Span, bool) {
	keys := make([]string, 0, len(path))
	for _, seg := range path {
		k, ok := segKey(seg)
		if !ok {
			break // an unusable segment ends the usable prefix
		}
		keys = append(keys, k)
	}
	for n := len(keys); n > 0; n-- {
		if sp, ok := f.spans[strings.Join(keys[:n], "\x00")]; ok {
			return sp, true
		}
	}
	return dsl.Span{}, false
}

// segKey encodes one path segment. Strings and integers never collide: indexes carry a prefix no
// field name can start with. Numbers decoded from JSON arrive as float64 or json.Number.
func segKey(seg any) (string, bool) {
	switch v := seg.(type) {
	case string:
		return "s" + v, true
	case int:
		return "i" + strconv.Itoa(v), true
	case int64:
		return "i" + strconv.FormatInt(v, 10), true
	case float64:
		if v == float64(int64(v)) {
			return "i" + strconv.FormatInt(int64(v), 10), true
		}
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return "i" + strconv.FormatInt(n, 10), true
		}
	}
	return "", false
}

type builder struct {
	src   string
	spans map[string]dsl.Span
}

// mark records the span of path unless an outer construct already did. Callers mark the outer
// node first, so where two nodes share a path (a quantifier and its body, `not` and its operand)
// the larger one wins.
func (b *builder) mark(path []any, sp dsl.Span) {
	keys := make([]string, len(path))
	for i, seg := range path {
		keys[i], _ = segKey(seg)
	}
	k := strings.Join(keys, "\x00")
	if _, ok := b.spans[k]; !ok {
		b.spans[k] = sp
	}
}

func with(path []any, seg ...any) []any {
	out := make([]any, 0, len(path)+len(seg))
	out = append(out, path...)
	return append(out, seg...)
}

var opNames = map[dsl.Op]string{
	dsl.OpEq: "eq", dsl.OpNe: "ne", dsl.OpGt: "gt", dsl.OpGte: "gte", dsl.OpLt: "lt", dsl.OpLte: "lte",
}

// expr builds the filter object of e, which occupies path. ent is the entity the fields of e
// belong to.
func (b *builder) expr(ent *catalog.Entity, e dsl.Expr, path []any) (map[string]any, error) {
	if e == nil {
		return nil, errors.New("missing expression")
	}
	sp := e.Extent()
	b.mark(path, sp)
	switch x := e.(type) {
	case *dsl.And:
		return b.list(ent, "and", x.Terms, sp, path)
	case *dsl.Or:
		return b.list(ent, "or", x.Terms, sp, path)
	case *dsl.Not:
		p := with(path, "not")
		b.mark(p, sp)
		inner, err := b.expr(ent, x.X, p)
		if err != nil {
			return nil, err
		}
		return map[string]any{"not": inner}, nil
	case *dsl.Compare:
		return b.compare(ent, x, path)
	case *dsl.In:
		return b.in(ent, x, path)
	case *dsl.Relation:
		return b.relation(ent, x, path)
	}
	return nil, fmt.Errorf("unsupported expression %T", e)
}

func (b *builder) list(ent *catalog.Entity, key string, terms []dsl.Expr, sp dsl.Span, path []any) (map[string]any, error) {
	p := with(path, key)
	b.mark(p, sp)
	out := make([]any, len(terms))
	for i, t := range terms {
		v, err := b.expr(ent, t, with(p, i))
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return map[string]any{key: out}, nil
}

// fieldOf returns the field name of p and its catalog entry. Inside a relation body the first
// segment is the lambda variable (D-065).
func fieldOf(ent *catalog.Entity, p dsl.Path, inRelation bool) (string, *catalog.Field, error) {
	name := p.Segments[len(p.Segments)-1]
	if inRelation && len(p.Segments) != 2 || !inRelation && len(p.Segments) != 1 {
		return "", nil, fmt.Errorf("unexpected path %q", strings.Join(p.Segments, "."))
	}
	f := ent.Field(name)
	if f == nil {
		return "", nil, fmt.Errorf("unknown field %q on %s", name, ent.Name)
	}
	return name, f, nil
}

func (b *builder) compare(ent *catalog.Entity, x *dsl.Compare, path []any) (map[string]any, error) {
	name, f, err := fieldOf(ent, x.Field, ent.Name != catalog.Root.Name)
	if err != nil {
		return nil, err
	}
	op, ok := opNames[x.Op]
	if !ok {
		return nil, fmt.Errorf("unsupported operator %q", x.Op)
	}
	v, err := literal(f, x.Value)
	if err != nil {
		return nil, err
	}
	b.mark(with(path, name), x.Field.Span)
	vsp := x.Value.Extent()
	b.markValue(with(path, name, op), x.Value, vsp)
	return map[string]any{name: map[string]any{op: v}}, nil
}

func (b *builder) in(ent *catalog.Entity, x *dsl.In, path []any) (map[string]any, error) {
	name, f, err := fieldOf(ent, x.Field, ent.Name != catalog.Root.Name)
	if err != nil {
		return nil, err
	}
	b.mark(with(path, name), x.Field.Span)
	inPath := with(path, name, "in")
	b.mark(inPath, x.Span)
	vals := make([]any, len(x.Values))
	for i, lit := range x.Values {
		v, err := literal(f, lit)
		if err != nil {
			return nil, err
		}
		vals[i] = v
		b.markValue(with(inPath, i), lit, lit.Extent())
	}
	return map[string]any{name: map[string]any{"in": vals}}, nil
}

// markValue records a literal at its operand path and, for time values, at the path of the
// TimeValue field Customer may report ("ago" or "date").
func (b *builder) markValue(path []any, lit dsl.Literal, sp dsl.Span) {
	b.mark(path, sp)
	switch lit.(type) {
	case dsl.Ago:
		b.mark(with(path, "ago"), sp)
	case dsl.Date:
		b.mark(with(path, "date"), sp)
	}
}

func (b *builder) relation(ent *catalog.Entity, x *dsl.Relation, path []any) (map[string]any, error) {
	if len(x.Path.Segments) == 0 {
		return nil, errors.New("relation without a name")
	}
	name := x.Path.Segments[len(x.Path.Segments)-1]
	rel := ent.Relation(name)
	if rel == nil {
		return nil, fmt.Errorf("unknown relation %q on %s", name, ent.Name)
	}
	b.mark(with(path, name), x.Path.Span)
	qp := with(path, name, string(x.Quantifier))
	b.mark(qp, x.Span)
	body, err := b.expr(rel.Entity, x.Body, qp)
	if err != nil {
		return nil, err
	}
	return map[string]any{name: map[string]any{string(x.Quantifier): body}}, nil
}

// literal encodes a value for a field of the given kind (D-058): Int as a JSON number, Decimal as
// a JSON string of the literal text, Bool, String, and time values as TimeValue objects.
func literal(f *catalog.Field, lit dsl.Literal) (any, error) {
	switch x := lit.(type) {
	case dsl.String:
		return x.Value, nil
	case dsl.Bool:
		return x.Value, nil
	case dsl.Number:
		if f.Kind == catalog.KindDecimal {
			return x.Text, nil
		}
		// Parsed rather than passed through as text: "007" is a valid literal but not valid JSON.
		n, err := strconv.ParseInt(x.Text, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("number %q of %s: %w", x.Text, f.Name, err)
		}
		return n, nil
	case dsl.Ago:
		return map[string]any{"ago": x.Text}, nil
	case dsl.Date:
		return map[string]any{"date": x.Text}, nil
	}
	return nil, fmt.Errorf("unsupported literal %T", lit)
}
