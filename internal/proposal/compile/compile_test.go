package compile

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/furknozg/coldbru/internal/proposal/dsl"
)

// sampleBrief is the DSL of the sample brief (docs/coldbru-gqdsl.md §8.2).
const sampleBrief = `city === "İstanbul" &&
cartItems.some(c => c.addedAt >= ago("30d")) &&
orders.none(o => o.orderDate >= ago("30d"))`

// sampleJSON is the normalized filter variable of §8.2.
const sampleJSON = `{
  "and": [
    { "city": { "eq": "İstanbul" } },
    { "cartItems": { "some": { "addedAt": { "gte": { "ago": "30d" } } } } },
    { "orders": { "none": { "orderDate": { "gte": { "ago": "30d" } } } } }
  ]
}`

func mustCompile(t *testing.T, src string) *Filter {
	t.Helper()
	f, errs := Compile(src)
	if len(errs) > 0 {
		t.Fatalf("Compile(%q) errors: %v", src, errs)
	}
	return f
}

// sameJSON fails unless got encodes to the same JSON value as want.
func sameJSON(t *testing.T, got *Filter, want string) {
	t.Helper()
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var g, w any
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, b)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("bad want: %v", err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("filter mismatch\n got: %s\nwant: %s", b, want)
	}
}

func TestCompileSampleBriefGolden(t *testing.T) {
	sameJSON(t, mustCompile(t, sampleBrief), sampleJSON)
}

func TestCompileGolden(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"single condition root is not wrapped", `city === "Ankara"`, `{"city":{"eq":"Ankara"}}`},
		{"eq", `city === "Ankara"`, `{"city":{"eq":"Ankara"}}`},
		{"ne", `city !== "Ankara"`, `{"city":{"ne":"Ankara"}}`},
		{"gt", `totalOrders > 3`, `{"totalOrders":{"gt":3}}`},
		{"gte", `totalOrders >= 3`, `{"totalOrders":{"gte":3}}`},
		{"lt", `totalOrders < 3`, `{"totalOrders":{"lt":3}}`},
		{"lte", `totalOrders <= 3`, `{"totalOrders":{"lte":3}}`},
		{"in strings", `loyaltyTier in ["Gold", "Platinum"]`, `{"loyaltyTier":{"in":["Gold","Platinum"]}}`},
		{"in ints", `sessions90d in [1, 2, 30]`, `{"sessions90d":{"in":[1,2,30]}}`},
		{"in decimals", `totalSpendGbp in [10, 20.5]`, `{"totalSpendGbp":{"in":["10","20.5"]}}`},
		{"in dates", `signupDate in [date("2026-01-01"), date("2026-02-01")]`,
			`{"signupDate":{"in":[{"date":"2026-01-01"},{"date":"2026-02-01"}]}}`},
		{"and", `city === "Ankara" && totalOrders > 1`, `{"and":[{"city":{"eq":"Ankara"}},{"totalOrders":{"gt":1}}]}`},
		{"or", `city === "Ankara" || city === "İzmir"`, `{"or":[{"city":{"eq":"Ankara"}},{"city":{"eq":"İzmir"}}]}`},
		{"not", `!(city === "Ankara")`, `{"not":{"city":{"eq":"Ankara"}}}`},
		{"not of and", `!(city === "Ankara" && totalOrders > 1)`,
			`{"not":{"and":[{"city":{"eq":"Ankara"}},{"totalOrders":{"gt":1}}]}}`},
		{"same field twice is not merged", `totalOrders > 1 && totalOrders < 9`,
			`{"and":[{"totalOrders":{"gt":1}},{"totalOrders":{"lt":9}}]}`},
		{"redundant condition is not simplified", `city === "Ankara" && city === "Ankara"`,
			`{"and":[{"city":{"eq":"Ankara"}},{"city":{"eq":"Ankara"}}]}`},
		{"or inside and", `city === "Ankara" && (totalOrders > 1 || retired === true)`,
			`{"and":[{"city":{"eq":"Ankara"}},{"or":[{"totalOrders":{"gt":1}},{"retired":{"eq":true}}]}]}`},
		{"and inside or", `(city === "Ankara" && totalOrders > 1) || retired === false`,
			`{"or":[{"and":[{"city":{"eq":"Ankara"}},{"totalOrders":{"gt":1}}]},{"retired":{"eq":false}}]}`},
		{"some", `orders.some(o => o.totalGbp > 100)`, `{"orders":{"some":{"totalGbp":{"gt":"100"}}}}`},
		{"none", `orders.none(o => o.totalGbp > 100)`, `{"orders":{"none":{"totalGbp":{"gt":"100"}}}}`},
		{"every", `orders.every(o => o.itemsCount >= 2)`, `{"orders":{"every":{"itemsCount":{"gte":2}}}}`},
		{"events relation", `events.some(e => e.eventType === "purchase")`, `{"events":{"some":{"eventType":{"eq":"purchase"}}}}`},
		{"and inside relation body", `orders.some(o => o.totalGbp > 50 && o.itemsCount >= 2)`,
			`{"orders":{"some":{"and":[{"totalGbp":{"gt":"50"}},{"itemsCount":{"gte":2}}]}}}`},
		{"or and not inside relation body", `orders.none(o => o.itemsCount > 5 || !(o.orderStatus === "delivered" && o.totalGbp < 10))`,
			`{"orders":{"none":{"or":[{"itemsCount":{"gt":5}},{"not":{"and":[{"orderStatus":{"eq":"delivered"}},{"totalGbp":{"lt":"10"}}]}}]}}}`},
		{"in inside relation body", `orders.some(o => o.orderStatus in ["delivered", "shipped"])`,
			`{"orders":{"some":{"orderStatus":{"in":["delivered","shipped"]}}}}`},
		{"relations side by side", `orders.some(o => o.totalGbp > 1) && orders.none(x => x.totalGbp > 1000)`,
			`{"and":[{"orders":{"some":{"totalGbp":{"gt":"1"}}}},{"orders":{"none":{"totalGbp":{"gt":"1000"}}}}]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sameJSON(t, mustCompile(t, tt.src), tt.want)
		})
	}
}

func TestCompileValueEncodings(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string // exact bytes: Marshal is deterministic
	}{
		{"int is a JSON number", `totalOrders === 7`, `{"totalOrders":{"eq":7}}`},
		{"negative int", `totalOrders > -1`, `{"totalOrders":{"gt":-1}}`},
		{"int with leading zeros stays valid JSON", `totalOrders === 007`, `{"totalOrders":{"eq":7}}`},
		{"decimal keeps the literal text", `totalSpendGbp >= 1234.50`, `{"totalSpendGbp":{"gte":"1234.50"}}`},
		{"decimal without fraction", `totalSpendGbp >= 1000`, `{"totalSpendGbp":{"gte":"1000"}}`},
		{"decimal one digit", `totalSpendGbp < 0.5`, `{"totalSpendGbp":{"lt":"0.5"}}`},
		{"bool true", `hasOpenCart === true`, `{"hasOpenCart":{"eq":true}}`},
		{"bool false", `retired !== false`, `{"retired":{"ne":false}}`},
		{"string with quotes and unicode", `city === "İs\"tan"`, `{"city":{"eq":"İs\"tan"}}`},
		{"date on a Date field", `signupDate === date("2026-01-31")`, `{"signupDate":{"eq":{"date":"2026-01-31"}}}`},
		{"ago on a Date field", `lastActiveDate >= ago("12h")`, `{"lastActiveDate":{"gte":{"ago":"12h"}}}`},
		{"date on a DateTime field", `lastCampaignSentAt < date("2026-03-01")`, `{"lastCampaignSentAt":{"lt":{"date":"2026-03-01"}}}`},
		{"ago on a DateTime field", `lastCampaignSentAt > ago("7d")`, `{"lastCampaignSentAt":{"gt":{"ago":"7d"}}}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(mustCompile(t, tt.src))
			if err != nil {
				t.Fatal(err)
			}
			// Customer's JSON is compared as bytes, but HTML escaping is irrelevant to it.
			if got := string(b); got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

func TestCompileDecimalIsNeverAJSONNumber(t *testing.T) {
	b, err := json.Marshal(mustCompile(t, `totalSpendGbp > 99999999999999999.99`))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"totalSpendGbp":{"gt":"99999999999999999.99"}}`; string(b) != want {
		t.Errorf("got %s, want %s", b, want)
	}
}

func TestCompileDeterministic(t *testing.T) {
	first, _ := json.Marshal(mustCompile(t, sampleBrief))
	for range 20 {
		again, _ := json.Marshal(mustCompile(t, sampleBrief))
		if string(again) != string(first) {
			t.Fatalf("output differs between runs:\n%s\n%s", first, again)
		}
	}
}

func TestCompileVariableMatchesJSON(t *testing.T) {
	f := mustCompile(t, sampleBrief)
	vb, _ := json.Marshal(f.Variable())
	fb, _ := json.Marshal(f)
	if string(vb) != string(fb) {
		t.Errorf("Variable and MarshalJSON differ:\n%s\n%s", vb, fb)
	}
	if f.Source() != sampleBrief {
		t.Errorf("Source() = %q", f.Source())
	}
}

func TestCompileErrors(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		codes []dsl.Code
	}{
		{"parse error is a one-element slice", `city ==== "x"`, []dsl.Code{dsl.CodeSyntaxError}},
		{"unsupported operator", `city == "x"`, []dsl.Code{dsl.CodeUnsupportedOperator}},
		{"unknown field", `citty === "x"`, []dsl.Code{dsl.CodeUnknownField}},
		{"closed list value", `loyaltyTier === "Goldd"`, []dsl.Code{dsl.CodeUnknownValue}},
		{"semantic pass returns every error", `citty === "x" && totalOrders === "y" && loyaltyTier === "Goldd"`,
			[]dsl.Code{dsl.CodeUnknownField, dsl.CodeTypeMismatch, dsl.CodeUnknownValue}},
		{"empty source", ``, []dsl.Code{dsl.CodeSyntaxError}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, errs := Compile(tt.src)
			if f != nil {
				t.Errorf("filter = %v, want nil", f)
			}
			var got []dsl.Code
			for _, e := range errs {
				got = append(got, e.Code)
			}
			if !reflect.DeepEqual(got, tt.codes) {
				t.Errorf("codes = %v, want %v (%v)", got, tt.codes, errs)
			}
		})
	}
}

// at is a 1-based line:col.
type at struct{ line, col int }

func TestSpanLookup(t *testing.T) {
	f := mustCompile(t, sampleBrief)
	tests := []struct {
		name       string
		path       []any
		start, end at
	}{
		{"root", []any{"filter"}, at{1, 1}, at{3, 44}},
		{"and list", []any{"filter", "and"}, at{1, 1}, at{3, 44}},
		{"first term", []any{"filter", "and", 0}, at{1, 1}, at{1, 20}},
		{"field key", []any{"filter", "and", 0, "city"}, at{1, 1}, at{1, 5}},
		{"literal of eq", []any{"filter", "and", 0, "city", "eq"}, at{1, 10}, at{1, 20}},
		{"relation term", []any{"filter", "and", 1}, at{2, 1}, at{2, 45}},
		{"relation name", []any{"filter", "and", 1, "cartItems"}, at{2, 1}, at{2, 10}},
		{"quantifier", []any{"filter", "and", 1, "cartItems", "some"}, at{2, 1}, at{2, 45}},
		{"lambda field is the child field", []any{"filter", "and", 2, "orders", "none", "orderDate"}, at{3, 18}, at{3, 29}},
		{"ago operand", []any{"filter", "and", 2, "orders", "none", "orderDate", "gte"}, at{3, 33}, at{3, 43}},
		{"ago key of the time value", []any{"filter", "and", 2, "orders", "none", "orderDate", "gte", "ago"}, at{3, 33}, at{3, 43}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp, ok := f.Span(tt.path)
			if !ok {
				t.Fatalf("Span(%v) not found", tt.path)
			}
			if got := (at{sp.Start.Line, sp.Start.Col}); got != tt.start {
				t.Errorf("start = %v, want %v", got, tt.start)
			}
			if got := (at{sp.End.Line, sp.End.Col}); got != tt.end {
				t.Errorf("end = %v, want %v", got, tt.end)
			}
		})
	}
}

func TestSpanSingleConditionRoot(t *testing.T) {
	f := mustCompile(t, `city === "Istnbul"`)
	sp, ok := f.Span([]any{"filter", "city", "eq"})
	if !ok || sp.Start.Col != 10 || sp.End.Col != 19 || sp.Start.Offset != 9 {
		t.Errorf("Span = %+v, %v; want the literal at cols 10-19", sp, ok)
	}
}

func TestSpanInList(t *testing.T) {
	src := `retired === false && loyaltyTier in ["Gold", "Silver", "Platinum"]`
	f := mustCompile(t, src)
	for i, want := range []string{`"Gold"`, `"Silver"`, `"Platinum"`} {
		sp, ok := f.Span([]any{"filter", "and", 1, "loyaltyTier", "in", i})
		if !ok {
			t.Fatalf("in[%d] not found", i)
		}
		if got := src[sp.Start.Offset:sp.End.Offset]; got != want {
			t.Errorf("in[%d] = %s, want %s", i, got, want)
		}
	}
	sp, ok := f.Span([]any{"filter", "and", 1, "loyaltyTier", "in"})
	if !ok || src[sp.Start.Offset:sp.End.Offset] != `loyaltyTier in ["Gold", "Silver", "Platinum"]` {
		t.Errorf("in list span = %+v, %v", sp, ok)
	}
}

func TestSpanNestedAndNot(t *testing.T) {
	src := `orders.none(o => o.itemsCount > 5 || !(o.orderStatus === "delivered" && o.totalGbp < 10))`
	f := mustCompile(t, src)
	sp, ok := f.Span([]any{"filter", "orders", "none", "or", 1, "not", "and", 0, "orderStatus", "eq"})
	if !ok {
		t.Fatal("deep path not found")
	}
	if got := src[sp.Start.Offset:sp.End.Offset]; got != `"delivered"` {
		t.Errorf("literal = %s", got)
	}
	// A not and its operand share a path; the larger node, the negation, owns it.
	sp, _ = f.Span([]any{"filter", "orders", "none", "or", 1, "not"})
	if got := src[sp.Start.Offset:sp.End.Offset]; !strings.HasPrefix(got, "!(") {
		t.Errorf("not span = %q", got)
	}
}

func TestSpanPrefixFallback(t *testing.T) {
	f := mustCompile(t, sampleBrief)
	tests := []struct {
		name string
		path []any
		want at // start of the span found
	}{
		{"unknown leaf falls back to the literal", []any{"filter", "and", 0, "city", "eq", "nope"}, at{1, 10}},
		{"unknown operator falls back to the field", []any{"filter", "and", 0, "city", "gt"}, at{1, 1}},
		{"index out of range falls back to the list", []any{"filter", "and", 7, "city"}, at{1, 1}},
		{"unknown root key falls back to the root", []any{"filter", "orders"}, at{1, 1}},
		{"deep unknown falls back to the deepest known", []any{"filter", "and", 2, "orders", "none", "orderDate", "gte", "at"}, at{3, 33}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp, ok := f.Span(tt.path)
			if !ok {
				t.Fatalf("Span(%v) not found", tt.path)
			}
			if got := (at{sp.Start.Line, sp.Start.Col}); got != tt.want {
				t.Errorf("start = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSpanUnknown(t *testing.T) {
	f := mustCompile(t, sampleBrief)
	for _, path := range [][]any{nil, {}, {"other"}, {"other", "and", 0}, {1, "filter"}, {nil}} {
		if sp, ok := f.Span(path); ok {
			t.Errorf("Span(%v) = %+v, want not found", path, sp)
		}
	}
}

// A path decoded from JSON carries numbers as float64 or json.Number (Customer's filterPath).
func TestSpanAcceptsDecodedNumbers(t *testing.T) {
	f := mustCompile(t, sampleBrief)
	want, _ := f.Span([]any{"filter", "and", 1, "cartItems"})
	var decoded []any
	if err := json.Unmarshal([]byte(`["filter","and",1,"cartItems"]`), &decoded); err != nil {
		t.Fatal(err)
	}
	if sp, ok := f.Span(decoded); !ok || sp != want {
		t.Errorf("float64 path: %+v, %v; want %+v", sp, ok, want)
	}
	dec := json.NewDecoder(strings.NewReader(`["filter","and",1,"cartItems"]`))
	dec.UseNumber()
	decoded = nil
	if err := dec.Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	if sp, ok := f.Span(decoded); !ok || sp != want {
		t.Errorf("json.Number path: %+v, %v; want %+v", sp, ok, want)
	}
	if _, ok := f.Span([]any{"filter", "and", 1.5}); !ok {
		t.Error("fractional index should fall back to the prefix")
	}
}

// Strings and integers must not collide in the index: a field called "0" is not index 0.
func TestSpanKeysDoNotCollide(t *testing.T) {
	a, _ := segKey("0")
	b, _ := segKey(0)
	if a == b {
		t.Errorf("segKey(%q) == segKey(0)", "0")
	}
}
