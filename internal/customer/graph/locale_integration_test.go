//go:build integration

package graph_test

import (
	"cmp"
	"encoding/json"
	"net/http"
	"slices"
	"testing"
)

type localeCoverage struct {
	Language           string   `json:"language"`
	Timezone           string   `json:"timezone"`
	Count              int      `json:"count"`
	Coverage           coverage `json:"coverage"`
	MaxFirstNameLength int      `json:"maxFirstNameLength"`
}

type localePreviewData struct {
	AudiencePreview struct {
		Count      int              `json:"count"`
		Coverage   coverage         `json:"coverage"`
		ByLanguage []langCoverage   `json:"byLanguage"`
		ByLocale   []localeCoverage `json:"byLocale"`
	} `json:"audiencePreview"`
}

const localePreviewQuery = `query($f: CustomerFilter!) {
  audiencePreview(filter: $f) {
    count coverage { both emailOnly smsOnly unreachable }
    byLanguage { language count maxFirstNameLength coverage { both emailOnly smsOnly unreachable } }
    byLocale { language timezone count maxFirstNameLength coverage { both emailOnly smsOnly unreachable } }
  }
}`

func localePreview(t *testing.T, filter map[string]any) localePreviewData {
	t.Helper()
	status, resp := post(t, liveServer(t), localePreviewQuery, map[string]any{"f": filter})
	if status != http.StatusOK || len(resp.Errors) > 0 {
		t.Fatalf("status = %d, errors = %+v", status, resp.Errors)
	}
	var d localePreviewData
	if err := json.Unmarshal(resp.Data, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

// TestSampleBriefByLocale checks the (language, timezone) split of the sample brief against a
// direct SQL group by, and that it adds up to the totals and to byLanguage (D-076).
func TestSampleBriefByLocale(t *testing.T) {
	d := localePreview(t, sampleFilter()).AudiencePreview

	sum, n := coverage{}, 0
	for _, l := range d.ByLocale {
		sum.Both += l.Coverage.Both
		sum.EmailOnly += l.Coverage.EmailOnly
		sum.SmsOnly += l.Coverage.SmsOnly
		sum.Unreachable += l.Coverage.Unreachable
		n += l.Count
	}
	if n != 138 || d.Count != 138 || sum != d.Coverage { // 138 reachable with the 3000-user seed (D-113)
		t.Errorf("byLocale counts sum to %d (coverage %+v), want 138 and %+v", n, sum, d.Coverage)
	}
	if !slices.IsSortedFunc(d.ByLocale, func(a, b localeCoverage) int {
		return cmp.Or(cmp.Compare(a.Language, b.Language), cmp.Compare(a.Timezone, b.Timezone))
	}) {
		t.Errorf("byLocale is not ordered by language then timezone: %+v", d.ByLocale)
	}

	// Direct SQL over the same audience, written independently of the package.
	const sampleSQL = `FROM dwh.dim_customer c
		WHERE c.city = 'İstanbul' AND c.account_status = 'active'
		  AND EXISTS (SELECT 1 FROM dwh.fact_cart_item ci WHERE ci.customer_id = c.customer_id AND ci.added_at >= now() - interval '30 days')
		  AND NOT EXISTS (SELECT 1 FROM dwh.fact_order o WHERE o.customer_id = c.customer_id AND o.order_date >= now() - interval '30 days')
		GROUP BY c.language, c.timezone ORDER BY c.language, c.timezone`
	rows, err := pool.Query(t.Context(), `SELECT c.language, c.timezone,
		count(*) FILTER (WHERE `+reachableSQL+`),
		COALESCE(max(char_length(c.first_name)) FILTER (WHERE `+reachableSQL+`), 0)::int `+sampleSQL)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var want []localeCoverage
	for rows.Next() {
		var l localeCoverage
		if err := rows.Scan(&l.Language, &l.Timezone, &l.Count, &l.MaxFirstNameLength); err != nil {
			t.Fatal(err)
		}
		want = append(want, l)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(want) != len(d.ByLocale) {
		t.Fatalf("byLocale has %d pairs, direct SQL group by has %d: %+v vs %+v", len(d.ByLocale), len(want), d.ByLocale, want)
	}
	for i, w := range want {
		g := d.ByLocale[i]
		if g.Language != w.Language || g.Timezone != w.Timezone || g.Count != w.Count || g.MaxFirstNameLength != w.MaxFirstNameLength {
			t.Errorf("pair %d = %+v, want %+v", i, g, w)
		}
		if g.Coverage.Both+g.Coverage.EmailOnly+g.Coverage.SmsOnly != g.Count {
			t.Errorf("pair %s/%s: coverage %+v does not add up to count %d", g.Language, g.Timezone, g.Coverage, g.Count)
		}
	}

	// byLanguage is the fold of byLocale.
	for _, l := range d.ByLanguage {
		got := 0
		for _, p := range d.ByLocale {
			if p.Language == l.Language {
				got += p.Count
			}
		}
		if got != l.Count {
			t.Errorf("language %s: byLocale pairs sum to %d, byLanguage says %d", l.Language, got, l.Count)
		}
	}
	// The sample brief's audience is all in İstanbul (D-076).
	for _, p := range d.ByLocale {
		if p.Timezone != "Europe/Istanbul" {
			t.Errorf("sample brief pair %+v is not in Europe/Istanbul", p)
		}
	}
}

// TestByLocaleSpansBothZones uses a broad filter whose audience lives in several time zones and
// compares every pair with SQL; pairs with only unreachable customers are listed with count 0.
func TestByLocaleSpansBothZones(t *testing.T) {
	d := localePreview(t, map[string]any{"totalOrders": map[string]any{"gte": 1}}).AudiencePreview
	pairs := map[[2]string]int{}
	for _, l := range d.ByLocale {
		pairs[[2]string{l.Language, l.Timezone}] = l.Count
	}
	if len(pairs) != 4 {
		t.Fatalf("pairs = %v, want tr/en x Europe/Istanbul/Europe/London", pairs)
	}
	for k, got := range pairs {
		want := sqlCount(t, `SELECT count(*) FROM dwh.dim_customer c WHERE c.total_orders >= 1 AND c.language = $1 AND c.timezone = $2 AND `+reachableSQL, k[0], k[1])
		if got != want {
			t.Errorf("%v: count = %d, SQL = %d", k, got, want)
		}
	}
}

// TestRecipientTimezone checks Recipient.timezone against the stored column.
func TestRecipientTimezone(t *testing.T) {
	status, resp := post(t, liveServer(t), `query($f: CustomerFilter!) {
  audienceRecipients(filter: $f, first: 1000) { nodes { customerId timezone } }
}`, map[string]any{"f": map[string]any{"totalOrders": map[string]any{"gte": 1}}})
	if status != http.StatusOK || len(resp.Errors) > 0 {
		t.Fatalf("status = %d, errors = %+v", status, resp.Errors)
	}
	var d struct {
		AudienceRecipients struct {
			Nodes []struct{ CustomerID, Timezone string } `json:"nodes"`
		} `json:"audienceRecipients"`
	}
	if err := json.Unmarshal(resp.Data, &d); err != nil {
		t.Fatal(err)
	}
	zones := map[string]bool{}
	for _, r := range d.AudienceRecipients.Nodes {
		var tz string
		if err := pool.QueryRow(t.Context(), `SELECT timezone FROM dwh.dim_customer WHERE customer_id = $1`, r.CustomerID).Scan(&tz); err != nil {
			t.Fatal(err)
		}
		if r.Timezone != tz {
			t.Errorf("%s: timezone = %q, stored %q", r.CustomerID, r.Timezone, tz)
		}
		zones[r.Timezone] = true
	}
	if len(d.AudienceRecipients.Nodes) == 0 || len(zones) < 2 {
		t.Errorf("got %d recipients in zones %v, want several zones", len(d.AudienceRecipients.Nodes), zones)
	}
}
