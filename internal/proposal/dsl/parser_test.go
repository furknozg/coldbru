package dsl

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// examples are every example of docs/coldbru-gqdsl.md §8.2 and §13, plus the
// sample brief of CLAUDE.md, as written in the document.
var examples = []string{
	// §13 A
	`city === "İstanbul" && totalSpendGbp >= 1000`,
	// §13 B
	`city in ["İstanbul", "Ankara", "İzmir"] && loyaltyTier in ["Gold", "Platinum"]`,
	// §13 C
	`lastCartAddDate >= ago("30d") && hasOpenCart === true`,
	// §13 D
	`orders.some(o => o.totalGbp > 1000)`,
	// §13 E and §8.2: the sample brief, multi-line
	"city === \"İstanbul\" &&\ncartItems.some(c => c.addedAt >= ago(\"30d\")) &&\norders.none(o => o.orderDate >= ago(\"30d\"))",
	// §13 F
	`orders.none(o => o.orderStatus === "cancelled")`,
	// §13 G
	`orders.every(o => o.totalGbp > 0)`,
	// §3.1, §3.3, §3.4, §3.5, §3.6, §3.8
	"ageYears >= 25 &&\n(city === \"İstanbul\" || city === \"Ankara\")",
	`city === "İstanbul"`,
	`ageYears === 25`,
	`loyaltyTier !== "Bronze"`,
	`signupDate < date("2026-01-01")`,
	`retired === false`,
	"city === \"İstanbul\" &&\n(ageYears >= 25 || totalSpendGbp > 10000)",
	`!(loyaltyTier === "Bronze")`,
	// §3.9
	`a === 1 || b === 2 && c === 3`,
	// §5.6
	`orders.some(o => o.totalGbp > 1000) && orders.some(o => o.orderStatus === "delivered")`,
	"orders.some(o =>\n  o.totalGbp > 1000 &&\n  o.orderStatus === \"delivered\"\n)",
	// §5.5: nested relation, the variable qualifies the inner path
	`orders.some(o => o.items.some(i => i.quantity >= 2))`,
	// §3.10
	`orders.some(o => o.totalGbp > 1000) || cartItems.every(c => c.quantity > 0)`,
}

func TestParseExamples(t *testing.T) {
	t.Parallel()
	for _, src := range examples {
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			e := mustParse(t, src)
			checkSpans(t, src, e)
			canon := Print(e)
			again := mustParse(t, canon)
			if !Equal(e, again) {
				t.Errorf("round trip changed the tree:\n source: %q\n canon:  %q", src, canon)
			}
			if got := Print(again); got != canon {
				t.Errorf("Print is not idempotent: %q then %q", canon, got)
			}
		})
	}
}

func TestVersion(t *testing.T) {
	t.Parallel()
	if Version != "0.1" {
		t.Errorf("Version = %q, want 0.1 (D-045)", Version)
	}
}

// Builders for expected trees; Equal ignores spans.
func path(segs ...string) Path { return Path{Segments: segs} }
func str(s string) String      { return String{Value: s} }
func num(s string) Number      { return Number{Text: s} }
func cmp(field string, op Op, v Literal) *Compare {
	return &Compare{Field: path(strings.Split(field, ".")...), Op: op, Value: v}
}

func TestParseTree(t *testing.T) {
	t.Parallel()
	a, b, c := cmp("a", OpEq, num("1")), cmp("b", OpEq, num("2")), cmp("c", OpEq, num("3"))
	tests := []struct {
		name string
		src  string
		want Expr
	}{
		{"compare", `city === "İstanbul"`, cmp("city", OpEq, str("İstanbul"))},
		{"all operators", `a !== 1 && b > 1 && c >= 1 && d < 1 && e <= 1`, &And{Terms: []Expr{
			cmp("a", OpNe, num("1")), cmp("b", OpGt, num("1")), cmp("c", OpGte, num("1")), cmp("d", OpLt, num("1")), cmp("e", OpLte, num("1"))}}},
		{"and chain is one node", `a === 1 && b === 2 && c === 3`, &And{Terms: []Expr{a, b, c}}},
		{"or chain is one node", `a === 1 || b === 2 || c === 3`, &Or{Terms: []Expr{a, b, c}}},
		{"and binds tighter than or", `a === 1 || b === 2 && c === 3`, &Or{Terms: []Expr{a, &And{Terms: []Expr{b, c}}}}},
		{"and binds tighter than or, other side", `a === 1 && b === 2 || c === 3`, &Or{Terms: []Expr{&And{Terms: []Expr{a, b}}, c}}},
		{"parentheses override precedence", `(a === 1 || b === 2) && c === 3`, &And{Terms: []Expr{&Or{Terms: []Expr{a, b}}, c}}},
		{"parenthesized same-kind group is flattened", `(a === 1 && b === 2) && c === 3`, &And{Terms: []Expr{a, b, c}}},
		{"right-nested same-kind group is flattened", `a === 1 && (b === 2 && c === 3)`, &And{Terms: []Expr{a, b, c}}},
		{"or inside or is flattened", `a === 1 || (b === 2 || c === 3)`, &Or{Terms: []Expr{a, b, c}}},
		{"redundant parentheses vanish", `(((a === 1)))`, a},
		{"not binds to a group", `!(a === 1 && b === 2)`, &Not{X: &And{Terms: []Expr{a, b}}}},
		{"not not", `!!(a === 1)`, &Not{X: &Not{X: a}}},
		{"not binds tighter than and", `!(a === 1) && b === 2`, &And{Terms: []Expr{&Not{X: a}, b}}},
		{"in list", `city in ["a", 'b', 3, true, ago("1d"), date("2026-01-01")]`, &In{Field: path("city"), Values: []Literal{
			str("a"), str("b"), num("3"), Bool{Value: true}, Ago{Text: "1d"}, Date{Text: "2026-01-01"}}}},
		{"in with one value", `city in ["a"]`, &In{Field: path("city"), Values: []Literal{str("a")}}},
		{"literals", `a === -5 && b === 3.14 && c === 007 && d === false`, &And{Terms: []Expr{
			cmp("a", OpEq, num("-5")), cmp("b", OpEq, num("3.14")), cmp("c", OpEq, num("007")), cmp("d", OpEq, Bool{})}}},
		{"time literals", `a >= ago("12h") && b < date("2024-02-29")`, &And{Terms: []Expr{
			cmp("a", OpGte, Ago{Text: "12h"}), cmp("b", OpLt, Date{Text: "2024-02-29"})}}},
		{"string escapes", `a === 'it\'s' && b === "q\"x\\y"`, &And{Terms: []Expr{
			cmp("a", OpEq, str("it's")), cmp("b", OpEq, str(`q"x\y`))}}},
		{"dotted field", `a.b.c === 1`, cmp("a.b.c", OpEq, num("1"))},
		{"relation", `orders.some(o => o.totalGbp > 1000)`, &Relation{Path: path("orders"), Quantifier: Some, Var: "o", Body: cmp("o.totalGbp", OpGt, num("1000"))}},
		{"none and every", `orders.none(o => o.x === 1) || orders.every(o => o.x === 2)`, &Or{Terms: []Expr{
			&Relation{Path: path("orders"), Quantifier: None, Var: "o", Body: cmp("o.x", OpEq, num("1"))},
			&Relation{Path: path("orders"), Quantifier: Every, Var: "o", Body: cmp("o.x", OpEq, num("2"))}}}},
		{"relation body is a full expression", `orders.some(o => o.a === 1 || o.b === 2 && o.c === 3)`, &Relation{
			Path: path("orders"), Quantifier: Some, Var: "o", Body: &Or{Terms: []Expr{
				cmp("o.a", OpEq, num("1")), &And{Terms: []Expr{cmp("o.b", OpEq, num("2")), cmp("o.c", OpEq, num("3"))}}}}}},
		{"nested relation path starts with the variable", `orders.some(o => o.items.some(i => i.q > 1))`, &Relation{
			Path: path("orders"), Quantifier: Some, Var: "o", Body: &Relation{
				Path: path("o", "items"), Quantifier: Some, Var: "i", Body: cmp("i.q", OpGt, num("1"))}}},
		{"negated relation", `!orders.some(o => o.x === 1)`, &Not{X: &Relation{
			Path: path("orders"), Quantifier: Some, Var: "o", Body: cmp("o.x", OpEq, num("1"))}}},
		{"sibling lambdas may reuse a variable", `orders.some(o => o.x === 1) && cartItems.some(o => o.y === 2)`, &And{Terms: []Expr{
			&Relation{Path: path("orders"), Quantifier: Some, Var: "o", Body: cmp("o.x", OpEq, num("1"))},
			&Relation{Path: path("cartItems"), Quantifier: Some, Var: "o", Body: cmp("o.y", OpEq, num("2"))}}}},
		{"variable name is plain outside its lambda", `orders.some(o => o.x === 1) && o.y === 2`, &And{Terms: []Expr{
			&Relation{Path: path("orders"), Quantifier: Some, Var: "o", Body: cmp("o.x", OpEq, num("1"))},
			cmp("o.y", OpEq, num("2"))}}},
		{"no spaces needed", `a===1&&b!==2||c>=3`, &Or{Terms: []Expr{&And{Terms: []Expr{
			cmp("a", OpEq, num("1")), cmp("b", OpNe, num("2"))}}, cmp("c", OpGte, num("3"))}}},
		{"newlines and tabs are whitespace", "a\t===\r\n1\n&&\nb === 2", &And{Terms: []Expr{a, b}}},
		{"keyword-like field names", `inbox === 1 && dateOfX === 2 && someone === 3 && agoX === 4`, &And{Terms: []Expr{
			cmp("inbox", OpEq, num("1")), cmp("dateOfX", OpEq, num("2")), cmp("someone", OpEq, num("3")), cmp("agoX", OpEq, num("4"))}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := mustParse(t, tt.src)
			if !Equal(got, tt.want) {
				t.Errorf("Parse(%q)\n got  %s\n want %s", tt.src, Print(got), Print(tt.want))
			}
			checkSpans(t, tt.src, got)
		})
	}
}

func TestParseFlattenedTermCounts(t *testing.T) {
	t.Parallel()
	and, ok := mustParse(t, `a === 1 && (b === 2 && (c === 3 && d === 4)) && e === 5`).(*And)
	if !ok || len(and.Terms) != 5 {
		t.Fatalf("got %#v, want one And with 5 terms", and)
	}
	or, ok := mustParse(t, `(a === 1 || b === 2) || c === 3`).(*Or)
	if !ok || len(or.Terms) != 3 {
		t.Fatalf("got %#v, want one Or with 3 terms", or)
	}
}

func TestParseSpans(t *testing.T) {
	t.Parallel()
	src := "city === \"İstanbul\" &&\n  cartItems.some(c => c.addedAt >= ago(\"30d\"))"
	and := mustParse(t, src).(*And)
	cmpNode := and.Terms[0].(*Compare)
	rel := and.Terms[1].(*Relation)
	body := rel.Body.(*Compare)
	tests := []struct {
		name       string
		got        Span
		start, end string
	}{
		{"And", and.Span, "1:1/0", "2:47/70"},
		{"Compare", cmpNode.Span, "1:1/0", "1:20/20"},
		{"Compare field", cmpNode.Field.Span, "1:1/0", "1:5/4"},
		{"Compare value", cmpNode.Value.Extent(), "1:10/9", "1:20/20"},
		{"Relation", rel.Span, "2:3/26", "2:47/70"},
		{"Relation path", rel.Path.Span, "2:3/26", "2:12/35"},
		{"Relation var", rel.VarSpan, "2:18/41", "2:19/42"},
		{"body", body.Span, "2:23/46", "2:46/69"},
		{"body value", body.Value.Extent(), "2:36/59", "2:46/69"},
	}
	for _, tt := range tests {
		if got := fmtPosOff(tt.got.Start); got != tt.start {
			t.Errorf("%s start = %s, want %s", tt.name, got, tt.start)
		}
		if got := fmtPosOff(tt.got.End); got != tt.end {
			t.Errorf("%s end = %s, want %s", tt.name, got, tt.end)
		}
	}
}

func TestParseSpansIncludeParentheses(t *testing.T) {
	t.Parallel()
	not := mustParse(t, `!(a === 1)`).(*Not)
	if got := fmtPosOff(not.Start) + "-" + fmtPosOff(not.End); got != "1:1/0-1:11/10" {
		t.Errorf("Not span = %s", got)
	}
	inner := not.X.Extent()
	if got := fmtPosOff(inner.Start) + "-" + fmtPosOff(inner.End); got != "1:2/1-1:11/10" {
		t.Errorf("parenthesized operand span = %s, want it to include the parentheses", got)
	}
}

// errCase is one parser error expectation. from and to are the line:col of
// Start and End; msg is a substring of the message.
type errCase struct {
	name string
	src  string
	code Code
	from string
	to   string
	msg  string
}

func runErrCases(t *testing.T, tests []errCase) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := parseErr(t, tt.src)
			if e.Code != tt.code {
				t.Errorf("code = %s, want %s (message: %s)", e.Code, tt.code, e.Message)
			}
			if got := fmtPos(e.Start); got != tt.from {
				t.Errorf("start = %s, want %s (message: %s)", got, tt.from, e.Message)
			}
			if got := fmtPos(e.End); got != tt.to {
				t.Errorf("end = %s, want %s (message: %s)", got, tt.to, e.Message)
			}
			if !strings.Contains(e.Message, tt.msg) {
				t.Errorf("message = %q, want it to contain %q", e.Message, tt.msg)
			}
			if e.Start != posAt(tt.src, e.Start.Offset) || e.End != posAt(tt.src, e.End.Offset) {
				t.Errorf("positions %v-%v disagree with their offsets", e.Start, e.End)
			}
		})
	}
}

func TestParseSyntaxErrors(t *testing.T) {
	t.Parallel()
	runErrCases(t, []errCase{
		{"empty", "", CodeSyntaxError, "1:1", "1:1", "unexpected end"},
		{"blank", "  \n ", CodeSyntaxError, "2:2", "2:2", "unexpected end"},
		{"dangling and", "a === 1 &&", CodeSyntaxError, "1:11", "1:11", "unexpected end"},
		{"dangling comparison", "a ===", CodeSyntaxError, "1:6", "1:6", "literal value"},
		{"two conditions", "a === 1 b === 2", CodeSyntaxError, "1:9", "1:10", "unexpected identifier `b`"},
		{"chained comparison", "a === 1 === 2", CodeSyntaxError, "1:9", "1:12", "cannot be chained"},
		{"comparison on a group", "(a === 1) === true", CodeSyntaxError, "1:11", "1:14", "cannot be chained or applied to a group"},
		{"comparison on a relation", "orders.some(o => o.x > 1) === true", CodeSyntaxError, "1:27", "1:30", "relation"},
		{"unclosed paren", "(a === 1", CodeSyntaxError, "1:9", "1:9", "`)` to close the group"},
		{"unmatched close paren", "a === 1)", CodeSyntaxError, "1:8", "1:9", "unmatched `)`"},
		{"empty parens", "()", CodeSyntaxError, "1:2", "1:3", "unexpected `)`"},
		{"missing close paren before and", "(a === 1 && b === 2", CodeSyntaxError, "1:20", "1:20", "`)`"},
		{"bare boolean field", "hasOpenCart", CodeSyntaxError, "1:1", "1:12", "hasOpenCart === true"},
		{"bare boolean in conjunction", "a === 1 && b", CodeSyntaxError, "1:12", "1:13", "b === true"},
		{"bare boolean in group", "!(a === 1) && (b)", CodeSyntaxError, "1:16", "1:17", "b === true"},
		{"negated bare boolean", "!retired", CodeSyntaxError, "1:1", "1:9", "retired === false"},
		{"negated bare boolean in conjunction", "a === 1 && !retired", CodeSyntaxError, "1:12", "1:20", "retired === false"},
		{"bare dotted field", "a.b", CodeSyntaxError, "1:1", "1:4", "a.b === true"},
		{"not applied to a comparison", "!a === 1", CodeSyntaxError, "1:1", "1:7", "wrap the comparison in parentheses"},
		{"not applied to in", "!city in [\"a\"]", CodeSyntaxError, "1:1", "1:9", "!(city in ...)"},
		{"literal on the left", "1000 < totalSpendGbp", CodeSyntaxError, "1:1", "1:5", "totalSpendGbp > 1000"},
		{"literal on the left, equality", "\"x\" === city", CodeSyntaxError, "1:1", "1:4", `city === "x"`},
		{"literal on the left, time", "ago(\"30d\") >= signupDate", CodeSyntaxError, "1:1", "1:11", `signupDate <= ago("30d")`},
		{"literal on the left of a call", "5 > f(1)", CodeSyntaxError, "1:1", "1:2", "put the field on the left"},
		{"literal on the left of in", "\"a\" in [\"a\"]", CodeSyntaxError, "1:1", "1:4", "left of `in`"},
		{"literal as a condition", "a === 1 && true", CodeSyntaxError, "1:12", "1:16", "not a condition"},
		{"empty in list", "city in []", CodeSyntaxError, "1:9", "1:11", "at least one literal"},
		{"empty in list with spaces", "city in [  ]", CodeSyntaxError, "1:9", "1:13", "at least one literal"},
		{"trailing comma in list", "city in [1,]", CodeSyntaxError, "1:12", "1:13", "trailing comma"},
		{"in without list", "city in \"a\"", CodeSyntaxError, "1:9", "1:12", "a list after `in`"},
		{"in with field list", "city in other", CodeSyntaxError, "1:9", "1:14", "a list after `in`"},
		{"in list holds a field", "city in [a]", CodeSyntaxError, "1:10", "1:11", "`a` is a field"},
		{"nested list", "city in [[1]]", CodeSyntaxError, "1:10", "1:11", "nested lists"},
		{"list missing comma", "city in [1 2]", CodeSyntaxError, "1:12", "1:13", "`,` or `]`"},
		{"unclosed list", "city in [1", CodeSyntaxError, "1:11", "1:11", "`,` or `]`"},
		{"list after a comparison", "city === [1]", CodeSyntaxError, "1:10", "1:11", "only allowed after `in`"},
		{"group after a comparison", "a > (1)", CodeSyntaxError, "1:5", "1:6", "literal value"},
		{"not after a comparison", "a === !b", CodeSyntaxError, "1:7", "1:8", "literal value"},
		{"null literal", "x === null", CodeSyntaxError, "1:7", "1:11", "no `null` literal"},
		{"undefined literal", "x !== undefined", CodeSyntaxError, "1:7", "1:16", "no `undefined` literal"},
		{"reserved word as field", "some === 1", CodeSyntaxError, "1:1", "1:5", "reserved word"},
		{"in as field", "in === 1", CodeSyntaxError, "1:1", "1:3", "reserved word"},
		{"reserved word after dot", "o.in === 1", CodeSyntaxError, "1:3", "1:5", "`in` is a reserved word"},
		{"true after dot", "a.true === 1", CodeSyntaxError, "1:3", "1:7", "`true` is a reserved word"},
		{"nothing after dot", "a. === 1", CodeSyntaxError, "1:4", "1:7", "expected a field name"},
		{"quantifier not called", "orders.some && x === 1", CodeSyntaxError, "1:8", "1:12", "must be called"},
		{"date used as a field", "a === 1 && date === 2", CodeSyntaxError, "1:12", "1:16", "must be called"},
		{"lambda without variable", "orders.some()", CodeSyntaxError, "1:13", "1:14", "needs a lambda"},
		{"lambda with parenthesized variable", "orders.some((o) => o.x > 1)", CodeSyntaxError, "1:13", "1:14", "without parentheses"},
		{"lambda with two variables", "orders.some(o, p => o.x > 1)", CodeSyntaxError, "1:14", "1:15", "exactly one variable"},
		{"lambda without arrow", "orders.some(o)", CodeSyntaxError, "1:14", "1:15", "`=>` after the lambda variable `o`"},
		{"lambda with non-name", "orders.some(1 => 1)", CodeSyntaxError, "1:13", "1:14", "lambda variable"},
		{"lambda variable in", "orders.some(in => in.x > 1)", CodeSyntaxError, "1:13", "1:15", "reserved word"},
		{"lambda variable true", "orders.some(true => 1)", CodeSyntaxError, "1:13", "1:17", "reserved word"},
		{"lambda variable ago", "orders.some(ago => 1)", CodeSyntaxError, "1:13", "1:16", "reserved word"},
		{"lambda variable some", "orders.some(some => 1)", CodeSyntaxError, "1:13", "1:17", "reserved word"},
		{"lambda with empty body", "orders.some(o => )", CodeSyntaxError, "1:18", "1:19", "condition"},
		{"unclosed lambda", "orders.some(o => o.x > 1", CodeSyntaxError, "1:25", "1:25", "`)` to close .some("},
		{"indexing", "orders[0].x === 1", CodeSyntaxError, "1:7", "1:8", "comparison operator"},
		{"unknown character", "a === 1 && $", CodeSyntaxError, "1:12", "1:13", "unexpected character '$'"},
		{"lexical error beats later tokens", "a === 'x\nb === 1", CodeSyntaxError, "1:7", "1:9", "unterminated string"},
		{"first error wins", "a === 1e3 && b ~= 'x'", CodeSyntaxError, "1:7", "1:9", "malformed number"},
	})
}

func TestParseMultiLineAndRuneColumns(t *testing.T) {
	t.Parallel()
	runErrCases(t, []errCase{
		{"second line", "a === 1 &&\n  b == 2", CodeUnsupportedOperator, "2:5", "2:7", "use === for equality"},
		{"third line after blank", "a === 1 &&\n\n  b === 2 $", CodeSyntaxError, "3:11", "3:12", "unexpected character '$'"},
		{"carriage return line break", "a === 1 &&\r\n  b == 2", CodeUnsupportedOperator, "2:5", "2:7", "use ==="},
		{"turkish string before the error", `sehir === "İstanbul" && x == 1`, CodeUnsupportedOperator, "1:27", "1:29", "use === for equality"},
		{"turkish letters in a string on the line above", "name === \"Şişli İğdır\" &&\n  ğ === 1", CodeSyntaxError, "2:3", "2:4", "unexpected character 'ğ'"},
		{"turkish ident char", `city === "İstanbul" && cıty === 1`, CodeSyntaxError, "1:25", "1:26", "unexpected character 'ı'"},
	})
	// Byte offsets still count bytes: İ is two bytes but one column.
	e := parseErr(t, `sehir === "İstanbul" && x == 1`)
	if e.Start.Offset != 27 || e.End.Offset != 29 {
		t.Errorf("offsets = %d-%d, want 27-29 (columns are runes, offsets are bytes)", e.Start.Offset, e.End.Offset)
	}
	if e.Near != "==" {
		t.Errorf("near = %q, want ==", e.Near)
	}
}

func TestParseUnsupportedOperators(t *testing.T) {
	t.Parallel()
	runErrCases(t, []errCase{
		{"tilde equals", `favouriteCategory ~= "electronics"`, CodeUnsupportedOperator, "1:19", "1:21", "`~=` (text search) is reserved"},
		{"loose equality", "x == 1", CodeUnsupportedOperator, "1:3", "1:5", "use === for equality"},
		{"loose inequality", "x != 1", CodeUnsupportedOperator, "1:3", "1:5", "use !== for inequality"},
		{"field to field", "totalSpendGbp > openCartValueGbp", CodeUnsupportedOperator, "1:17", "1:33", "`openCartValueGbp` is a field"},
		{"field to field equality", "a === b", CodeUnsupportedOperator, "1:7", "1:8", "comparing two fields is reserved"},
		{"dotted field to field", "a === b.c", CodeUnsupportedOperator, "1:7", "1:10", "`b.c` is a field"},
		{"field to field inside a lambda", "orders.some(o => o.a > o.b)", CodeUnsupportedOperator, "1:24", "1:27", "`o.b` is a field"},
		{"operator after a group on the left is not unsupported", "x ~= 1 && y === 2", CodeUnsupportedOperator, "1:3", "1:5", "reserved"},
	})
}

func TestParseUnsupportedExpressions(t *testing.T) {
	t.Parallel()
	runErrCases(t, []errCase{
		{"function call", `fetch("https://example.com")`, CodeUnsupportedExpression, "1:1", "1:7", "function call `fetch(...)`"},
		{"eval", `eval("some code")`, CodeUnsupportedExpression, "1:1", "1:6", "function call `eval(...)`"},
		{"map", "orders.map(o => o.totalGbp)", CodeUnsupportedExpression, "1:1", "1:12", "method call `.map(...)`"},
		{"reduce", "orders.reduce((a, b) => a + b.totalGbp, 0)", CodeUnsupportedExpression, "1:1", "1:15", "method call `.reduce(...)`"},
		{"filter inside a comparison", "x === orders.filter(1)", CodeUnsupportedExpression, "1:7", "1:21", "method call `.filter(...)`"},
		{"call on the right", "x === foo(1)", CodeUnsupportedExpression, "1:7", "1:11", "function call `foo(...)`"},
		{"method on a literal", `x === "a".toUpperCase()`, CodeUnsupportedExpression, "1:10", "1:11", "methods on values"},
		{"assignment", "customer.loyaltyPoints = 9999", CodeUnsupportedExpression, "1:24", "1:25", "assignment is not supported"},
		{"assignment at start", "= 1", CodeUnsupportedExpression, "1:1", "1:2", "assignment is not supported"},
		{"arrow function at top level", "o => o.x > 1", CodeUnsupportedExpression, "1:3", "1:5", "only allowed inside"},
		{"relation as a value", "x === orders.some(o => o.x > 1)", CodeUnsupportedExpression, "1:7", "1:14", "relation predicate cannot be used as a value"},
		{"call in a lambda body", "orders.some(o => o.items.length() > 1)", CodeUnsupportedExpression, "1:18", "1:33", "method call `.length(...)`"},
	})
}

func TestParseRelationScope(t *testing.T) {
	t.Parallel()
	runErrCases(t, []errCase{
		{"bare root field in body", "orders.some(o => total > 1)", CodeInvalidRelationScope, "1:18", "1:23", "write `o.total`"},
		{"bare field after a valid term", `orders.some(o => o.x > 1 && city === "a")`, CodeInvalidRelationScope, "1:29", "1:33", "write `o.city`"},
		{"bare field in an in-test", `orders.some(o => city in ["a"])`, CodeInvalidRelationScope, "1:18", "1:22", "write `o.city`"},
		{"bare relation in body", "orders.some(o => orders.some(p => p.x > 1))", CodeInvalidRelationScope, "1:18", "1:24", "write `o.orders`"},
		{"variable alone", "orders.some(o => o > 1)", CodeInvalidRelationScope, "1:18", "1:19", "not a field"},
		{"another variable name", "orders.some(o => p.x > 1)", CodeInvalidRelationScope, "1:18", "1:21", "write `o.p.x`"},
		{"outer variable in nested body", "orders.some(o => o.a.some(p => o.x > 1))", CodeInvalidRelationScope, "1:32", "1:35", "enclosing relation"},
		{"nested lambda reuses the variable", "orders.some(o => o.a.some(o => o.x > 1))", CodeInvalidRelationScope, "1:27", "1:28", "already used"},
		{"three levels reuse outermost", "a.some(x => x.b.some(y => y.c.some(x => x.d > 1)))", CodeInvalidRelationScope, "1:36", "1:37", "already used"},
		{"inner relation path uses outer variable", "a.some(x => x.b.some(y => x.c.some(z => z.d > 1)))", CodeInvalidRelationScope, "1:27", "1:30", "enclosing relation"},
	})
}

func TestParseTimeLiterals(t *testing.T) {
	t.Parallel()
	valid := []string{
		`a >= ago("1d")`, `a >= ago("30d")`, `a >= ago("12h")`, `a >= ago("36500d")`, `a >= ago("876000h")`, `a >= ago("36500h")`,
		`a >= ago( "30d" )`, `a >= ago('30d')`,
		`a < date("2026-01-01")`, `a < date("2024-02-29")`, `a < date("2000-02-29")`, `a < date("2026-12-31")`, `a < date('2026-04-30')`,
	}
	for _, src := range valid {
		t.Run(src, func(t *testing.T) {
			t.Parallel()
			mustParse(t, src)
		})
	}
	runErrCases(t, []errCase{
		{"ago zero", `a >= ago("0d")`, CodeSyntaxError, "1:10", "1:14", `ago("0d") is invalid`},
		{"ago leading zero", `a >= ago("030d")`, CodeSyntaxError, "1:10", "1:16", "is invalid"},
		{"ago negative", `a >= ago("-1d")`, CodeSyntaxError, "1:10", "1:15", "is invalid"},
		{"ago uppercase unit", `a >= ago("30D")`, CodeSyntaxError, "1:10", "1:15", "is invalid"},
		{"ago unsupported unit", `a >= ago("2w")`, CodeSyntaxError, "1:10", "1:14", "d (days) or h (hours)"},
		{"ago minutes", `a >= ago("30m")`, CodeSyntaxError, "1:10", "1:15", "is invalid"},
		{"ago no unit", `a >= ago("30")`, CodeSyntaxError, "1:10", "1:14", "is invalid"},
		{"ago unit only", `a >= ago("d")`, CodeSyntaxError, "1:10", "1:13", "is invalid"},
		{"ago empty", `a >= ago("")`, CodeSyntaxError, "1:10", "1:12", "is invalid"},
		{"ago fraction", `a >= ago("1.5d")`, CodeSyntaxError, "1:10", "1:16", "is invalid"},
		{"ago inner space", `a >= ago("30 d")`, CodeSyntaxError, "1:10", "1:16", "is invalid"},
		{"ago outer space", `a >= ago(" 30d")`, CodeSyntaxError, "1:10", "1:16", "is invalid"},
		{"ago too many days", `a >= ago("36501d")`, CodeSyntaxError, "1:10", "1:18", "maximum is 36500d or 876000h"},
		{"ago too many hours", `a >= ago("876001h")`, CodeSyntaxError, "1:10", "1:19", "maximum is 36500d or 876000h"},
		{"ago huge number does not overflow", `a >= ago("99999999999999999999999d")`, CodeSyntaxError, "1:10", "1:36", "too far back"},
		{"ago number argument", `a >= ago(30)`, CodeSyntaxError, "1:10", "1:12", "exactly one string argument"},
		{"ago no argument", `a >= ago()`, CodeSyntaxError, "1:10", "1:11", "exactly one string argument"},
		{"ago two arguments", `a >= ago("30d", 1)`, CodeSyntaxError, "1:15", "1:16", "exactly one argument"},
		{"ago unclosed", `a >= ago("30d"`, CodeSyntaxError, "1:15", "1:15", "`)` to close ago("},
		{"ago not called", `a >= ago`, CodeSyntaxError, "1:6", "1:9", "must be called"},
		{"ago field argument", `a >= ago(b)`, CodeSyntaxError, "1:10", "1:11", "exactly one string argument"},
		{"date impossible day", `a < date("2025-02-29")`, CodeSyntaxError, "1:10", "1:22", "real calendar date"},
		{"date april 31", `a < date("2026-04-31")`, CodeSyntaxError, "1:10", "1:22", "real calendar date"},
		{"date month 13", `a < date("2026-13-01")`, CodeSyntaxError, "1:10", "1:22", "real calendar date"},
		{"date month 0", `a < date("2026-00-10")`, CodeSyntaxError, "1:10", "1:22", "real calendar date"},
		{"date day 0", `a < date("2026-01-00")`, CodeSyntaxError, "1:10", "1:22", "real calendar date"},
		{"date short parts", `a < date("2026-1-1")`, CodeSyntaxError, "1:10", "1:20", "real calendar date"},
		{"date two digit year", `a < date("26-01-01")`, CodeSyntaxError, "1:10", "1:20", "real calendar date"},
		{"date slashes", `a < date("2026/01/01")`, CodeSyntaxError, "1:10", "1:22", "real calendar date"},
		{"date with time", `a < date("2026-01-01T00:00:00Z")`, CodeSyntaxError, "1:10", "1:32", "real calendar date"},
		{"date non-digits", `a < date("2026-0a-01")`, CodeSyntaxError, "1:10", "1:22", "real calendar date"},
		{"date number argument", `a < date(20260101)`, CodeSyntaxError, "1:10", "1:18", "exactly one string argument"},
		{"date not called", `a < date`, CodeSyntaxError, "1:5", "1:9", "must be called"},
		{"ago on the right of in with a bad argument", `a in [ago("0d")]`, CodeSyntaxError, "1:11", "1:15", "is invalid"},
	})
}

func TestParseInList(t *testing.T) {
	t.Parallel()
	values := func(n int) string {
		parts := make([]string, n)
		for i := range parts {
			parts[i] = fmt.Sprintf("%d", i)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	in, ok := mustParse(t, "city in "+values(MaxInValues)).(*In)
	if !ok || len(in.Values) != MaxInValues {
		t.Fatalf("an in-list of %d values must parse", MaxInValues)
	}
	src := "city in " + values(MaxInValues+1)
	e := parseErr(t, src)
	if e.Code != CodeComplexityLimit {
		t.Fatalf("code = %s, want %s", e.Code, CodeComplexityLimit)
	}
	// The error points at the 101st value, the first one over the limit.
	if want := strings.Index(src, "100"); e.Start.Offset != want || e.End.Offset != want+3 {
		t.Errorf("error at %d-%d, want the 101st value at %d-%d", e.Start.Offset, e.End.Offset, want, want+3)
	}
	if !strings.Contains(e.Message, "more than 100 values") {
		t.Errorf("message = %q", e.Message)
	}
}

func TestParseSourceLimit(t *testing.T) {
	t.Parallel()
	pad := func(total int) string {
		prefix, suffix := `a === "`, `"`
		return prefix + strings.Repeat("x", total-len(prefix)-len(suffix)) + suffix
	}
	if src := pad(MaxSourceBytes); len(src) != 4096 {
		t.Fatalf("test setup: len = %d", len(src))
	} else {
		mustParse(t, src)
	}
	src := pad(MaxSourceBytes + 1)
	e := parseErr(t, src)
	if e.Code != CodeComplexityLimit {
		t.Fatalf("code = %s, want %s", e.Code, CodeComplexityLimit)
	}
	if got := fmtPos(e.Start); got != "1:1" || !strings.Contains(e.Message, "4097 bytes") {
		t.Errorf("error = %v (start %s)", e, got)
	}
	// The limit is checked before lexing: invalid text does not change the code.
	if e := parseErr(t, strings.Repeat("$", 5000)); e.Code != CodeComplexityLimit {
		t.Errorf("oversized invalid source: code = %s, want %s", e.Code, CodeComplexityLimit)
	}
	// Bytes, not runes: 2049 two-byte runes in a string exceed 4096 bytes.
	multi := `a === "` + strings.Repeat("İ", 2045) + `"`
	if len(multi) <= MaxSourceBytes {
		t.Fatalf("test setup: %d bytes", len(multi))
	}
	if e := parseErr(t, multi); e.Code != CodeComplexityLimit {
		t.Errorf("multi-byte source: code = %s, want %s", e.Code, CodeComplexityLimit)
	}
}

// nested builds `levels` wrappers around a single comparison, alternating
// Or and And so that nothing flattens: the comparison ends up at depth levels+1.
func nested(levels int) string {
	s := "a === 1"
	for i := range levels {
		if i%2 == 0 {
			s = "b === 1 && (" + s + ")"
		} else {
			s = "b === 1 || (" + s + ")"
		}
	}
	return s
}

// relationChain builds n nested relations; the comparison is at depth n+1.
func relationChain(n int) string {
	vars := "abcdefghijklmnop"
	var open, closing strings.Builder
	prev := ""
	for i := range n {
		p := "orders"
		if prev != "" {
			p = prev + ".items"
		}
		fmt.Fprintf(&open, "%s.some(%c => ", p, vars[i])
		closing.WriteString(")")
		prev = string(vars[i])
	}
	return open.String() + prev + ".q > 1" + closing.String()
}

func TestParseDepthLimit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		src     string
		wantErr bool
		offset  int // expected error start offset
	}{
		{"flat", "a === 1", false, 0},
		{"alternating and/or, comparison at depth 8", nested(7), false, 0},
		{"alternating and/or, comparison at depth 9", nested(8), true, strings.LastIndex(nested(8), "b === 1")},
		{"not chain, comparison at depth 8", strings.Repeat("!", 7) + "(a === 1)", false, 0},
		{"not chain, comparison at depth 9", strings.Repeat("!", 8) + "(a === 1)", true, 8},
		{"relation chain, comparison at depth 8", relationChain(7), false, 0},
		{"relation chain, comparison at depth 9", relationChain(8), true, strings.Index(relationChain(8), "h.q")},
		{"parentheses add no depth", strings.Repeat("(", 30) + "a === 1" + strings.Repeat(")", 30), false, 0},
		{"same-kind groups are flattened and add no depth", "a === 1 && (b === 1 && (c === 1 && (d === 1 && (e === 1 && (f === 1 && (g === 1 && (h === 1 && (i === 1 && j === 1))))))))", false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !tt.wantErr {
				mustParse(t, tt.src)
				return
			}
			e := parseErr(t, tt.src)
			if e.Code != CodeComplexityLimit || !strings.Contains(e.Message, "deeper than 8") {
				t.Errorf("error = %v, want COMPLEXITY_LIMIT about depth", e)
			}
			if e.Start.Offset != tt.offset {
				t.Errorf("error offset = %d, want %d (the first node past depth 8)", e.Start.Offset, tt.offset)
			}
		})
	}
}

func andOf(n int) string {
	terms := make([]string, n)
	for i := range terms {
		terms[i] = fmt.Sprintf("f%d === %d", i, i)
	}
	return strings.Join(terms, " && ")
}

func TestParseNodeLimit(t *testing.T) {
	t.Parallel()
	relation := "orders.some(o => o.q > 1)" // 2 nodes: the quantifier and the comparison
	tests := []struct {
		name    string
		src     string
		wantErr bool
	}{
		{"1 and + 99 comparisons = 100 nodes", andOf(99), false},
		{"1 and + 100 comparisons = 101 nodes", andOf(100), true},
		{"a long in-list is one node", andOf(98) + " && city in [" + strings.TrimSuffix(strings.Repeat("1, ", 100), ", ") + "]", false},
		{"in-lists count once each", andOf(99) + " && city in [1]", true},
		{"each relation counts quantifier and body", "(" + strings.TrimSuffix(strings.Repeat(relation+" && ", 49), " && ") + ") && a === 1", false},
		{"relation quantifiers are counted", "(" + strings.TrimSuffix(strings.Repeat(relation+" && ", 49), " && ") + ") && a === 1 && b === 1", true},
		{"not counts one", "!(" + andOf(98) + ")", false},
		{"not counts one, over", "!(" + andOf(99) + ")", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !tt.wantErr {
				mustParse(t, tt.src)
				return
			}
			e := parseErr(t, tt.src)
			if e.Code != CodeComplexityLimit || !strings.Contains(e.Message, "more than 100 condition nodes") {
				t.Errorf("error = %v, want COMPLEXITY_LIMIT about nodes", e)
			}
		})
	}
	// The error points at the first node past the limit: the last comparison.
	src := andOf(100)
	e := parseErr(t, src)
	if want := strings.LastIndex(src, "f99"); e.Start.Offset != want {
		t.Errorf("error offset = %d, want %d (the 101st node)", e.Start.Offset, want)
	}
}

func TestParseErrorsAreTyped(t *testing.T) {
	t.Parallel()
	_, err := Parse("a ===")
	var pe *Error
	if !errors.As(err, &pe) {
		t.Fatalf("error %T is not *Error", err)
	}
	e, err := Parse("a === 1")
	if err != nil || e == nil {
		t.Fatalf("Parse = %v, %v", e, err)
	}
	if e2, err := Parse("a ==="); e2 != nil || err == nil {
		t.Errorf("failed Parse must return a nil Expr and an error, got %v, %v", e2, err)
	}
}

// FuzzParse checks the parser contract on arbitrary input: it never panics;
// every error has a known code and consistent positions; every success
// round-trips through Print.
func FuzzParse(f *testing.F) {
	for _, s := range examples {
		f.Add(s)
	}
	for _, s := range []string{
		``, ` `, `(`, `)`, `!`, `!!`, `((((`, `a`, `a ===`, `a === 1 &&`, `a === 1 ||`, `a === `, `=== 1`,
		`x === 'abc`, `x === "a\`, `x === 'a\q'`, `x === 1e3`, `x === +1`, `x === -`, `x === 1.`, `x === .5`,
		"x === 'a\nb'", "a\u00a0=== 1", "\xff", "a === '\xff'", `ş === 1`, `city === "İstanbul" && cıty === 1`,
		`1000 < totalSpendGbp`, `hasOpenCart`, `!retired`, `!a === 1`, `city in []`, `city in [1,]`, `city in [[1]]`, `city in [`,
		`fetch("x")`, `eval("some code")`, `orders.map(o => o.x)`, `orders.reduce((a, b) => a + b.x, 0)`, `a.b.c(`, `x = 1`, `o => o.x`,
		`x ~= "a"`, `x == 1`, `x != 1`, `a === b`, `a === b.c`, `a === null`, `x === "a".b()`,
		`orders.some`, `orders.some(`, `orders.some()`, `orders.some(o`, `orders.some(o =>`, `orders.some(o => )`, `orders.some((o) => 1)`,
		`orders.some(o, p => 1)`, `orders.some(in => 1)`, `orders.some(o => x > 1)`, `orders.some(o => o.a.some(o => o.x > 1))`,
		`orders.some(o => o.a.some(p => o.x > 1))`, `orders.some(o => o > 1)`, `a.some(x => x.b.some(y => y.c.some(z => z.d)))`,
		`a >= ago`, `a >= ago(`, `a >= ago()`, `a >= ago(30)`, `a >= ago("0d")`, `a >= ago("36501d")`, `a >= ago("99999999999999999999d")`,
		`a < date("2025-02-29")`, `a < date("2026-1-1")`, `a < date(`, `ago("30d") > a`, `a === ago("30d").x`,
		`some === 1`, `o.in === 1`, `a. === 1`, `a..b === 1`, `a === 1 === 2`, `(a === 1) === true`, `a === 1)`,
		"a === 1 &&\n\n  b === \"ğü\" $", "a === 1\r\n&&\r\nb == 2",
		`a===1&&b!==2||c>=3`, `!!!!!!!!(a === 1)`, `a === "\\" && b === '\''`, `a === "\"" `,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		e, err := Parse(src)
		if err != nil {
			if e != nil {
				t.Fatalf("Parse returned both a tree and an error for %q", src)
			}
			checkParseError(t, src, err)
			return
		}
		if e == nil {
			t.Fatalf("Parse returned nil, nil for %q", src)
		}
		checkSpans(t, src, e)
		canon := Print(e)
		if len(canon) > MaxSourceBytes {
			return // the canonical form can be longer than the source (spaces, escapes)
		}
		again, err := Parse(canon)
		if err != nil {
			t.Fatalf("Print output does not parse:\n source: %q\n canon:  %q\n error:  %v", src, canon, err)
		}
		if !Equal(again, e) {
			t.Fatalf("Parse(Print(x)) differs from x:\n source: %q\n canon:  %q\n again:  %q", src, canon, Print(again))
		}
		if got := Print(again); got != canon {
			t.Fatalf("Print is not idempotent: %q then %q", canon, got)
		}
	})
}

func checkParseError(t *testing.T, src string, err error) {
	t.Helper()
	var pe *Error
	if !errors.As(err, &pe) {
		t.Fatalf("error %T for %q is not *Error", err, src)
	}
	switch pe.Code {
	case CodeSyntaxError, CodeUnsupportedOperator, CodeUnsupportedExpression, CodeInvalidRelationScope, CodeComplexityLimit:
	default:
		t.Fatalf("unexpected code %q for %q", pe.Code, src)
	}
	if pe.Message == "" {
		t.Errorf("empty message for %q", src)
	}
	if pe.Start.Offset < 0 || pe.Start.Offset > pe.End.Offset || pe.End.Offset > len(src) {
		t.Fatalf("offsets %d-%d outside source of %d bytes (%q): %v", pe.Start.Offset, pe.End.Offset, len(src), src, pe)
	}
	if pe.Start != posAt(src, pe.Start.Offset) || pe.End != posAt(src, pe.End.Offset) {
		t.Errorf("positions %+v-%+v disagree with their offsets for %q", pe.Start, pe.End, src)
	}
	if !validUTF8(pe.Near) || strings.ContainsAny(pe.Near, "\r\n") {
		t.Errorf("near %q is not a single valid line", pe.Near)
	}
	if !strings.HasPrefix(pe.Error(), string(pe.Code)+" at ") {
		t.Errorf("Error() = %q", pe.Error())
	}
}
