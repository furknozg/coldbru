package audience

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/furknozg/coldbru/internal/customer/graph/model"
)

// Limits on a filter (D-062). Counting rules:
//
//   - Depth: the root filter object is depth 1. Every filter object nested
//     inside an and/or element, a not operand or a some/none/every quantifier
//     is one level deeper. A filter whose deepest object is at depth 9 is
//     rejected.
//   - Nodes: every comparison operator (eq, ne, gt, gte, lt, lte, in; an in
//     list is one node however long), every and/or/not key and every
//     some/none/every quantifier counts one. Plain field names do not.
//   - In values: the length of one in list.
const (
	MaxDepth    = 8
	MaxNodes    = 100
	MaxInValues = 100
)

// Largest ago values accepted: 100 years of days, and the same span in hours.
// Larger numbers could overflow the interval type and add nothing.
const (
	maxAgoDays  = 36500
	maxAgoHours = 36500 * 24
)

var agoPattern = regexp.MustCompile(`^[1-9][0-9]*(d|h)$`)

// Compiler compiles a CustomerFilter into SQL. It is safe for concurrent use.
type Compiler struct {
	// Cities validates and canonicalizes city values.
	Cities *Cities
}

// Compiled is a filter turned into SQL for dwh.dim_customer aliased as c.
type Compiled struct {
	// Where is a boolean SQL expression over c (and correlated subqueries). It
	// is either an atom or fully parenthesized, so it can be joined with AND/OR
	// without extra parentheses. It does not contain the reachability rule.
	Where string
	// Args are the values of $1..$n in order.
	Args []any
}

// Compile validates f against the catalog and limits and returns the WHERE
// fragment. A nil or empty filter matches every customer. Errors are *Error.
func (c *Compiler) Compile(f *model.CustomerFilter) (Compiled, error) {
	s := &state{cities: c.Cities}
	if f == nil {
		return Compiled{Where: sqlTrue}, nil
	}
	where, err := s.object(customerEntity, reflect.ValueOf(f).Elem(), []any{"filter"}, 1)
	if err != nil {
		return Compiled{}, err
	}
	return Compiled{Where: where, Args: s.args}, nil
}

const sqlTrue = "TRUE"

// state is one compilation: the growing argument list and the node counter.
type state struct {
	cities *Cities
	args   []any
	nodes  int
}

// arg stores v and returns its placeholder.
func (s *state) arg(v any) string {
	s.args = append(s.args, v)
	return "$" + strconv.Itoa(len(s.args))
}

// node counts one condition node and enforces MaxNodes.
func (s *state) node(path []any) error {
	s.nodes++
	if s.nodes > MaxNodes {
		return limitExceeded(path, "filter has more than %d condition nodes", MaxNodes)
	}
	return nil
}

// object compiles one filter object (CustomerFilter, OrderFilter, ...) held in
// v, a struct value. Everything set in it is ANDed.
func (s *state) object(ent *Entity, v reflect.Value, path []any, depth int) (string, error) {
	if depth > MaxDepth {
		return "", limitExceeded(path, "filter is nested deeper than %d levels", MaxDepth)
	}
	var parts []string

	for _, comb := range []struct {
		field, key, join string
	}{{"And", "and", " AND "}, {"Or", "or", " OR "}} {
		list := v.FieldByName(comb.field)
		if list.IsNil() {
			continue
		}
		p := withSeg(path, comb.key)
		if err := s.node(p); err != nil {
			return "", err
		}
		if list.Len() == 0 {
			return "", badFilter(p, "%s needs at least one filter", comb.key)
		}
		sub := make([]string, 0, list.Len())
		for i := range list.Len() {
			elemPath := withSeg(p, i)
			elem := list.Index(i)
			if elem.IsNil() {
				return "", badFilter(elemPath, "%s element must not be null", comb.key)
			}
			sql, err := s.object(ent, elem.Elem(), elemPath, depth+1)
			if err != nil {
				return "", err
			}
			sub = append(sub, sql)
		}
		parts = append(parts, group(sub, comb.join))
	}

	if not := v.FieldByName("Not"); !not.IsNil() {
		p := withSeg(path, "not")
		if err := s.node(p); err != nil {
			return "", err
		}
		sql, err := s.object(ent, not.Elem(), p, depth+1)
		if err != nil {
			return "", err
		}
		parts = append(parts, notExpr(sql))
	}

	for i := range ent.Fields {
		f := &ent.Fields[i]
		fv := v.Field(f.index)
		if fv.IsNil() {
			continue
		}
		sql, err := s.field(ent, f, fv.Interface(), withSeg(path, f.Name))
		if err != nil {
			return "", err
		}
		parts = append(parts, sql)
	}

	for i := range ent.Relations {
		rel := &ent.Relations[i]
		rv := v.Field(rel.index)
		if rv.IsNil() {
			continue
		}
		sql, err := s.relation(rel, rv.Elem(), withSeg(path, rel.Name), depth)
		if err != nil {
			return "", err
		}
		parts = append(parts, sql)
	}

	return group(parts, " AND "), nil
}

// group joins parts with op; it returns TRUE for none and the bare part for one.
func group(parts []string, op string) string {
	switch len(parts) {
	case 0:
		return sqlTrue
	case 1:
		return parts[0]
	}
	return "(" + strings.Join(parts, op) + ")"
}

// notExpr negates x with NULL counted as no match, so NOT of an unknown
// condition is true (D-062).
func notExpr(x string) string { return "NOT COALESCE(" + paren(x) + ", false)" }

// paren wraps x in parentheses unless one pair already encloses all of it.
func paren(x string) string {
	if x[0] == '(' {
		depth := 0
		for i := range len(x) {
			switch x[i] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					if i == len(x)-1 {
						return x
					}
					return "(" + x + ")"
				}
			}
		}
	}
	return "(" + x + ")"
}

// relation compiles a some/none/every object of one relation. Several
// quantifiers are ANDed (D-058).
func (s *state) relation(rel *Relation, rv reflect.Value, path []any, depth int) (string, error) {
	var parts []string
	for _, q := range []struct{ field, key string }{{"Some", "some"}, {"None", "none"}, {"Every", "every"}} {
		qv := rv.FieldByName(q.field)
		if qv.IsNil() {
			continue
		}
		p := withSeg(path, q.key)
		if err := s.node(p); err != nil {
			return "", err
		}
		inner, err := s.object(rel.Entity, qv.Elem(), p, depth+1)
		if err != nil {
			return "", err
		}
		sub := rel.Entity
		exists := fmt.Sprintf("SELECT 1 FROM %s %s WHERE %s = %s",
			sub.Table, sub.Alias, sub.Qualified(childKey), rootKey)
		switch q.key {
		case "some":
			parts = append(parts, fmt.Sprintf("EXISTS (%s AND %s)", exists, paren(inner)))
		case "none":
			parts = append(parts, fmt.Sprintf("NOT EXISTS (%s AND %s)", exists, paren(inner)))
		default: // every: no related row fails the predicate
			parts = append(parts, fmt.Sprintf("NOT EXISTS (%s AND %s)", exists, notExpr(inner)))
		}
	}
	if len(parts) == 0 {
		return "", badFilter(path, "relation filter needs some, none or every")
	}
	return group(parts, " AND "), nil
}

// operation is one set operator of a field filter with its single operand.
type operation struct {
	op  string
	val any
}

// operators is the set of operators found in a field filter object.
type operators struct {
	ops   []operation
	in    []any
	hasIn bool
}

func put[T any](ops []operation, name string, p *T) []operation {
	if p == nil {
		return ops
	}
	return append(ops, operation{name, *p})
}

func anyList[T any](in []T) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}

// extract reads the operators of a typed filter object in fixed order
// (eq, ne, gt, gte, lt, lte, in).
func extract(filter any) operators {
	var o operators
	switch x := filter.(type) {
	case *model.StringFilter:
		o.ops = put(put(o.ops, "eq", x.Eq), "ne", x.Ne)
		o.hasIn, o.in = x.In != nil, anyList(x.In)
	case *model.BooleanFilter:
		o.ops = put(put(o.ops, "eq", x.Eq), "ne", x.Ne)
	case *model.IntFilter:
		o.ops = putOrdered(o.ops, x.Eq, x.Ne, x.Gt, x.Gte, x.Lt, x.Lte)
		o.hasIn, o.in = x.In != nil, anyList(x.In)
	case *model.DecimalFilter:
		o.ops = putOrdered(o.ops, x.Eq, x.Ne, x.Gt, x.Gte, x.Lt, x.Lte)
		o.hasIn, o.in = x.In != nil, anyList(x.In)
	case *model.DateFilter:
		o.ops = putOrdered(o.ops, x.Eq, x.Ne, x.Gt, x.Gte, x.Lt, x.Lte)
		o.hasIn, o.in = x.In != nil, anyList(x.In)
	case *model.DateTimeFilter:
		o.ops = putOrdered(o.ops, x.Eq, x.Ne, x.Gt, x.Gte, x.Lt, x.Lte)
		o.hasIn, o.in = x.In != nil, anyList(x.In)
	}
	return o
}

func putOrdered[T any](ops []operation, eq, ne, gt, gte, lt, lte *T) []operation {
	ops = put(ops, "eq", eq)
	ops = put(ops, "ne", ne)
	ops = put(ops, "gt", gt)
	ops = put(ops, "gte", gte)
	ops = put(ops, "lt", lt)
	return put(ops, "lte", lte)
}

// field compiles the operators of one field filter, ANDed.
func (s *state) field(ent *Entity, f *Field, filter any, path []any) (string, error) {
	o := extract(filter)
	if len(o.ops) == 0 && !o.hasIn {
		return "", badFilter(path, "%s needs at least one operator", f.Name)
	}
	col := ent.Qualified(f.Column)
	var parts []string

	for _, op := range o.ops {
		opPath := withSeg(path, op.op)
		if err := s.node(opPath); err != nil {
			return "", err
		}
		operand, err := s.operand(f, op.val, opPath)
		if err != nil {
			return "", err
		}
		switch op.op {
		case "eq":
			parts = append(parts, col+" IS NOT DISTINCT FROM "+operand)
		case "ne":
			parts = append(parts, col+" IS DISTINCT FROM "+operand)
		default:
			parts = append(parts, col+" "+sqlCompare[op.op]+" "+operand)
		}
	}

	if o.hasIn {
		inPath := withSeg(path, "in")
		if err := s.node(inPath); err != nil {
			return "", err
		}
		sql, err := s.inList(col, f, o.in, inPath)
		if err != nil {
			return "", err
		}
		parts = append(parts, sql)
	}
	return group(parts, " AND "), nil
}

var sqlCompare = map[string]string{"gt": ">", "gte": ">=", "lt": "<", "lte": "<="}

// operand validates v for f and returns its SQL expression with the value
// stored as an argument.
func (s *state) operand(f *Field, v any, path []any) (string, error) {
	switch f.Kind {
	case KindString:
		str, err := s.checkString(f, v.(string), path)
		if err != nil {
			return "", err
		}
		return s.arg(str), nil
	case KindInt:
		return s.arg(int64(v.(int))) + "::bigint", nil
	case KindDecimal:
		return s.arg(string(v.(model.Decimal))) + "::numeric", nil
	case KindBool:
		return s.arg(v.(bool)) + "::boolean", nil
	case KindDate, KindDateTime:
		tv := v.(model.TimeValue) // single operands are stored by value
		return s.timeOperand(f, &tv, path)
	}
	return "", fmt.Errorf("audience: field %s has unknown kind %d", f.Name, f.Kind)
}

// checkString validates a text value against the field's closed list or the
// city dictionary and returns the value to send (the canonical city spelling).
func (s *state) checkString(f *Field, v string, path []any) (string, error) {
	switch {
	case f.City:
		if canon, ok := s.cities.Lookup(v); ok {
			return canon, nil
		}
		return "", unknownValue(path, f.Name, v, s.cities.Suggest(v))
	case f.Closed != nil:
		if slices.Contains(f.Closed, v) {
			return v, nil
		}
		return "", unknownValue(path, f.Name, v, Suggest(v, f.Closed))
	}
	return v, nil
}

// inList compiles an in operator. Plain values become one typed array
// argument; time values, which are expressions, become an IN list.
func (s *state) inList(col string, f *Field, values []any, path []any) (string, error) {
	if len(values) == 0 {
		return "", badFilter(path, "in needs at least one value")
	}
	if len(values) > MaxInValues {
		return "", limitExceeded(path, "in has %d values, the limit is %d", len(values), MaxInValues)
	}
	switch f.Kind {
	case KindString:
		strs := make([]string, len(values))
		for i, v := range values {
			str, err := s.checkString(f, v.(string), withSeg(path, i))
			if err != nil {
				return "", err
			}
			strs[i] = str
		}
		return col + " = ANY(" + s.arg(strs) + "::text[])", nil
	case KindInt:
		ints := make([]int64, len(values))
		for i, v := range values {
			ints[i] = int64(v.(int))
		}
		return col + " = ANY(" + s.arg(ints) + "::bigint[])", nil
	case KindDecimal:
		// text[] then numeric[]: pgx has no binary encoder from strings to numeric[].
		decs := make([]string, len(values))
		for i, v := range values {
			decs[i] = string(v.(model.Decimal))
		}
		return col + " = ANY(" + s.arg(decs) + "::text[]::numeric[])", nil
	case KindDate, KindDateTime:
		exprs := make([]string, len(values))
		for i, v := range values {
			e, err := s.timeOperand(f, v.(*model.TimeValue), withSeg(path, i))
			if err != nil {
				return "", err
			}
			exprs[i] = e
		}
		return col + " IN (" + strings.Join(exprs, ", ") + ")", nil
	}
	return "", badFilter(path, "%s does not support in", f.Name)
}

// timeOperand compiles a TimeValue for a Date or DateTime column. ago is
// resolved against now() when the query runs (D-039).
func (s *state) timeOperand(f *Field, tv *model.TimeValue, path []any) (string, error) {
	if tv == nil {
		return "", badFilter(path, "time value must not be null")
	}
	set := 0
	for _, b := range []bool{tv.Ago != nil, tv.Date != nil, tv.At != nil} {
		if b {
			set++
		}
	}
	if set != 1 {
		return "", badFilter(path, "time value must set exactly one of ago, date, at")
	}
	isDate := f.Kind == KindDate
	switch {
	case tv.Ago != nil:
		p := withSeg(path, "ago")
		interval, err := agoInterval(*tv.Ago)
		if err != nil {
			return "", &Error{Code: CodeBadUserInput, Message: err.Error(), FilterPath: p}
		}
		ph := s.arg(interval)
		if isDate {
			return "(now() - " + ph + "::interval)::date", nil
		}
		return "now() - " + ph + "::interval", nil
	case tv.Date != nil:
		ph := s.arg(tv.Date.Time)
		if isDate {
			return ph + "::date", nil
		}
		return ph + "::date::timestamptz", nil
	default:
		ph := s.arg(*tv.At)
		if isDate {
			return ph + "::timestamptz::date", nil
		}
		return ph + "::timestamptz", nil
	}
}

// agoInterval turns "30d" into the interval text "30 days" and "12h" into
// "12 hours".
func agoInterval(ago string) (string, error) {
	if !agoPattern.MatchString(ago) {
		return "", fmt.Errorf(`ago %q must match ^[1-9][0-9]*(d|h)$, for example "30d" or "12h"`, ago)
	}
	unit := ago[len(ago)-1]
	n, err := strconv.Atoi(ago[:len(ago)-1])
	maxN, word := maxAgoDays, "days"
	if unit == 'h' {
		maxN, word = maxAgoHours, "hours"
	}
	if err != nil || n > maxN {
		return "", fmt.Errorf("ago %q is too large, the limit is %d %s", ago, maxN, word)
	}
	return strconv.Itoa(n) + " " + word, nil
}
