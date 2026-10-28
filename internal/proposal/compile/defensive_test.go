package compile

import (
	"testing"

	"github.com/furknozg/coldbru/internal/proposal/catalog"
	"github.com/furknozg/coldbru/internal/proposal/dsl"
)

// Compile runs catalog.Check first, so the builder never sees these trees from source text. They
// are built by hand to show the builder reports an error instead of panicking.
func TestBuilderRejectsTreesCheckWouldHaveRefused(t *testing.T) {
	path := func(segs ...string) dsl.Path { return dsl.Path{Segments: segs} }
	str := dsl.String{Value: "x"}
	tests := []struct {
		name string
		e    dsl.Expr
	}{
		{"unknown field", &dsl.Compare{Field: path("nope"), Op: dsl.OpEq, Value: str}},
		{"dotted path at the root", &dsl.Compare{Field: path("a", "b"), Op: dsl.OpEq, Value: str}},
		{"unknown operator", &dsl.Compare{Field: path("city"), Op: dsl.Op("~="), Value: str}},
		{"literal type the builder does not know", &dsl.Compare{Field: path("city"), Op: dsl.OpEq}},
		{"huge int", &dsl.Compare{Field: path("totalOrders"), Op: dsl.OpEq, Value: dsl.Number{Text: "99999999999999999999"}}},
		{"in with unknown field", &dsl.In{Field: path("nope"), Values: []dsl.Literal{str}}},
		{"in with bad literal", &dsl.In{Field: path("city"), Values: []dsl.Literal{nil}}},
		{"unknown relation", &dsl.Relation{Path: path("nope"), Quantifier: dsl.Some, Var: "o", Body: &dsl.Compare{Field: path("o", "x"), Op: dsl.OpEq, Value: str}}},
		{"relation without a name", &dsl.Relation{Quantifier: dsl.Some, Var: "o"}},
		{"bad term inside and", &dsl.And{Terms: []dsl.Expr{&dsl.Compare{Field: path("nope"), Op: dsl.OpEq, Value: str}}}},
		{"bad operand inside not", &dsl.Not{X: &dsl.Compare{Field: path("nope"), Op: dsl.OpEq, Value: str}}},
		{"bad body inside relation", &dsl.Relation{Path: path("orders"), Quantifier: dsl.Some, Var: "o", Body: &dsl.Compare{Field: path("o", "nope"), Op: dsl.OpEq, Value: str}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &builder{spans: map[string]dsl.Span{}}
			if _, err := b.expr(catalog.Root, tt.e, []any{Root}); err == nil {
				t.Error("want an error")
			}
		})
	}
	b := &builder{spans: map[string]dsl.Span{}}
	if _, err := b.expr(catalog.Root, nil, []any{Root}); err == nil {
		t.Error("nil expression: want an error")
	}
}
