//go:build integration

package graph_test

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furknozg/coldbru/internal/customer/audience"
	"github.com/furknozg/coldbru/internal/customer/graph"
	"github.com/furknozg/coldbru/internal/customer/pgtest"
)

// pool is the customer_svc pool of the seeded container shared by the package.
var pool *pgxpool.Pool

func TestMain(m *testing.M) { os.Exit(run(m)) }

func run(m *testing.M) int {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	env, err := pgtest.Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start seeded postgres: %v\n", err)
		return 1
	}
	defer env.Close()
	pool = env.Pool
	return m.Run()
}

// liveServer is the real handler over the seeded database with the city
// dictionary loaded the way cmd/customer does it.
func liveServer(t *testing.T) http.Handler {
	t.Helper()
	cities, err := audience.LoadCities(t.Context(), pool)
	if err != nil {
		t.Fatalf("LoadCities: %v", err)
	}
	svc := audience.NewService(pool, cities)
	return graph.NewHandler(graph.NewResolver(svc, slog.New(slog.NewTextHandler(io.Discard, nil))))
}

type coverage struct {
	Both        int `json:"both"`
	EmailOnly   int `json:"emailOnly"`
	SmsOnly     int `json:"smsOnly"`
	Unreachable int `json:"unreachable"`
}

type langCoverage struct {
	Language string   `json:"language"`
	Count    int      `json:"count"`
	Coverage coverage `json:"coverage"`
}

type previewData struct {
	AudiencePreview struct {
		Count      int            `json:"count"`
		Coverage   coverage       `json:"coverage"`
		ByLanguage []langCoverage `json:"byLanguage"`
	} `json:"audiencePreview"`
}

const fullPreviewQuery = `query($f: CustomerFilter!) {
  audiencePreview(filter: $f) { count coverage { both emailOnly smsOnly unreachable } byLanguage { language count coverage { both emailOnly smsOnly unreachable } } }
}`

func preview(t *testing.T, h http.Handler, filter map[string]any) previewData {
	t.Helper()
	status, resp := post(t, h, fullPreviewQuery, map[string]any{"f": filter})
	if status != http.StatusOK || len(resp.Errors) > 0 {
		t.Fatalf("status = %d, errors = %+v", status, resp.Errors)
	}
	var d previewData
	if err := json.Unmarshal(resp.Data, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func cityFilter(city string) map[string]any {
	return map[string]any{"city": map[string]any{"eq": city}}
}

// sqlCount runs a count query as the service role.
func sqlCount(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(t.Context(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// reachableSQL is the D-062 rule written out independently of the package.
const reachableSQL = `c.account_status = 'active' AND
  ((c.email IS NOT NULL AND c.email_opt_in AND NOT c.email_bounced) OR (c.phone_e164 IS NOT NULL AND c.sms_opt_in))`

func TestSampleBriefAudience(t *testing.T) {
	h := liveServer(t)
	d := preview(t, h, sampleFilter()).AudiencePreview

	want := coverage{Both: 20, EmailOnly: 13, SmsOnly: 13, Unreachable: 3}
	if d.Count != 46 || d.Coverage != want {
		t.Errorf("count = %d, coverage = %+v; want 46 and %+v (D-010, D-062)", d.Count, d.Coverage, want)
	}
	if sms := d.Coverage.Both + d.Coverage.SmsOnly; sms != 33 {
		t.Errorf("SMS-eligible = %d, want 33", sms)
	}
	if email := d.Coverage.Both + d.Coverage.EmailOnly; email != 33 {
		t.Errorf("email-eligible = %d, want 33", email)
	}
	if d.Count != d.Coverage.Both+d.Coverage.EmailOnly+d.Coverage.SmsOnly {
		t.Errorf("count %d is not both + emailOnly + smsOnly", d.Count)
	}

	// The language split adds up to the totals.
	var sum coverage
	langs := 0
	for _, l := range d.ByLanguage {
		sum.Both += l.Coverage.Both
		sum.EmailOnly += l.Coverage.EmailOnly
		sum.SmsOnly += l.Coverage.SmsOnly
		sum.Unreachable += l.Coverage.Unreachable
		langs += l.Count
	}
	if sum != d.Coverage || langs != d.Count {
		t.Errorf("byLanguage sums to %+v / %d, want %+v / %d", sum, langs, d.Coverage, d.Count)
	}
	if !slices.IsSortedFunc(d.ByLanguage, func(a, b langCoverage) int { return cmp.Compare(a.Language, b.Language) }) {
		t.Errorf("byLanguage is not ordered by language: %+v", d.ByLanguage)
	}
}

func TestCityMatchingIgnoresCaseAndDots(t *testing.T) {
	h := liveServer(t)
	want := sqlCount(t, `SELECT count(*) FROM dwh.dim_customer c WHERE c.city = 'İstanbul' AND `+reachableSQL)
	if want == 0 {
		t.Fatal("no reachable customers in İstanbul: seed changed?")
	}
	for _, spelling := range []string{"İstanbul", "istanbul", "ISTANBUL", "Istanbul", "ıstanbul"} {
		t.Run(spelling, func(t *testing.T) {
			if got := preview(t, h, cityFilter(spelling)).AudiencePreview.Count; got != want {
				t.Errorf("count = %d, want %d (the SQL count for İstanbul)", got, want)
			}
		})
	}
	for _, c := range []struct{ typed, stored string }{{"ESKISEHIR", "Eskişehir"}, {"DIYARBAKIR", "Diyarbakır"}, {"mugla", "Muğla"}} {
		want := sqlCount(t, `SELECT count(*) FROM dwh.dim_customer c WHERE c.city = $1 AND `+reachableSQL, c.stored)
		if got := preview(t, h, cityFilter(c.typed)).AudiencePreview.Count; got != want || want == 0 {
			t.Errorf("%s: count = %d, want %d (> 0)", c.typed, got, want)
		}
	}
	t.Run("in list", func(t *testing.T) {
		want := sqlCount(t, `SELECT count(*) FROM dwh.dim_customer c WHERE c.city IN ('İstanbul','Ankara') AND `+reachableSQL)
		f := map[string]any{"city": map[string]any{"in": []any{"ISTANBUL", "ankara"}}}
		if got := preview(t, h, f).AudiencePreview.Count; got != want {
			t.Errorf("count = %d, want %d", got, want)
		}
	})
}

func TestUnknownValuesAreRejectedWithSuggestions(t *testing.T) {
	h := liveServer(t)
	tests := []struct {
		name       string
		filter     map[string]any
		field      string
		suggestion string
	}{
		{"misspelled city", cityFilter("Istnbul"), "city", "İstanbul"},
		{"lower case tier", map[string]any{"loyaltyTier": map[string]any{"eq": "gold"}}, "loyaltyTier", "Gold"},
		{"relation status", map[string]any{"orders": map[string]any{"some": map[string]any{"orderStatus": map[string]any{"eq": "deliverd"}}}}, "orderStatus", "delivered"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, resp := post(t, h, previewQuery, map[string]any{"f": tt.filter})
			if status != http.StatusOK {
				t.Errorf("status = %d, want 200", status)
			}
			wantCode(t, resp, graph.CodeUnknownValue)
			ext := resp.Errors[0].Extensions
			if ext["field"] != tt.field {
				t.Errorf("field = %v, want %s", ext["field"], tt.field)
			}
			sugg, _ := ext["suggestions"].([]any)
			if !slices.Contains(sugg, any(tt.suggestion)) {
				t.Errorf("suggestions = %v, want to contain %q", sugg, tt.suggestion)
			}
		})
	}
}

type recipient struct {
	CustomerID    string  `json:"customerId"`
	FirstName     string  `json:"firstName"`
	Language      string  `json:"language"`
	EmailEligible bool    `json:"emailEligible"`
	SmsEligible   bool    `json:"smsEligible"`
	Email         *string `json:"email"`
	Phone         *string `json:"phone"`
}

type recipientsData struct {
	AudienceRecipients struct {
		Nodes    []recipient `json:"nodes"`
		PageInfo struct {
			HasNextPage bool    `json:"hasNextPage"`
			EndCursor   *string `json:"endCursor"`
		} `json:"pageInfo"`
	} `json:"audienceRecipients"`
}

const fullRecipientsQuery = `query($f: CustomerFilter!, $first: Int!, $after: String) {
  audienceRecipients(filter: $f, first: $first, after: $after) {
    nodes { customerId firstName language emailEligible smsEligible email phone }
    pageInfo { hasNextPage endCursor }
  }
}`

func TestRecipientsPagingCoversTheAudienceOnce(t *testing.T) {
	h := liveServer(t)

	var all []recipient
	var after *string
	pages := 0
	for {
		vars := map[string]any{"f": sampleFilter(), "first": 10, "after": after}
		status, resp := post(t, h, fullRecipientsQuery, vars)
		if status != http.StatusOK || len(resp.Errors) > 0 {
			t.Fatalf("page %d: status = %d, errors = %+v", pages, status, resp.Errors)
		}
		var d recipientsData
		if err := json.Unmarshal(resp.Data, &d); err != nil {
			t.Fatal(err)
		}
		page := d.AudienceRecipients
		pages++
		if len(page.Nodes) == 0 || len(page.Nodes) > 10 {
			t.Fatalf("page %d has %d nodes, want 1..10", pages, len(page.Nodes))
		}
		all = append(all, page.Nodes...)
		if want := page.Nodes[len(page.Nodes)-1].CustomerID; page.PageInfo.EndCursor == nil ||
			*page.PageInfo.EndCursor != audience.EncodeCursor(want) {
			t.Fatalf("page %d endCursor = %v, want the cursor of %s", pages, page.PageInfo.EndCursor, want)
		}
		if !page.PageInfo.HasNextPage {
			break
		}
		if len(page.Nodes) != 10 {
			t.Fatalf("page %d is short (%d) but hasNextPage is true", pages, len(page.Nodes))
		}
		after = page.PageInfo.EndCursor
		if pages > 10 {
			t.Fatal("paging does not end")
		}
	}

	if len(all) != 46 || pages != 5 {
		t.Errorf("got %d recipients in %d pages, want 46 in 5", len(all), pages)
	}
	seen := map[string]bool{}
	smsOK, emailOK := 0, 0
	for i, r := range all {
		if i > 0 && all[i-1].CustomerID >= r.CustomerID {
			t.Errorf("customerId %s after %s: not strictly increasing", r.CustomerID, all[i-1].CustomerID)
		}
		if seen[r.CustomerID] {
			t.Errorf("duplicate %s", r.CustomerID)
		}
		seen[r.CustomerID] = true
		if !r.EmailEligible && !r.SmsEligible {
			t.Errorf("%s is reachable on no channel", r.CustomerID)
		}
		if (r.Email != nil) != r.EmailEligible {
			t.Errorf("%s: email present = %v, emailEligible = %v", r.CustomerID, r.Email != nil, r.EmailEligible)
		}
		if (r.Phone != nil) != r.SmsEligible {
			t.Errorf("%s: phone present = %v, smsEligible = %v", r.CustomerID, r.Phone != nil, r.SmsEligible)
		}
		if r.EmailEligible {
			emailOK++
		}
		if r.SmsEligible {
			smsOK++
		}
	}
	if emailOK != 33 || smsOK != 33 {
		t.Errorf("email-eligible = %d, SMS-eligible = %d, want 33 and 33", emailOK, smsOK)
	}

	// The same people an independent SQL query finds for the sample brief.
	rows, err := pool.Query(t.Context(), `
		SELECT c.customer_id FROM dwh.dim_customer c
		WHERE c.city = 'İstanbul' AND `+reachableSQL+`
		  AND EXISTS (SELECT 1 FROM dwh.fact_cart_item ci WHERE ci.customer_id = c.customer_id AND ci.added_at >= now() - interval '30 days')
		  AND NOT EXISTS (SELECT 1 FROM dwh.fact_order o WHERE o.customer_id = c.customer_id AND o.order_date >= now() - interval '30 days')
		ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var direct []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		direct = append(direct, id)
	}
	got := make([]string, len(all))
	for i, r := range all {
		got[i] = r.CustomerID
	}
	if !slices.Equal(got, direct) {
		t.Errorf("GraphQL recipients differ from the direct SQL result\n got: %v\nwant: %v", got, direct)
	}
}

func TestRecipientsLastPageAndEmptyAudience(t *testing.T) {
	h := liveServer(t)
	t.Run("one page holds everything", func(t *testing.T) {
		_, resp := post(t, h, fullRecipientsQuery, map[string]any{"f": sampleFilter(), "first": 1000})
		var d recipientsData
		if err := json.Unmarshal(resp.Data, &d); err != nil {
			t.Fatal(err)
		}
		if n := len(d.AudienceRecipients.Nodes); n != 46 || d.AudienceRecipients.PageInfo.HasNextPage {
			t.Errorf("nodes = %d, hasNextPage = %v; want 46, false", n, d.AudienceRecipients.PageInfo.HasNextPage)
		}
	})
	t.Run("no match", func(t *testing.T) {
		f := map[string]any{"ageYears": map[string]any{"gt": 200}}
		_, resp := post(t, h, fullRecipientsQuery, map[string]any{"f": f, "first": 10})
		var d recipientsData
		if err := json.Unmarshal(resp.Data, &d); err != nil {
			t.Fatal(err)
		}
		p := d.AudienceRecipients
		if len(p.Nodes) != 0 || p.PageInfo.HasNextPage || p.PageInfo.EndCursor != nil {
			t.Errorf("page = %+v, want empty, no next page, null cursor", p)
		}
		pv := preview(t, h, f).AudiencePreview
		if pv.Count != 0 || pv.Coverage != (coverage{}) || len(pv.ByLanguage) != 0 {
			t.Errorf("preview = %+v, want all zero", pv)
		}
	})
}

func TestNullSemantics(t *testing.T) {
	h := liveServer(t)
	total := sqlCount(t, `SELECT count(*) FROM dwh.dim_customer c WHERE `+reachableSQL)
	withNull := sqlCount(t, `SELECT count(*) FROM dwh.dim_customer c WHERE c.favourite_category IS NULL AND `+reachableSQL)
	nonNull := sqlCount(t, `SELECT count(*) FROM dwh.dim_customer c WHERE c.favourite_category <> 'X' AND `+reachableSQL)
	if withNull == 0 {
		t.Fatal("no reachable customer has a NULL favourite_category: the test would prove nothing")
	}
	if total != withNull+nonNull {
		t.Fatalf("sanity: total %d != null %d + non-null %d", total, withNull, nonNull)
	}

	t.Run("ne includes NULL", func(t *testing.T) {
		f := map[string]any{"favouriteCategory": map[string]any{"ne": "X"}}
		if got := preview(t, h, f).AudiencePreview.Count; got != total {
			t.Errorf("count = %d, want %d (plain SQL <> would give %d)", got, total, nonNull)
		}
	})
	t.Run("eq is not distinct", func(t *testing.T) {
		f := map[string]any{"favouriteCategory": map[string]any{"eq": "Fashion"}}
		want := sqlCount(t, `SELECT count(*) FROM dwh.dim_customer c WHERE c.favourite_category IS NOT DISTINCT FROM 'Fashion' AND `+reachableSQL)
		if got := preview(t, h, f).AudiencePreview.Count; got != want {
			t.Errorf("count = %d, want %d", got, want)
		}
	})
	t.Run("not of eq includes NULL", func(t *testing.T) {
		f := map[string]any{"not": map[string]any{"favouriteCategory": map[string]any{"eq": "X"}}}
		if got := preview(t, h, f).AudiencePreview.Count; got != total {
			t.Errorf("count = %d, want %d", got, total)
		}
	})
	t.Run("ordering never matches NULL", func(t *testing.T) {
		f := map[string]any{"dependants": map[string]any{"gte": 0}}
		want := sqlCount(t, `SELECT count(*) FROM dwh.dim_customer c WHERE c.dependants >= 0 AND `+reachableSQL)
		allCount := sqlCount(t, `SELECT count(*) FROM dwh.dim_customer c WHERE `+reachableSQL)
		if got := preview(t, h, f).AudiencePreview.Count; got != want {
			t.Errorf("count = %d, want %d (all reachable: %d)", got, want, allCount)
		}
	})
	t.Run("every is true without related rows", func(t *testing.T) {
		f := map[string]any{"orders": map[string]any{"every": map[string]any{"orderStatus": map[string]any{"eq": "delivered"}}}}
		want := sqlCount(t, `SELECT count(*) FROM dwh.dim_customer c WHERE `+reachableSQL+
			` AND NOT EXISTS (SELECT 1 FROM dwh.fact_order o WHERE o.customer_id = c.customer_id AND o.order_status IS DISTINCT FROM 'delivered')`)
		if got := preview(t, h, f).AudiencePreview.Count; got != want {
			t.Errorf("count = %d, want %d", got, want)
		}
	})
}

func TestInvalidInputThroughTheHandler(t *testing.T) {
	h := liveServer(t)
	t.Run("ago 30x", func(t *testing.T) {
		f := map[string]any{"signupDate": map[string]any{"gte": map[string]any{"ago": "30x"}}}
		_, resp := post(t, h, previewQuery, map[string]any{"f": f})
		wantCode(t, resp, graph.CodeBadUserInput)
	})
	t.Run("101 values in an in list", func(t *testing.T) {
		vals := make([]any, 101)
		for i := range vals {
			vals[i] = fmt.Sprintf("Europe/City%d", i)
		}
		f := map[string]any{"timezone": map[string]any{"in": vals}}
		_, resp := post(t, h, previewQuery, map[string]any{"f": f})
		wantCode(t, resp, graph.CodeComplexityLimit)
	})
	t.Run("100 values are fine", func(t *testing.T) {
		vals := make([]any, 100)
		for i := range vals {
			vals[i] = fmt.Sprintf("Europe/City%d", i)
		}
		f := map[string]any{"timezone": map[string]any{"in": vals}}
		if got := preview(t, h, f).AudiencePreview.Count; got != 0 {
			t.Errorf("count = %d, want 0", got)
		}
	})
	t.Run("nesting over 8 levels", func(t *testing.T) {
		f := map[string]any{"sessions90d": map[string]any{"gte": 0}}
		for range 8 {
			f = map[string]any{"not": f}
		}
		_, resp := post(t, h, previewQuery, map[string]any{"f": f})
		wantCode(t, resp, graph.CodeComplexityLimit)
	})
}

func TestLoadCities(t *testing.T) {
	cities, err := audience.LoadCities(t.Context(), pool)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"İstanbul", "Eskişehir", "Diyarbakır", "Muğla", "London"} {
		if got, ok := cities.Lookup(audience.Key(name)); !ok || got != name {
			t.Errorf("Lookup(%q) = %q, %v; want the stored spelling", name, got, ok)
		}
	}
	want := sqlCount(t, `SELECT count(DISTINCT city) FROM dwh.dim_customer`)
	if got := len(cities.Names()); got != want {
		t.Errorf("%d cities loaded, want %d", got, want)
	}
}
