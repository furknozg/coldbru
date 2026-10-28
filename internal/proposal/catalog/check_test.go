package catalog

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/furknozg/coldbru/internal/proposal/dsl"
)

const sampleBrief = `city === "İstanbul" && cartItems.some(c => c.addedAt >= ago("30d")) && orders.none(o => o.orderDate >= ago("30d"))`

func mustParse(t testing.TB, src string) dsl.Expr {
	t.Helper()
	e, err := dsl.Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	return e
}

func check(t testing.TB, src string) []*dsl.Error {
	t.Helper()
	return Check(src, mustParse(t, src))
}

func TestCheckValid(t *testing.T) {
	t.Parallel()
	for _, src := range []string{
		sampleBrief,
		`city === "İstanbul"`,
		`city !== "Ankara" && city in ["İstanbul", "Ankara", "Foo"]`, // city is validated by Customer only
		`ageYears >= 25 && (sex === "Male" || sex in ["Female"])`,
		`ageYears === -5`,
		`ageYears === 2147483647`,
		`ageYears === -2147483648`,
		`ageYears in [1, 2, 3]`,
		`totalSpendGbp > 10`,
		`totalSpendGbp > 10.5`,
		`totalSpendGbp === 10.50`,
		`totalSpendGbp in [1, 2.5, 3.25]`,
		`commuteDistanceKm <= 12.75`,
		`hasOpenCart === true`,
		`retired !== false`,
		`signupDate === date("2026-01-01")`,
		`signupDate !== date("2026-01-01")`,
		`signupDate in [date("2026-01-01"), date("2026-01-02")]`,
		`signupDate >= ago("30d") && lastActiveDate < ago("12h")`,
		`signupDate <= date("2026-01-01")`,
		`lastCampaignSentAt < ago("12h")`,
		`lastCampaignSentAt === date("2026-01-01")`,
		`loyaltyTier in ["Gold", "Platinum"] && loyaltyTier !== "Bronze"`,
		`countryCode === "TR" && language === "tr" && preferredPlatform in ["ios", "android", "web"]`,
		`!(loyaltyTier === "Bronze")`,
		`orders.some(o => o.totalGbp > 1000)`,
		`orders.every(o => o.totalGbp > 0)`,
		`orders.none(o => o.orderStatus === "cancelled" || o.orderStatus in ["returned", "refunded"])`,
		`orders.some(o => o.platform === "anything" && o.countryCode === "XX" && o.shippingCity === "Nowhere")`, // not closed in Customer
		`orders.some(o => !(o.itemsCount >= 3))`,
		`cartItems.some(c => c.cartStatus === "abandoned" && c.unitPriceGbp >= 9.99)`,
		`events.some(e => e.eventType === "purchase" && e.eventTime >= ago("7d") && e.valueGbp >= 20)`,
		`orders.some(o => o.totalGbp > 1) && orders.some(o => o.totalGbp < 1)`,
	} {
		if errs := check(t, src); errs != nil {
			t.Errorf("Check(%q) = %v, want nil", src, errs)
		}
	}
}

// wantErr is one expected error: code, the highlighted source text and a message fragment.
type wantErr struct {
	code dsl.Code
	near string
	msg  string
	sugg []string
}

func TestCheckErrors(t *testing.T) {
	t.Parallel()
	const (
		field    = dsl.CodeUnknownField
		relation = dsl.CodeUnknownRelation
		value    = dsl.CodeUnknownValue
		mismatch = dsl.CodeTypeMismatch
	)
	tests := []struct {
		name string
		src  string
		want []wantErr
	}{
		// Unknown fields.
		{"typo with suggestion", `cty === "x"`, []wantErr{{field, "cty", `did you mean "city"`, []string{"city"}}}},
		{"transposition", `ctiy === "x"`, []wantErr{{field, "ctiy", `did you mean "city"`, []string{"city"}}}},
		{"case-insensitive", `AGEYEARS > 1`, []wantErr{{field, "AGEYEARS", `did you mean "ageYears"`, []string{"ageYears"}}}},
		{"prefix suggestions", `loyalty > 1`, []wantErr{{field, "loyalty", `did you mean "loyaltyTier" or "loyaltyPoints"`, []string{"loyaltyTier", "loyaltyPoints"}}}},
		{"at most three suggestions", `vehicle > 1`, []wantErr{{field, "vehicle", "did you mean", []string{"vehicleMake", "vehicleModel", "vehicleYear"}}}},
		{"no suggestion, long list omitted", `zzzzzz === 1`, []wantErr{{field, "zzzzzz", `unknown field "zzzzzz" on customer`, nil}}},
		{"personal field", `email === "a@b.c"`, []wantErr{{field, "email", `unknown field "email"`, []string{"emailsOpened90d"}}}},
		{"consent field", `emailOptIn === true`, []wantErr{{field, "emailOptIn", `unknown field "emailOptIn"`, nil}}},
		{"dotted root path", `city.name === "x"`, []wantErr{{field, "city.name", "no dotted paths", nil}}},
		{"relation field through root", `orders.totalGbp > 1`, []wantErr{{field, "orders.totalGbp", "orders.some(o => o.totalGbp ...)", nil}}},
		{"unknown lambda field lists fields", `orders.some(o => o.foo > 1)`, []wantErr{{field, "o.foo", "fields: orderDate, orderStatus", nil}}},
		{"unknown lambda field suggests", `orders.some(o => o.totalGpb > 1)`, []wantErr{{field, "o.totalGpb", `did you mean "totalGbp"`, []string{"totalGbp"}}}},
		{"customer field in lambda", `orders.some(o => o.city === "x")`, []wantErr{{field, "o.city", `unknown field "city" on orders`, nil}}},
		{"extra segment in lambda", `orders.some(o => o.totalGbp.x > 1)`, []wantErr{{field, "o.totalGbp.x", "write o.<field>", nil}}},

		// Relations.
		{"relation compared", `orders === 1`, []wantErr{{mismatch, "orders", "orders is a relation, not a field; use orders.some(o => ...)", nil}}},
		{"relation in list", `cartItems in [1]`, []wantErr{{mismatch, "cartItems", "use cartItems.some(c => ...)", nil}}},
		{"field used as relation", `city.some(c => c.x === 1)`, []wantErr{{relation, "city", "city is a field, not a relation", nil}}},
		{"relation typo", `ordrs.some(o => o.totalGbp > 1)`, []wantErr{{relation, "ordrs", `did you mean "orders"`, []string{"orders"}}}},
		{"relation unknown lists relations", `foo.some(f => f.x > 1)`, []wantErr{{relation, "foo", "relations: orders, cartItems, events", nil}}},
		{"dotted relation path", `a.b.some(x => x.c > 1)`, []wantErr{{relation, "a.b", `"a.b" is not a relation of customer`, nil}}},
		{"nested relation", `orders.some(o => o.items.some(i => i.quantity >= 2))`, []wantErr{{relation, "o.items", "orders has no relations (relations do not nest)", nil}}},
		{"nested relation names a field", `orders.some(o => o.totalGbp.some(t => t.x > 1))`, []wantErr{{relation, "o.totalGbp", "totalGbp is a field, not a relation", nil}}},
		{"unknown relation hides body", `foo.some(f => f.bad > 1)`, []wantErr{{relation, "foo", "unknown relation", nil}}},

		// Operators.
		{"ordering on string", `loyaltyTier > "Gold"`, []wantErr{{mismatch, ">", "operator > cannot be used on loyaltyTier (String); allowed operators: ===, !==, in", nil}}},
		{"in on bool", `hasOpenCart in [true]`, []wantErr{{mismatch, "in", "operator in cannot be used on hasOpenCart (Bool); allowed operators: ===, !==", nil}}},
		{"ordering on bool", `retired >= true`, []wantErr{{mismatch, ">=", "operator >= cannot be used on retired", nil}}},
		{"operator error skips the value check", `loyaltyTier > 5`, []wantErr{{mismatch, ">", "operator >", nil}}},

		// Literal types.
		{"string on int", `ageYears === "25"`, []wantErr{{mismatch, `"25"`, "ageYears is of type Int; compare it with a whole number such as 25, not a string", nil}}},
		{"fraction on int", `ageYears > 25.5`, []wantErr{{mismatch, "25.5", "use a whole number between -2147483648 and 2147483647, not 25.5", nil}}},
		{"int above int32", `ageYears > 2147483648`, []wantErr{{mismatch, "2147483648", "whole number", nil}}},
		{"int below int32", `ageYears < -2147483649`, []wantErr{{mismatch, "-2147483649", "whole number", nil}}},
		{"huge int", `ageYears === 99999999999999999999`, []wantErr{{mismatch, "99999999999999999999", "whole number", nil}}},
		{"bool on int", `ageYears === true`, []wantErr{{mismatch, "true", "not true/false", nil}}},
		{"ago on int", `ageYears >= ago("30d")`, []wantErr{{mismatch, `ago("30d")`, "not ago()", nil}}},
		{"third decimal", `totalSpendGbp > 10.005`, []wantErr{{mismatch, "10.005", "at most 2 decimal places, not 10.005", nil}}},
		{"string on decimal", `totalSpendGbp > "one thousand"`, []wantErr{{mismatch, `"one thousand"`, "totalSpendGbp is of type Decimal", nil}}},
		{"bool on decimal", `totalSpendGbp === true`, []wantErr{{mismatch, "true", "Decimal", nil}}},
		{"number on string", `city === 5`, []wantErr{{mismatch, "5", "compare it with a string literal", nil}}},
		{"bool on string", `city === true`, []wantErr{{mismatch, "true", "String", nil}}},
		{"ago on string", `city === ago("1d")`, []wantErr{{mismatch, `ago("1d")`, "String", nil}}},
		{"string on bool", `retired === "yes"`, []wantErr{{mismatch, `"yes"`, "compare it with true or false", nil}}},
		{"number on bool", `retired === 1`, []wantErr{{mismatch, "1", "Bool", nil}}},
		{"string on date", `signupDate >= "2026-01-01"`, []wantErr{{mismatch, `"2026-01-01"`, `date("YYYY-MM-DD") or ago("30d")`, nil}}},
		{"number on date", `signupDate > 5`, []wantErr{{mismatch, "5", "Date", nil}}},
		{"bool on datetime", `lastCampaignSentAt === false`, []wantErr{{mismatch, "false", "DateTime", nil}}},
		{"ago equality", `signupDate === ago("30d")`, []wantErr{{mismatch, `ago("30d")`, "ago() can only be used with >, >=, < or <=, not ===", nil}}},
		{"ago inequality", `lastCampaignSentAt !== ago("1d")`, []wantErr{{mismatch, `ago("1d")`, "not !==", nil}}},
		{"ago in list", `signupDate in [date("2026-01-01"), ago("1d")]`, []wantErr{{mismatch, `ago("1d")`, "not in", nil}}},
		{"each bad list element", `ageYears in [1, "a", 2.5, 3]`, []wantErr{{mismatch, `"a"`, "Int", nil}, {mismatch, "2.5", "whole number", nil}}},

		// Closed lists.
		{"wrong case", `loyaltyTier === "gold"`, []wantErr{{value, `"gold"`, `"gold" is not a value of loyaltyTier; did you mean "Gold"`, []string{"Gold"}}}},
		{"unknown tier lists allowed", `loyaltyTier === "Diamond"`, []wantErr{{value, `"Diamond"`, "allowed values: Bronze, Silver, Gold, Platinum", nil}}},
		{"per element", `loyaltyTier in ["Gold", "Silver", "Gld"]`, []wantErr{{value, `"Gld"`, `did you mean "Gold"`, []string{"Gold"}}}},
		{"prefix on lower-cased values", `sex === "M"`, []wantErr{{value, `"M"`, `did you mean "Male"`, []string{"Male"}}}},
		{"country is case-sensitive", `countryCode === "tr"`, []wantErr{{value, `"tr"`, `did you mean "TR" or "GB"`, []string{"TR", "GB"}}}},
		{"language", `language === "turkish"`, []wantErr{{value, `"turkish"`, "allowed values: tr, en", nil}}},
		{"platform", `preferredPlatform in ["IOS"]`, []wantErr{{value, `"IOS"`, `did you mean "ios"`, []string{"ios"}}}},
		{"spelling variant in lambda", `orders.none(o => o.orderStatus === "canceled")`, []wantErr{{value, `"canceled"`, `did you mean "cancelled"`, []string{"cancelled"}}}},
		{"cart status", `cartItems.some(c => c.cartStatus === "open " )`, []wantErr{{value, `"open "`, `did you mean "open"`, []string{"open"}}}},
		{"event type prefix", `events.some(e => e.eventType === "add")`, []wantErr{{value, `"add"`, "did you mean", []string{"add_to_cart"}}}},
		{"type error wins over closed check", `loyaltyTier === 5`, []wantErr{{mismatch, "5", "String", nil}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			errs := check(t, tc.src)
			if len(errs) != len(tc.want) {
				t.Fatalf("Check(%q) = %d errors %v, want %d", tc.src, len(errs), errs, len(tc.want))
			}
			for i, w := range tc.want {
				e := errs[i]
				if e.Code != w.code || e.Near != w.near || !strings.Contains(e.Message, w.msg) {
					t.Errorf("error %d of %q\n got  %s: %q near %q\n want %s: contains %q near %q", i, tc.src, e.Code, e.Message, e.Near, w.code, w.msg, w.near)
				}
				if !slices.Equal(e.Suggestions, w.sugg) {
					t.Errorf("error %d suggestions = %v, want %v", i, e.Suggestions, w.sugg)
				}
				checkRange(t, tc.src, e)
			}
		})
	}
}

// checkRange verifies an error's range lies in the source, its positions match its offsets and the
// Near excerpt is the highlighted text.
func checkRange(t testing.TB, src string, e *dsl.Error) {
	t.Helper()
	if e.Start.Offset < 0 || e.End.Offset > len(src) || e.Start.Offset > e.End.Offset {
		t.Errorf("range %d..%d outside source %q", e.Start.Offset, e.End.Offset, src)
		return
	}
	if e.Start != posAt(src, e.Start.Offset) || e.End != posAt(src, e.End.Offset) {
		t.Errorf("%s: positions %+v-%+v do not match offsets in %q", e.Code, e.Start, e.End, src)
	}
}

func posAt(src string, off int) dsl.Pos {
	line, col := 1, 1
	for i, r := range src {
		if i >= off {
			break
		}
		if r == '\n' {
			line++
			col = 1
			continue
		}
		col++
	}
	return dsl.Pos{Offset: off, Line: line, Col: col}
}

func TestCheckPositions(t *testing.T) {
	t.Parallel()
	src := "city === \"İstanbul\" &&\n  ageYears === \"25\" &&\n  loyaltyTier   >   \"Gold\"\n"
	errs := check(t, src)
	if len(errs) != 2 {
		t.Fatalf("errors = %v", errs)
	}
	type pos struct{ sl, sc, el, ec int }
	for i, want := range []pos{{2, 16, 2, 20}, {3, 17, 3, 18}} {
		e := errs[i]
		got := pos{e.Start.Line, e.Start.Col, e.End.Line, e.End.Col}
		if got != want {
			t.Errorf("error %d (%s) at %+v, want %+v", i, e.Code, got, want)
		}
		checkRange(t, src, e)
	}
	if errs[0].Near != `"25"` || errs[1].Near != ">" {
		t.Errorf("near = %q, %q", errs[0].Near, errs[1].Near)
	}
	if !strings.HasPrefix(errs[0].Error(), "TYPE_MISMATCH at 2:16:") {
		t.Errorf("Error() = %q", errs[0].Error())
	}

	// A column counts runes: the İ before the value is one column.
	errs = check(t, `city in ["İstanbul"] && cty === "x"`)
	if len(errs) != 1 || errs[0].Start.Col != 25 || errs[0].Start.Offset != 25 {
		t.Errorf("rune columns: %+v", errs)
	}
}

func TestCheckOperatorRange(t *testing.T) {
	t.Parallel()
	// `in` with newlines between the field and the list.
	src := "hasOpenCart\n  in\n  [true]"
	errs := check(t, src)
	if len(errs) != 1 || errs[0].Near != "in" || errs[0].Start.Line != 2 || errs[0].Start.Col != 3 || errs[0].End.Col != 5 {
		t.Fatalf("errors = %+v", errs)
	}
	checkRange(t, src, errs[0])
}

func TestCheckMultipleErrorsInSourceOrder(t *testing.T) {
	t.Parallel()
	src := `cty === 1 && orders.some(o => o.nope > 1 && o.totalGbp === "x") && ageYears === "x" && loyaltyTier === "gold" && foo.some(f => f.x > 1)`
	errs := check(t, src)
	wantCodes := []dsl.Code{dsl.CodeUnknownField, dsl.CodeUnknownField, dsl.CodeTypeMismatch, dsl.CodeTypeMismatch, dsl.CodeUnknownValue, dsl.CodeUnknownRelation}
	if len(errs) != len(wantCodes) {
		t.Fatalf("errors = %v", errs)
	}
	for i, e := range errs {
		if e.Code != wantCodes[i] {
			t.Errorf("error %d = %s, want %s", i, e.Code, wantCodes[i])
		}
		if i > 0 && e.Start.Offset < errs[i-1].Start.Offset {
			t.Errorf("error %d (%d) comes before error %d (%d)", i, e.Start.Offset, i-1, errs[i-1].Start.Offset)
		}
	}
}

func TestCheckCapsErrors(t *testing.T) {
	t.Parallel()
	terms := make([]string, MaxErrors+5)
	for i := range terms {
		terms[i] = fmt.Sprintf(`ageYears === "v%d"`, i)
	}
	src := strings.Join(terms, " || ")
	errs := check(t, src)
	if MaxErrors != 10 || len(errs) != MaxErrors {
		t.Fatalf("got %d errors, want %d", len(errs), MaxErrors)
	}
	if errs[MaxErrors-1].Near != `"v9"` {
		t.Errorf("last kept error = %q, want the 10th in source order", errs[MaxErrors-1].Near)
	}
}

// TestCheckValidAfterFix: the first error never hides the others, and fixing all of them passes.
func TestCheckValidAfterFix(t *testing.T) {
	t.Parallel()
	bad := `cty === "x" && loyaltyTier === "gold" && totalSpendGbp > 1.234`
	good := `city === "x" && loyaltyTier === "Gold" && totalSpendGbp > 1.23`
	if n := len(check(t, bad)); n != 3 {
		t.Errorf("bad: %d errors, want 3", n)
	}
	if errs := check(t, good); errs != nil {
		t.Errorf("good: %v", errs)
	}
}

func TestCheckNilAndForeignNodes(t *testing.T) {
	t.Parallel()
	if errs := Check("", nil); errs != nil {
		t.Errorf("Check(nil) = %v", errs)
	}
	for _, e := range []dsl.Expr{(*dsl.And)(nil), (*dsl.Or)(nil), (*dsl.Not)(nil), (*dsl.Compare)(nil), (*dsl.In)(nil), (*dsl.Relation)(nil)} {
		if errs := Check("", e); errs != nil {
			t.Errorf("Check(%T nil) = %v", e, errs)
		}
	}
	// Hand-built trees the parser would never produce must not panic.
	src := `a === 1`
	for _, e := range []dsl.Expr{
		&dsl.Compare{Field: dsl.Path{Segments: []string{"city"}}, Op: dsl.OpEq},
		&dsl.Compare{Field: dsl.Path{Segments: nil}, Op: dsl.OpEq, Value: dsl.String{}},
		&dsl.In{Field: dsl.Path{Segments: []string{"hasOpenCart"}}},
		&dsl.Relation{Path: dsl.Path{Segments: nil}, Quantifier: dsl.Some, Var: "o"},
		&dsl.Relation{Path: dsl.Path{Segments: []string{"orders"}}, Quantifier: dsl.Some, Var: "o", Body: &dsl.Compare{
			Field: dsl.Path{Segments: []string{"x", "totalGbp"}}, Op: dsl.OpGt, Value: dsl.Number{Text: "1"}}},
	} {
		_ = Check(src, e) // positions are zero values; only the absence of a panic matters
	}
}

// TestQuantifierNotInCatalog uses a small catalog whose relation lacks `every`.
func TestQuantifierNotInCatalog(t *testing.T) {
	t.Parallel()
	root := &Entity{
		Name: "customer",
		Relations: []Relation{{
			Name: "orders", Quantifiers: []dsl.Quantifier{dsl.Some, dsl.None},
			Entity: &Entity{Name: "orders", Fields: []Field{{Name: "total", Kind: KindInt, Ops: []dsl.Op{dsl.OpGt}}}},
		}},
	}
	src := "orders.every(o => o.total > 1 && o.nope > 2)"
	errs := checkAgainst(root, src, mustParse(t, src))
	if len(errs) != 2 {
		t.Fatalf("errors = %v", errs)
	}
	if errs[0].Code != dsl.CodeUnknownRelation || errs[0].Near != "every" || !strings.Contains(errs[0].Message, "use one of: some, none") {
		t.Errorf("quantifier error = %+v", errs[0])
	}
	if errs[1].Code != dsl.CodeUnknownField {
		t.Errorf("body is still checked: %+v", errs[1])
	}
	if errs := checkAgainst(root, "orders.some(o => o.total > 1)", mustParse(t, "orders.some(o => o.total > 1)")); errs != nil {
		t.Errorf("valid quantifier: %v", errs)
	}
}

func TestSuggest(t *testing.T) {
	t.Parallel()
	names := []string{"city", "country", "countryCode", "language", "loyaltyTier", "loyaltyPoints"}
	tests := []struct {
		in   string
		want []string
	}{
		{"cty", []string{"city"}},
		{"CITY", []string{"city"}},
		{"count", []string{"country", "countryCode"}},
		{"loyal", []string{"loyaltyTier", "loyaltyPoints"}},
		{"languag", []string{"language"}},
		{"lanuage", []string{"language"}},
		{"zzz", nil},
		{"", nil},
	}
	for _, tc := range tests {
		if got := suggest(tc.in, names); !slices.Equal(got, tc.want) {
			t.Errorf("suggest(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
	if got := suggest("a", []string{"ab", "ac", "ad", "ae"}); len(got) != maxSuggestions {
		t.Errorf("suggest keeps %d, want %d", len(got), maxSuggestions)
	}
	// An exact prefix match ranks above a mere edit-distance match.
	if got := suggest("cat", []string{"bat", "category"}); !slices.Equal(got, []string{"category", "bat"}) {
		t.Errorf("ranking = %v", got)
	}
}

func TestAdvance(t *testing.T) {
	t.Parallel()
	p := advance(dsl.Pos{Offset: 0, Line: 1, Col: 1}, "ab\nİc")
	if p != (dsl.Pos{Offset: 6, Line: 2, Col: 3}) {
		t.Errorf("advance = %+v", p)
	}
}

// FuzzCheck: whatever dsl.Parse accepts, Check must not panic, must stay within MaxErrors, and
// must return well-formed, source-ordered errors.
func FuzzCheck(f *testing.F) {
	for _, s := range []string{
		sampleBrief,
		`city === "İstanbul" && totalSpendGbp >= 1000`,
		`city in ["İstanbul", "Ankara", "İzmir"] && loyaltyTier in ["Gold", "Platinum"]`,
		`lastCartAddDate >= ago("30d") && hasOpenCart === true`,
		`orders.some(o => o.totalGbp > 1000)`,
		"city === \"İstanbul\" &&\ncartItems.some(c => c.addedAt >= ago(\"30d\")) &&\norders.none(o => o.orderDate >= ago(\"30d\"))",
		`orders.none(o => o.orderStatus === "cancelled")`,
		`orders.every(o => o.totalGbp > 0)`,
		"ageYears >= 25 &&\n(city === \"İstanbul\" || city === \"Ankara\")",
		`ageYears === 25`,
		`loyaltyTier !== "Bronze"`,
		`signupDate < date("2026-01-01")`,
		`retired === false`,
		`!(loyaltyTier === "Bronze")`,
		`a === 1 || b === 2 && c === 3`,
		`orders.some(o => o.totalGbp > 1000) && orders.some(o => o.orderStatus === "delivered")`,
		"orders.some(o =>\n  o.totalGbp > 1000 &&\n  o.orderStatus === \"delivered\"\n)",
		`orders.some(o => o.items.some(i => i.quantity >= 2))`,
		`orders.some(o => o.totalGbp > 1000) || cartItems.every(c => c.quantity > 0)`,
		`ageYears === "25"`, `ageYears > 25.5`, `ageYears > 2147483648`, `totalSpendGbp > 10.005`,
		`loyaltyTier === "gold"`, `loyaltyTier > "Gold"`, `hasOpenCart in [true]`, `signupDate === ago("30d")`,
		`signupDate in [ago("1d")]`, `orders === 1`, `city.some(c => c.x === 1)`, `orders.totalGbp > 1`,
		`events.some(e => e.eventType === "add")`, `cty === 1 && foo.some(f => f.x > 1)`,
		`ş === 1`, `city === "İ"`,
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, src string) {
		e, err := dsl.Parse(src)
		if err != nil {
			return
		}
		errs := Check(src, e)
		if len(errs) > MaxErrors {
			t.Fatalf("%d errors for %q", len(errs), src)
		}
		last := 0
		for _, ce := range errs {
			switch ce.Code {
			case dsl.CodeUnknownField, dsl.CodeUnknownRelation, dsl.CodeUnknownValue, dsl.CodeTypeMismatch:
			default:
				t.Fatalf("code %q for %q", ce.Code, src)
			}
			if ce.Message == "" || len(ce.Suggestions) > maxSuggestions {
				t.Fatalf("bad error %+v for %q", ce, src)
			}
			if ce.Start.Offset < last {
				t.Fatalf("errors out of order for %q: %v", src, errs)
			}
			last = ce.Start.Offset
			checkRange(t, src, ce)
		}
	})
}
