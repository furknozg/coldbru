package dsl

import "testing"

func TestPrint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"double quotes", `city === 'Ankara'`, `city === "Ankara"`},
		{"quote inside single quoted", `a === 'say "hi"'`, `a === "say \"hi\""`},
		{"escaped quote", `a === "say \"hi\""`, `a === "say \"hi\""`},
		{"escaped single quote is plain", `a === 'it\'s'`, `a === "it's"`},
		{"backslash", `a === 'a\\b'`, `a === "a\\b"`},
		{"turkish", `city === 'İstanbul'`, `city === "İstanbul"`},
		{"spacing around operators", `a===1&&b!==2||c>=3`, `a === 1 && b !== 2 || c >= 3`},
		{"all comparison operators", `a>1&&b>=1&&c<1&&d<=1&&e===1&&f!==1`, `a > 1 && b >= 1 && c < 1 && d <= 1 && e === 1 && f !== 1`},
		{"no space after not", `! (a === 1)`, `!(a === 1)`},
		{"double not", `! ! (a === 1)`, `!!(a === 1)`},
		{"not of a group", `!(a === 1 || b === 2)`, `!(a === 1 || b === 2)`},
		{"not of a relation needs no parentheses", `!orders.some(o => o.x > 1)`, `!orders.some(o => o.x > 1)`},
		{"redundant parentheses dropped", `((a === 1))`, `a === 1`},
		{"and inside or needs none", `(a === 1 && b === 2) || c === 3`, `a === 1 && b === 2 || c === 3`},
		{"or inside and keeps parentheses", `(a === 1 || b === 2) && c === 3`, `(a === 1 || b === 2) && c === 3`},
		{"or inside and, either side", `a === 1 && (b === 2 || c === 3)`, `a === 1 && (b === 2 || c === 3)`},
		{"same-kind groups merge", `a === 1 && (b === 2 && c === 3)`, `a === 1 && b === 2 && c === 3`},
		{"or inside not inside and", `!(a === 1 || b === 2) && c === 3`, `!(a === 1 || b === 2) && c === 3`},
		{"lists", `a in [ 1 ,2,  "x" ]`, `a in [1, 2, "x"]`},
		{"numbers keep their text", `a === 007 && b === -0.50 && c === 12`, `a === 007 && b === -0.50 && c === 12`},
		{"booleans", `a === true && b !== false`, `a === true && b !== false`},
		{"time literals", `a >= ago('30d') && b < date('2026-01-01')`, `a >= ago("30d") && b < date("2026-01-01")`},
		{"relation", "orders.some( o =>\n  o.x > 1 &&\n  o.y === 'z'\n)", `orders.some(o => o.x > 1 && o.y === "z")`},
		{"relation inside or", `orders.none(o => o.x > 1) || cartItems.every(c => c.q === 1)`, `orders.none(o => o.x > 1) || cartItems.every(c => c.q === 1)`},
		{"nested relation", `orders.some(o=>o.items.some(i=>i.q>=2))`, `orders.some(o => o.items.some(i => i.q >= 2))`},
		{"or inside a lambda body is not parenthesized", `orders.some(o => o.a === 1 || o.b === 2)`, `orders.some(o => o.a === 1 || o.b === 2)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := mustParse(t, tt.src)
			got := Print(e)
			if got != tt.want {
				t.Errorf("Print(Parse(%q)) = %q, want %q", tt.src, got, tt.want)
			}
			if again := Print(mustParse(t, got)); again != got {
				t.Errorf("Print is not idempotent: %q then %q", got, again)
			}
			if !Equal(mustParse(t, got), e) {
				t.Errorf("round trip changed the tree for %q", tt.src)
			}
		})
	}
}

// TestPrintBuiltTrees prints trees that were not produced by Parse, to cover
// the precedence rules independently of the parser.
func TestPrintBuiltTrees(t *testing.T) {
	t.Parallel()
	a, b, c := cmp("a", OpEq, num("1")), cmp("b", OpEq, num("2")), cmp("c", OpEq, num("3"))
	tests := []struct {
		name string
		e    Expr
		want string
	}{
		{"or under and", &And{Terms: []Expr{&Or{Terms: []Expr{a, b}}, c}}, `(a === 1 || b === 2) && c === 3`},
		{"and under or", &Or{Terms: []Expr{&And{Terms: []Expr{a, b}}, c}}, `a === 1 && b === 2 || c === 3`},
		{"not over and", &Not{X: &And{Terms: []Expr{a, b}}}, `!(a === 1 && b === 2)`},
		{"not over compare", &Not{X: a}, `!(a === 1)`},
		{"not over not", &Not{X: &Not{X: a}}, `!!(a === 1)`},
		{"not over in", &Not{X: &In{Field: path("a"), Values: []Literal{num("1")}}}, `!(a in [1])`},
		{"string with quote and backslash", cmp("a", OpEq, str(`q"\`)), `a === "q\"\\"`},
		{"unknown literal prints nothing special", cmp("a", OpEq, Bool{Value: true}), `a === true`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := Print(tt.e)
			if got != tt.want {
				t.Errorf("Print = %q, want %q", got, tt.want)
			}
			if !Equal(mustParse(t, got), tt.e) {
				t.Errorf("Parse(%q) is not Equal to the printed tree", got)
			}
		})
	}
}

func TestPrintExamplesAreStable(t *testing.T) {
	t.Parallel()
	for _, src := range examples {
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			once := Print(mustParse(t, src))
			twice := Print(mustParse(t, once))
			if once != twice {
				t.Errorf("not idempotent:\n once  %q\n twice %q", once, twice)
			}
		})
	}
}

func TestPrintSampleBrief(t *testing.T) {
	t.Parallel()
	src := "city === \"İstanbul\" &&\ncartItems.some(c => c.addedAt >= ago(\"30d\")) &&\norders.none(o => o.orderDate >= ago(\"30d\"))"
	want := `city === "İstanbul" && cartItems.some(c => c.addedAt >= ago("30d")) && orders.none(o => o.orderDate >= ago("30d"))`
	if got := Print(mustParse(t, src)); got != want {
		t.Errorf("Print = %q, want %q", got, want)
	}
}
