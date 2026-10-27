package audience

import (
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/furknozg/coldbru/internal/customer/graph/model"
)

func testCompiler(t *testing.T) *Compiler {
	t.Helper()
	cities, err := NewCities([]string{"İstanbul", "Ankara", "Eskişehir", "Diyarbakır", "London"})
	if err != nil {
		t.Fatal(err)
	}
	return &Compiler{Cities: cities}
}

// parse decodes filter JSON written like a GraphQL variable. Date and At need
// no help: only Go literals build those, see timeFilter tests.
func parse(t *testing.T, src string) *model.CustomerFilter {
	t.Helper()
	var f model.CustomerFilter
	dec := json.NewDecoder(strings.NewReader(src))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		t.Fatalf("parse %s: %v", src, err)
	}
	return &f
}

func ptr[T any](v T) *T { return &v }

// goldenCase is a filter with the exact SQL and arguments it compiles to. The
// integration test also runs every case against Postgres.
type goldenCase struct {
	name     string
	filter   *model.CustomerFilter
	wantSQL  string
	wantArgs []any
}

func goldenCases(t *testing.T) []goldenCase {
	t.Helper()
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	instant := time.Date(2026, 10, 27, 9, 30, 0, 0, time.UTC)

	return []goldenCase{
		{
			name: "docs 8.2 sample brief",
			filter: parse(t, `{"and":[
				{"city":{"eq":"İstanbul"}},
				{"cartItems":{"some":{"addedAt":{"gte":{"ago":"30d"}}}}},
				{"orders":{"none":{"orderDate":{"gte":{"ago":"30d"}}}}}]}`),
			wantSQL: "(c.city IS NOT DISTINCT FROM $1" +
				" AND EXISTS (SELECT 1 FROM dwh.fact_cart_item ci WHERE ci.customer_id = c.customer_id AND (ci.added_at >= now() - $2::interval))" +
				" AND NOT EXISTS (SELECT 1 FROM dwh.fact_order o WHERE o.customer_id = c.customer_id AND (o.order_date >= now() - $3::interval)))",
			wantArgs: []any{"İstanbul", "30 days", "30 days"},
		},
		{
			name:     "empty filter matches everyone",
			filter:   parse(t, `{}`),
			wantSQL:  "TRUE",
			wantArgs: nil,
		},
		{
			name:     "city is canonicalized, lower case",
			filter:   parse(t, `{"city":{"eq":"istanbul"}}`),
			wantSQL:  "c.city IS NOT DISTINCT FROM $1",
			wantArgs: []any{"İstanbul"},
		},
		{
			name:     "city is canonicalized, ASCII upper case with diacritics",
			filter:   parse(t, `{"city":{"eq":"ESKISEHIR"}}`),
			wantSQL:  "c.city IS NOT DISTINCT FROM $1",
			wantArgs: []any{"Eskişehir"},
		},
		{
			name:     "city in canonicalizes every element",
			filter:   parse(t, `{"city":{"in":["DIYARBAKIR","ıstanbul","Ankara"]}}`),
			wantSQL:  "c.city = ANY($1::text[])",
			wantArgs: []any{[]string{"Diyarbakır", "İstanbul", "Ankara"}},
		},
		{
			name:     "ne is null safe",
			filter:   parse(t, `{"favouriteCategory":{"ne":"X"}}`),
			wantSQL:  "c.favourite_category IS DISTINCT FROM $1",
			wantArgs: []any{"X"},
		},
		{
			name:     "fields of one object are ANDed in catalog order",
			filter:   parse(t, `{"loyaltyTier":{"eq":"Gold"},"totalSpendGbp":{"gte":"100.50"}}`),
			wantSQL:  "(c.total_spend_gbp >= $1::numeric AND c.loyalty_tier IS NOT DISTINCT FROM $2)",
			wantArgs: []any{"100.50", "Gold"},
		},
		{
			name:     "not treats unknown as no match",
			filter:   parse(t, `{"not":{"loyaltyTier":{"eq":"Gold"}}}`),
			wantSQL:  "NOT COALESCE((c.loyalty_tier IS NOT DISTINCT FROM $1), false)",
			wantArgs: []any{"Gold"},
		},
		{
			name:     "or",
			filter:   parse(t, `{"or":[{"sex":{"eq":"Male"}},{"hasOpenCart":{"eq":true}}]}`),
			wantSQL:  "(c.sex IS NOT DISTINCT FROM $1 OR c.has_open_cart IS NOT DISTINCT FROM $2::boolean)",
			wantArgs: []any{"Male", true},
		},
		{
			name:     "boolean ne",
			filter:   parse(t, `{"retired":{"ne":true}}`),
			wantSQL:  "c.retired IS DISTINCT FROM $1::boolean",
			wantArgs: []any{true},
		},
		{
			name:     "several operators on one field",
			filter:   parse(t, `{"ageYears":{"gte":18,"lt":65}}`),
			wantSQL:  "(c.age_years >= $1::bigint AND c.age_years < $2::bigint)",
			wantArgs: []any{int64(18), int64(65)},
		},
		{
			name:     "all ordering operators plain, in fixed order",
			filter:   parse(t, `{"loyaltyPoints":{"lte":9,"gt":1,"eq":5,"ne":6,"lt":8,"gte":2}}`),
			wantSQL:  "(c.loyalty_points IS NOT DISTINCT FROM $1::bigint AND c.loyalty_points IS DISTINCT FROM $2::bigint AND c.loyalty_points > $3::bigint AND c.loyalty_points >= $4::bigint AND c.loyalty_points < $5::bigint AND c.loyalty_points <= $6::bigint)",
			wantArgs: []any{int64(5), int64(6), int64(1), int64(2), int64(8), int64(9)},
		},
		{
			name:     "int in",
			filter:   parse(t, `{"ageYears":{"in":[18,19]}}`),
			wantSQL:  "c.age_years = ANY($1::bigint[])",
			wantArgs: []any{[]int64{18, 19}},
		},
		{
			name:     "decimal in",
			filter:   parse(t, `{"totalSpendGbp":{"in":["10","20.50"]}}`),
			wantSQL:  "c.total_spend_gbp = ANY($1::text[]::numeric[])",
			wantArgs: []any{[]string{"10", "20.50"}},
		},
		{
			name:     "decimal eq",
			filter:   parse(t, `{"commuteDistanceKm":{"eq":"12.5"}}`),
			wantSQL:  "c.commute_distance_km IS NOT DISTINCT FROM $1::numeric",
			wantArgs: []any{"12.5"},
		},
		{
			name:     "date column: ago",
			filter:   parse(t, `{"lastActiveDate":{"gte":{"ago":"7d"}}}`),
			wantSQL:  "c.last_active_date >= (now() - $1::interval)::date",
			wantArgs: []any{"7 days"},
		},
		{
			name: "date column: date",
			filter: &model.CustomerFilter{SignupDate: &model.DateFilter{
				Lt: &model.TimeValue{Date: &model.Date{Time: day}},
			}},
			wantSQL:  "c.signup_date < $1::date",
			wantArgs: []any{day},
		},
		{
			name: "date column: at",
			filter: &model.CustomerFilter{SignupDate: &model.DateFilter{
				Eq: &model.TimeValue{At: &instant},
			}},
			wantSQL:  "c.signup_date IS NOT DISTINCT FROM $1::timestamptz::date",
			wantArgs: []any{instant},
		},
		{
			name: "date column: in of time values",
			filter: &model.CustomerFilter{SignupDate: &model.DateFilter{
				In: []*model.TimeValue{{Ago: ptr("1d")}, {Date: &model.Date{Time: day}}},
			}},
			wantSQL:  "c.signup_date IN ((now() - $1::interval)::date, $2::date)",
			wantArgs: []any{"1 days", day},
		},
		{
			name:     "datetime column: ago in hours",
			filter:   parse(t, `{"lastCampaignSentAt":{"gte":{"ago":"12h"}}}`),
			wantSQL:  "c.last_campaign_sent_at >= now() - $1::interval",
			wantArgs: []any{"12 hours"},
		},
		{
			name: "datetime column: date",
			filter: &model.CustomerFilter{LastCampaignSentAt: &model.DateTimeFilter{
				Lt: &model.TimeValue{Date: &model.Date{Time: day}},
			}},
			wantSQL:  "c.last_campaign_sent_at < $1::date::timestamptz",
			wantArgs: []any{day},
		},
		{
			name: "datetime column: at",
			filter: &model.CustomerFilter{LastCampaignSentAt: &model.DateTimeFilter{
				Lte: &model.TimeValue{At: &instant},
			}},
			wantSQL:  "c.last_campaign_sent_at <= $1::timestamptz",
			wantArgs: []any{instant},
		},
		{
			name:   "relation some, every, none together are ANDed",
			filter: parse(t, `{"orders":{"some":{"totalGbp":{"gt":"50"}},"every":{"orderStatus":{"eq":"delivered"}},"none":{"orderStatus":{"eq":"refunded"}}}}`),
			wantSQL: "(EXISTS (SELECT 1 FROM dwh.fact_order o WHERE o.customer_id = c.customer_id AND (o.total_gbp > $1::numeric))" +
				" AND NOT EXISTS (SELECT 1 FROM dwh.fact_order o WHERE o.customer_id = c.customer_id AND (o.order_status IS NOT DISTINCT FROM $2))" +
				" AND NOT EXISTS (SELECT 1 FROM dwh.fact_order o WHERE o.customer_id = c.customer_id AND NOT COALESCE((o.order_status IS NOT DISTINCT FROM $3), false)))",
			wantArgs: []any{"50", "refunded", "delivered"},
		},
		{
			name:     "every alone",
			filter:   parse(t, `{"events":{"every":{"platform":{"eq":"ios"}}}}`),
			wantSQL:  "NOT EXISTS (SELECT 1 FROM dwh.fact_event e WHERE e.customer_id = c.customer_id AND NOT COALESCE((e.platform IS NOT DISTINCT FROM $1), false))",
			wantArgs: []any{"ios"},
		},
		{
			name:     "relation with nested logic and an empty inner filter",
			filter:   parse(t, `{"cartItems":{"some":{},"none":{"or":[{"cartStatus":{"eq":"open"}},{"not":{"quantity":{"gt":3}}}]}}}`),
			wantSQL:  "(EXISTS (SELECT 1 FROM dwh.fact_cart_item ci WHERE ci.customer_id = c.customer_id AND (TRUE)) AND NOT EXISTS (SELECT 1 FROM dwh.fact_cart_item ci WHERE ci.customer_id = c.customer_id AND (ci.cart_status IS NOT DISTINCT FROM $1 OR NOT COALESCE((ci.quantity > $2::bigint), false))))",
			wantArgs: []any{"open", int64(3)},
		},
		{
			name:     "free text is never validated",
			filter:   parse(t, `{"vehicleMake":{"eq":"Ford"},"timezone":{"in":["Europe/London"]}}`),
			wantSQL:  "(c.timezone = ANY($1::text[]) AND c.vehicle_make IS NOT DISTINCT FROM $2)",
			wantArgs: []any{[]string{"Europe/London"}, "Ford"},
		},
	}
}

func TestCompileGolden(t *testing.T) {
	t.Parallel()
	c := testCompiler(t)
	for _, tt := range goldenCases(t) {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := c.Compile(tt.filter)
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			if got.Where != tt.wantSQL {
				t.Errorf("Where =\n  %s\nwant\n  %s", got.Where, tt.wantSQL)
			}
			if !reflect.DeepEqual(got.Args, tt.wantArgs) {
				t.Errorf("Args = %#v, want %#v", got.Args, tt.wantArgs)
			}
		})
	}
}

func TestCompileNilFilter(t *testing.T) {
	t.Parallel()
	got, err := testCompiler(t).Compile(nil)
	if err != nil || got.Where != "TRUE" || len(got.Args) != 0 {
		t.Errorf("Compile(nil) = %+v, %v; want TRUE and no args", got, err)
	}
}

// TestCompileNeverInlinesValues checks that hostile text reaches SQL only as
// an argument (D-007, D-062).
func TestCompileNeverInlinesValues(t *testing.T) {
	t.Parallel()
	evil := `x'; DROP TABLE dwh.dim_customer; --`
	f := &model.CustomerFilter{
		Timezone:      &model.StringFilter{Eq: ptr(evil), In: []string{evil}},
		VehicleMake:   &model.StringFilter{Ne: ptr(evil)},
		Orders:        &model.OrderRelationFilter{Some: &model.OrderFilter{DiscountCode: &model.StringFilter{Eq: ptr(evil)}}},
		MaritalStatus: &model.StringFilter{Eq: ptr(`" OR 1=1`)},
	}
	got, err := testCompiler(t).Compile(f)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"DROP", "x'", "1=1", "--", "\""} {
		if strings.Contains(got.Where, bad) {
			t.Errorf("Where contains %q: %s", bad, got.Where)
		}
	}
	if n := strings.Count(got.Where, "$"); n != len(got.Args) {
		t.Errorf("%d placeholders for %d args", n, len(got.Args))
	}
}

// TestPlaceholdersMatchArgs checks the numbering is dense: $1..$n, each once.
func TestPlaceholdersMatchArgs(t *testing.T) {
	t.Parallel()
	f := parse(t, `{"and":[{"city":{"in":["Ankara"]}},{"orders":{"some":{"totalGbp":{"gt":"1"},"orderDate":{"lt":{"ago":"2d"}}}}},{"ageYears":{"in":[1,2]}}]}`)
	got, err := testCompiler(t).Compile(f)
	if err != nil {
		t.Fatal(err)
	}
	for i := range got.Args {
		ph := "$" + strconv.Itoa(i+1)
		if strings.Count(got.Where, ph+"::")+strings.Count(got.Where, ph+")")+strings.Count(got.Where, ph+" ") == 0 {
			t.Errorf("placeholder %s missing in %s", ph, got.Where)
		}
	}
	if strings.Contains(got.Where, "$"+strconv.Itoa(len(got.Args)+1)) {
		t.Errorf("more placeholders than %d args: %s", len(got.Args), got.Where)
	}
}

// codeOf returns the *Error behind err or fails the test.
func codeOf(t *testing.T, err error) *Error {
	t.Helper()
	var ae *Error
	if !errors.As(err, &ae) {
		t.Fatalf("error = %v (%T), want *audience.Error", err, err)
	}
	return ae
}

func TestCompileErrors(t *testing.T) {
	t.Parallel()
	c := testCompiler(t)
	tests := []struct {
		name     string
		filter   *model.CustomerFilter
		wantCode string
		wantPath string
	}{
		{"unknown city", parse(t, `{"city":{"eq":"Istnbul"}}`), CodeUnknownValue, "filter.city.eq"},
		{"unknown city in list", parse(t, `{"city":{"in":["Ankara","Atlantis"]}}`), CodeUnknownValue, "filter.city.in[1]"},
		{"unknown tier lower case", parse(t, `{"loyaltyTier":{"eq":"gold"}}`), CodeUnknownValue, "filter.loyaltyTier.eq"},
		{"unknown tier with ne", parse(t, `{"loyaltyTier":{"ne":"Diamond"}}`), CodeUnknownValue, "filter.loyaltyTier.ne"},
		{"unknown sex", parse(t, `{"sex":{"in":["Male","Other"]}}`), CodeUnknownValue, "filter.sex.in[1]"},
		{"unknown language nested", parse(t, `{"and":[{"language":{"eq":"de"}}]}`), CodeUnknownValue, "filter.and[0].language.eq"},
		{"unknown order status", parse(t, `{"orders":{"some":{"orderStatus":{"eq":"lost"}}}}`), CodeUnknownValue, "filter.orders.some.orderStatus.eq"},
		{"unknown cart status", parse(t, `{"cartItems":{"none":{"cartStatus":{"eq":"x"}}}}`), CodeUnknownValue, "filter.cartItems.none.cartStatus.eq"},
		{"unknown event type", parse(t, `{"events":{"every":{"eventType":{"eq":"x"}}}}`), CodeUnknownValue, "filter.events.every.eventType.eq"},
		{"unknown platform", parse(t, `{"preferredPlatform":{"eq":"windows"}}`), CodeUnknownValue, "filter.preferredPlatform.eq"},
		{"unknown country", parse(t, `{"countryCode":{"eq":"DE"}}`), CodeUnknownValue, "filter.countryCode.eq"},
		{"ago bad unit", agoFilter("30x"), CodeBadUserInput, "filter.signupDate.gte.ago"},
		{"ago zero", agoFilter("0d"), CodeBadUserInput, "filter.signupDate.gte.ago"},
		{"ago leading zero", agoFilter("030d"), CodeBadUserInput, "filter.signupDate.gte.ago"},
		{"ago no unit", agoFilter("30"), CodeBadUserInput, "filter.signupDate.gte.ago"},
		{"ago negative", agoFilter("-1d"), CodeBadUserInput, "filter.signupDate.gte.ago"},
		{"ago empty", agoFilter(""), CodeBadUserInput, "filter.signupDate.gte.ago"},
		{"ago too large", agoFilter("99999999999999999999d"), CodeBadUserInput, "filter.signupDate.gte.ago"},
		{"ago days over cap", agoFilter("36501d"), CodeBadUserInput, "filter.signupDate.gte.ago"},
		{"empty string in", parse(t, `{"city":{"in":[]}}`), CodeBadUserInput, "filter.city.in"},
		{"empty int in", parse(t, `{"ageYears":{"in":[]}}`), CodeBadUserInput, "filter.ageYears.in"},
		{"empty and", parse(t, `{"and":[]}`), CodeBadUserInput, "filter.and"},
		{"empty or", parse(t, `{"or":[]}`), CodeBadUserInput, "filter.or"},
		{"field without operator", parse(t, `{"city":{}}`), CodeBadUserInput, "filter.city"},
		{"relation without quantifier", parse(t, `{"orders":{}}`), CodeBadUserInput, "filter.orders"},
		{"time value without a field", &model.CustomerFilter{SignupDate: &model.DateFilter{Eq: &model.TimeValue{}}}, CodeBadUserInput, "filter.signupDate.eq"},
		{"in over the limit", parse(t, `{"ageYears":{"in":[`+strings.TrimSuffix(strings.Repeat("1,", 101), ",")+`]}}`), CodeComplexityLimit, "filter.ageYears.in"},
		{"relation inside a relation filter path", parse(t, `{"orders":{"some":{"and":[{"orderStatus":{"eq":"x"}}]}}}`), CodeUnknownValue, "filter.orders.some.and[0].orderStatus.eq"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := c.Compile(tt.filter)
			ae := codeOf(t, err)
			if ae.Code != tt.wantCode {
				t.Errorf("code = %s, want %s (%s)", ae.Code, tt.wantCode, ae.Message)
			}
			if got := FormatPath(ae.FilterPath); got != tt.wantPath {
				t.Errorf("path = %s, want %s", got, tt.wantPath)
			}
		})
	}
}

func agoFilter(ago string) *model.CustomerFilter {
	return &model.CustomerFilter{SignupDate: &model.DateFilter{Gte: &model.TimeValue{Ago: &ago}}}
}

func TestUnknownValueExtensions(t *testing.T) {
	t.Parallel()
	c := testCompiler(t)

	_, err := c.Compile(parse(t, `{"city":{"eq":"Istnbul"}}`))
	ae := codeOf(t, err)
	if ae.Extensions["field"] != "city" || ae.Extensions["value"] != "Istnbul" {
		t.Errorf("extensions = %v, want field city and value Istnbul", ae.Extensions)
	}
	if got, _ := ae.Extensions["suggestions"].([]string); len(got) != 1 || got[0] != "İstanbul" {
		t.Errorf("suggestions = %v, want [İstanbul]", ae.Extensions["suggestions"])
	}

	_, err = c.Compile(parse(t, `{"loyaltyTier":{"eq":"gold"}}`))
	ae = codeOf(t, err)
	if got, _ := ae.Extensions["suggestions"].([]string); len(got) != 1 || got[0] != "Gold" {
		t.Errorf("suggestions = %v, want [Gold]", ae.Extensions["suggestions"])
	}

	_, err = c.Compile(parse(t, `{"city":{"eq":"Atlantis"}}`))
	ae = codeOf(t, err)
	if got, ok := ae.Extensions["suggestions"].([]string); !ok || got == nil || len(got) != 0 {
		t.Errorf("suggestions = %#v, want an empty non-nil list", ae.Extensions["suggestions"])
	}
}

// notChain wraps a one-comparison filter in n nested not objects.
func notChain(n int) *model.CustomerFilter {
	f := &model.CustomerFilter{Sessions90d: &model.IntFilter{Eq: ptr(1)}}
	for range n {
		f = &model.CustomerFilter{Not: f}
	}
	return f
}

func TestDepthLimit(t *testing.T) {
	t.Parallel()
	c := testCompiler(t)
	// The root is depth 1 and each not adds one: 7 nots reach depth 8, 8 nots depth 9.
	if _, err := c.Compile(notChain(MaxDepth - 1)); err != nil {
		t.Fatalf("depth %d: error = %v, want ok", MaxDepth, err)
	}
	_, err := c.Compile(notChain(MaxDepth))
	if ae := codeOf(t, err); ae.Code != CodeComplexityLimit {
		t.Fatalf("depth %d: code = %s, want %s", MaxDepth+1, ae.Code, CodeComplexityLimit)
	}

	t.Run("relation levels count", func(t *testing.T) {
		t.Parallel()
		// root 1, orders.some filter 2, then nots up to depth 8 and one over.
		build := func(nots int) *model.CustomerFilter {
			o := &model.OrderFilter{ItemsCount: &model.IntFilter{Eq: ptr(1)}}
			for range nots {
				o = &model.OrderFilter{Not: o}
			}
			return &model.CustomerFilter{Orders: &model.OrderRelationFilter{Some: o}}
		}
		if _, err := c.Compile(build(MaxDepth - 2)); err != nil {
			t.Fatalf("depth %d: error = %v, want ok", MaxDepth, err)
		}
		_, err := c.Compile(build(MaxDepth - 1))
		if ae := codeOf(t, err); ae.Code != CodeComplexityLimit {
			t.Fatalf("depth %d: code = %s, want %s", MaxDepth+1, ae.Code, CodeComplexityLimit)
		}
	})
}

func TestNodeLimit(t *testing.T) {
	t.Parallel()
	c := testCompiler(t)
	or := func(n int) *model.CustomerFilter {
		f := &model.CustomerFilter{}
		for range n {
			f.Or = append(f.Or, &model.CustomerFilter{Sessions90d: &model.IntFilter{Eq: ptr(1)}})
		}
		return f
	}
	// One node for the or key plus one per eq: 99 elements make exactly 100.
	if _, err := c.Compile(or(MaxNodes - 1)); err != nil {
		t.Fatalf("%d nodes: error = %v, want ok", MaxNodes, err)
	}
	_, err := c.Compile(or(MaxNodes))
	if ae := codeOf(t, err); ae.Code != CodeComplexityLimit {
		t.Fatalf("%d nodes: code = %s, want %s", MaxNodes+1, ae.Code, CodeComplexityLimit)
	}
}

func TestInLimitBoundary(t *testing.T) {
	t.Parallel()
	vals := make([]string, MaxInValues)
	for i := range vals {
		vals[i] = "v" + strconv.Itoa(i)
	}
	got, err := testCompiler(t).Compile(&model.CustomerFilter{Timezone: &model.StringFilter{In: vals}})
	if err != nil {
		t.Fatalf("%d values: error = %v, want ok", MaxInValues, err)
	}
	if arr, _ := got.Args[0].([]string); len(arr) != MaxInValues {
		t.Errorf("array arg has %d values, want %d", len(arr), MaxInValues)
	}
	_, err = testCompiler(t).Compile(&model.CustomerFilter{Timezone: &model.StringFilter{In: append(vals, "x")}})
	if ae := codeOf(t, err); ae.Code != CodeComplexityLimit {
		t.Fatalf("%d values: code = %s, want %s", MaxInValues+1, ae.Code, CodeComplexityLimit)
	}
}

func TestAgoInterval(t *testing.T) {
	t.Parallel()
	tests := []struct{ in, want string }{
		{"30d", "30 days"}, {"1d", "1 days"}, {"12h", "12 hours"}, {"36500d", "36500 days"}, {"876000h", "876000 hours"},
	}
	for _, tt := range tests {
		got, err := agoInterval(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("agoInterval(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
}
