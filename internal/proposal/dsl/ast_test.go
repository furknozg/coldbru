package dsl

import "testing"

func TestEqual(t *testing.T) {
	t.Parallel()
	same := []struct{ name, a, b string }{
		{"whitespace and quoting differ", `a   ===   'x'`, `a === "x"`},
		{"spans differ", "a === 1 &&\n b === 2", `a === 1 && b === 2`},
		{"redundant parentheses", `((a === 1))`, `a === 1`},
		{"flattened groups", `(a === 1 && b === 2) && c === 3`, `a === 1 && b === 2 && c === 3`},
		{"relation", `orders.some(o => o.x > 1)`, `orders.some( o => o.x>1 )`},
		{"in list", `a in [1, "b"]`, `a in [1,"b"]`},
	}
	for _, tt := range same {
		t.Run("equal/"+tt.name, func(t *testing.T) {
			t.Parallel()
			if !Equal(mustParse(t, tt.a), mustParse(t, tt.b)) {
				t.Errorf("%q and %q must be Equal", tt.a, tt.b)
			}
		})
	}
	different := []struct{ name, a, b string }{
		{"field", `a === 1`, `b === 1`},
		{"dotted field", `a.b === 1`, `a.c === 1`},
		{"operator", `a === 1`, `a !== 1`},
		{"number text", `a === 1`, `a === 1.0`},
		{"number vs string", `a === 1`, `a === "1"`},
		{"bool", `a === true`, `a === false`},
		{"string value", `a === "x"`, `a === "y"`},
		{"ago text", `a >= ago("1d")`, `a >= ago("2d")`},
		{"ago vs date", `a >= ago("1d")`, `a >= date("2026-01-01")`},
		{"date text", `a >= date("2026-01-01")`, `a >= date("2026-01-02")`},
		{"literal kind ago", `a >= ago("1d")`, `a >= "1d"`},
		{"and vs or", `a === 1 && b === 1`, `a === 1 || b === 1`},
		{"term order", `a === 1 && b === 1`, `b === 1 && a === 1`},
		{"term count", `a === 1 && b === 1`, `a === 1 && b === 1 && c === 1`},
		{"not", `!(a === 1)`, `a === 1`},
		{"not operand", `!(a === 1)`, `!(a === 2)`},
		{"in values", `a in [1, 2]`, `a in [1, 3]`},
		{"in length", `a in [1]`, `a in [1, 2]`},
		{"in vs compare", `a in [1]`, `a === 1`},
		{"quantifier", `orders.some(o => o.x > 1)`, `orders.none(o => o.x > 1)`},
		{"relation path", `orders.some(o => o.x > 1)`, `cartItems.some(o => o.x > 1)`},
		{"variable", `orders.some(o => o.x > 1)`, `orders.some(p => p.x > 1)`},
		{"body", `orders.some(o => o.x > 1)`, `orders.some(o => o.x > 2)`},
		{"relation vs compare", `orders.some(o => o.x > 1)`, `orders.x > 1`},
	}
	for _, tt := range different {
		t.Run("different/"+tt.name, func(t *testing.T) {
			t.Parallel()
			a, b := mustParse(t, tt.a), mustParse(t, tt.b)
			if Equal(a, b) || Equal(b, a) {
				t.Errorf("%q and %q must differ", tt.a, tt.b)
			}
		})
	}
}

func TestEqualNil(t *testing.T) {
	t.Parallel()
	e := mustParse(t, `a === 1`)
	if !Equal(nil, nil) {
		t.Error("Equal(nil, nil) must be true")
	}
	if Equal(e, nil) || Equal(nil, e) {
		t.Error("a tree never equals nil")
	}
	var nilCmp *Compare
	var nilAnd *And
	var nilOr *Or
	var nilNot *Not
	var nilIn *In
	var nilRel *Relation
	for _, n := range []Expr{nilCmp, nilAnd, nilOr, nilNot, nilIn, nilRel} {
		if Equal(n, e) || Equal(e, n) {
			t.Errorf("typed nil %T must not equal a tree", n)
		}
		if !Equal(n, n) {
			t.Errorf("typed nil %T must equal itself", n)
		}
	}
}

func TestSpanExtent(t *testing.T) {
	t.Parallel()
	e := mustParse(t, "a === 1")
	sp := e.Extent()
	if sp.Start.Offset != 0 || sp.End.Offset != 7 {
		t.Errorf("Extent() = %+v", sp)
	}
}
