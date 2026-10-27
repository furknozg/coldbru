//go:build integration

package consent_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furknozg/coldbru/internal/customer/audience"
	"github.com/furknozg/coldbru/internal/customer/auth"
	"github.com/furknozg/coldbru/internal/customer/consent"
	"github.com/furknozg/coldbru/internal/customer/graph"
	"github.com/furknozg/coldbru/internal/customer/pgtest"
)

const (
	notificationToken = "notification-test-token"
	adminToken        = "admin-test-token"
	proposalToken     = "proposal-test-token"
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

// server is the real handler, behind the token middleware, over the seeded database.
func server(t *testing.T) http.Handler {
	t.Helper()
	cities, err := audience.LoadCities(t.Context(), pool)
	if err != nil {
		t.Fatalf("LoadCities: %v", err)
	}
	r := graph.NewResolver(audience.NewService(pool, cities), consent.NewService(pool),
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	set, err := auth.NewTokenSet(map[string]string{
		auth.CallerProposal: proposalToken, auth.CallerNotification: notificationToken, auth.CallerAdmin: adminToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	return set.Middleware(graph.NewHandler(r))
}

type gqlError struct {
	Message    string         `json:"message"`
	Path       []any          `json:"path"`
	Extensions map[string]any `json:"extensions"`
}

type response struct {
	Data   json.RawMessage `json:"data"`
	Errors []gqlError      `json:"errors"`
}

// post sends a GraphQL request and decodes the envelope. It is safe to call from several goroutines.
func post(t *testing.T, h http.Handler, token, query string, vars map[string]any) response {
	t.Helper()
	body, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		t.Error(err)
		return response{}
	}
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var resp response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Errorf("decode %q: %v", rec.Body.String(), err)
	}
	return resp
}

type status struct {
	CustomerID string `json:"customerId"`
	Channel    string `json:"channel"`
	OptedIn    bool   `json:"optedIn"`
	Eligible   bool   `json:"eligible"`
}

const statusQuery = `query($ids: [ID!]!, $ch: String!) { consentStatus(customerIds: $ids, channel: $ch) { customerId channel optedIn eligible } }`

func consentStatus(t *testing.T, h http.Handler, ids []string, channel string) []status {
	t.Helper()
	resp := post(t, h, notificationToken, statusQuery, map[string]any{"ids": ids, "ch": channel})
	if len(resp.Errors) > 0 {
		t.Fatalf("consentStatus errors: %+v", resp.Errors)
	}
	var d struct {
		ConsentStatus []status `json:"consentStatus"`
	}
	if err := json.Unmarshal(resp.Data, &d); err != nil {
		t.Fatal(err)
	}
	return d.ConsentStatus
}

const updateMutation = `mutation($i: UpdateConsentInput!) { updateConsent(input: $i) { customerId channel optedIn eligible } }`

func updateVars(id, channel string, optedIn bool, source string) map[string]any {
	return map[string]any{"i": map[string]any{"customerId": id, "channel": channel, "optedIn": optedIn, "source": source}}
}

func updateConsent(t *testing.T, h http.Handler, id, channel string, optedIn bool) status {
	t.Helper()
	resp := post(t, h, adminToken, updateMutation, updateVars(id, channel, optedIn, "admin_tool"))
	if len(resp.Errors) > 0 {
		t.Fatalf("updateConsent errors: %+v", resp.Errors)
	}
	var d struct {
		UpdateConsent status `json:"updateConsent"`
	}
	if err := json.Unmarshal(resp.Data, &d); err != nil {
		t.Fatal(err)
	}
	return d.UpdateConsent
}

func wantCode(t *testing.T, resp response, code string) {
	t.Helper()
	if len(resp.Errors) != 1 || resp.Errors[0].Extensions["code"] != code {
		t.Fatalf("errors = %+v, want exactly one with code %s", resp.Errors, code)
	}
}

// pick returns the first customer id (by id) matching where, failing if none does.
func pick(t *testing.T, where string, args ...any) string {
	t.Helper()
	var id string
	err := pool.QueryRow(t.Context(),
		"SELECT customer_id FROM dwh.dim_customer c WHERE "+where+" ORDER BY customer_id LIMIT 1", args...).Scan(&id)
	if err != nil {
		t.Fatalf("no customer for %q: %v", where, err)
	}
	return id
}

func count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(t.Context(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

func maxConsentNumber(t *testing.T) int {
	t.Helper()
	return count(t, `SELECT max(substring(consent_id from '^CNS-([0-9]+)$')::int) FROM dwh.fact_consent`)
}

// flagsMatchHistory is the seed's consistency check (03_customer_seed.sql): the
// number of customers whose opt-in flags differ from their latest fact_consent
// row. It must always be zero.
func flagsMatchHistory(t *testing.T) int {
	t.Helper()
	return count(t, `
SELECT count(*)
FROM dwh.dim_customer c
JOIN LATERAL (
    SELECT bool_or(channel = 'email' AND consent_status = 'opted_in') FILTER (WHERE channel = 'email') AS email_in,
           bool_or(channel = 'sms'   AND consent_status = 'opted_in') FILTER (WHERE channel = 'sms')   AS sms_in
    FROM (SELECT DISTINCT ON (channel) channel, consent_status
          FROM dwh.fact_consent f
          WHERE f.customer_id = c.customer_id
          ORDER BY channel, changed_at DESC, consent_id DESC) last
) l ON true
WHERE c.email_opt_in IS DISTINCT FROM coalesce(l.email_in, false)
   OR c.sms_opt_in   IS DISTINCT FROM coalesce(l.sms_in, false)`)
}

func requireInvariant(t *testing.T) {
	t.Helper()
	if n := flagsMatchHistory(t); n != 0 {
		t.Fatalf("%d customers have opt-in flags that differ from fact_consent", n)
	}
}

func allCustomerIDs(t *testing.T) []string {
	t.Helper()
	rows, err := pool.Query(t.Context(), `SELECT customer_id FROM dwh.dim_customer ORDER BY customer_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return ids
}

func TestSeedStartsConsistent(t *testing.T) { requireInvariant(t) }

func TestStatusKnownAndUnknownIDs(t *testing.T) {
	h := server(t)
	smsEligible := pick(t, "c.account_status = 'active' AND c.sms_opt_in AND c.phone_e164 IS NOT NULL")
	smsOptedOut := pick(t, "NOT c.sms_opt_in")
	// opted in on SMS but not reachable: an inactive or deleted account, or no number
	optedInUnreachable := pick(t, "c.sms_opt_in AND (c.account_status <> 'active' OR c.phone_e164 IS NULL)")
	ids := []string{"CUS-999999", smsEligible, "nobody", smsOptedOut, optedInUnreachable, smsEligible}

	got := consentStatus(t, h, ids, "sms")
	want := []status{
		{"CUS-999999", "sms", false, false},
		{smsEligible, "sms", true, true},
		{"nobody", "sms", false, false},
		{smsOptedOut, "sms", false, false},
		{optedInUnreachable, "sms", true, false},
		{smsEligible, "sms", true, true},
	}
	if !slices.Equal(got, want) {
		t.Errorf("sms status =\n%+v\nwant\n%+v", got, want)
	}

	// email: a bounced address is opted in but not eligible
	bounced := pick(t, "c.account_status = 'active' AND c.email_opt_in AND c.email_bounced")
	got = consentStatus(t, h, []string{bounced}, "email")
	if want := []status{{bounced, "email", true, false}}; !slices.Equal(got, want) {
		t.Errorf("bounced email status = %+v, want %+v", got, want)
	}

	if got := consentStatus(t, h, []string{}, "sms"); len(got) != 0 {
		t.Errorf("empty id list returned %+v", got)
	}
}

func TestStatusValidationThroughTheHandler(t *testing.T) {
	h := server(t)
	ids := make([]string, consent.MaxIDs+1)
	for i := range ids {
		ids[i] = fmt.Sprintf("CUS-%06d", i+1)
	}
	wantCode(t, post(t, h, notificationToken, statusQuery, map[string]any{"ids": ids, "ch": "sms"}), graph.CodeBadUserInput)
	wantCode(t, post(t, h, notificationToken, statusQuery, map[string]any{"ids": ids[:1], "ch": "push"}), graph.CodeBadUserInput)
	// exactly the limit is accepted
	if got := consentStatus(t, h, ids[:consent.MaxIDs], "email"); len(got) != consent.MaxIDs {
		t.Errorf("rows = %d, want %d", len(got), consent.MaxIDs)
	}
}

// TestStatusEligibilityAgreesWithAudienceRecipients: for every customer, on both
// channels, consentStatus.eligible is true exactly when audienceRecipients lists
// the customer as eligible on that channel (D-062 rule, one source of truth).
func TestStatusEligibilityAgreesWithAudienceRecipients(t *testing.T) {
	h := server(t)
	const recipients = `query($after: String) {
  audienceRecipients(filter: {}, first: 1000, after: $after) { nodes { customerId emailEligible smsEligible } pageInfo { hasNextPage endCursor } }
}`
	emailOK, smsOK := map[string]bool{}, map[string]bool{}
	var after any
	for {
		resp := post(t, h, proposalToken, recipients, map[string]any{"after": after})
		if len(resp.Errors) > 0 {
			t.Fatalf("errors: %+v", resp.Errors)
		}
		var d struct {
			AudienceRecipients struct {
				Nodes []struct {
					CustomerID    string `json:"customerId"`
					EmailEligible bool   `json:"emailEligible"`
					SmsEligible   bool   `json:"smsEligible"`
				} `json:"nodes"`
				PageInfo struct {
					HasNextPage bool    `json:"hasNextPage"`
					EndCursor   *string `json:"endCursor"`
				} `json:"pageInfo"`
			} `json:"audienceRecipients"`
		}
		if err := json.Unmarshal(resp.Data, &d); err != nil {
			t.Fatal(err)
		}
		for _, n := range d.AudienceRecipients.Nodes {
			emailOK[n.CustomerID] = n.EmailEligible
			smsOK[n.CustomerID] = n.SmsEligible
		}
		if !d.AudienceRecipients.PageInfo.HasNextPage {
			break
		}
		after = *d.AudienceRecipients.PageInfo.EndCursor
	}

	ids := allCustomerIDs(t)
	for _, tc := range []struct {
		channel string
		want    map[string]bool
	}{{"email", emailOK}, {"sms", smsOK}} {
		got := consentStatus(t, h, ids, tc.channel)
		if len(got) != len(ids) {
			t.Fatalf("%s: rows = %d, want %d", tc.channel, len(got), len(ids))
		}
		eligible := 0
		for i, s := range got {
			if s.CustomerID != ids[i] {
				t.Fatalf("%s: row %d is %s, want %s (request order)", tc.channel, i, s.CustomerID, ids[i])
			}
			if s.Eligible != tc.want[s.CustomerID] {
				t.Errorf("%s: %s eligible = %v, audienceRecipients says %v", tc.channel, s.CustomerID, s.Eligible, tc.want[s.CustomerID])
			}
			if s.Eligible {
				eligible++
			}
		}
		t.Logf("%s: %d of %d customers eligible, same set as audienceRecipients", tc.channel, eligible, len(ids))
		if eligible == 0 {
			t.Errorf("%s: nobody eligible, the comparison proves nothing", tc.channel)
		}
	}
}

type coverage struct {
	Both        int `json:"both"`
	EmailOnly   int `json:"emailOnly"`
	SmsOnly     int `json:"smsOnly"`
	Unreachable int `json:"unreachable"`
}

func cityCoverage(t *testing.T, h http.Handler, city string) (int, coverage) {
	t.Helper()
	resp := post(t, h, proposalToken,
		`query($f: CustomerFilter!) { audiencePreview(filter: $f) { count coverage { both emailOnly smsOnly unreachable } } }`,
		map[string]any{"f": map[string]any{"city": map[string]any{"eq": city}}})
	if len(resp.Errors) > 0 {
		t.Fatalf("errors: %+v", resp.Errors)
	}
	var d struct {
		AudiencePreview struct {
			Count    int      `json:"count"`
			Coverage coverage `json:"coverage"`
		} `json:"audiencePreview"`
	}
	if err := json.Unmarshal(resp.Data, &d); err != nil {
		t.Fatal(err)
	}
	return d.AudiencePreview.Count, d.AudiencePreview.Coverage
}

func customerCity(t *testing.T, id string) string {
	t.Helper()
	var city string
	if err := pool.QueryRow(t.Context(), `SELECT city FROM dwh.dim_customer WHERE customer_id = $1`, id).Scan(&city); err != nil {
		t.Fatal(err)
	}
	return city
}

type consentRow struct {
	ID, Customer, Channel, Status, Source string
	ChangedAt                             time.Time
}

func latestRow(t *testing.T, id, channel string) consentRow {
	t.Helper()
	var r consentRow
	err := pool.QueryRow(t.Context(), `
SELECT consent_id, customer_id, channel, consent_status, source, changed_at FROM dwh.fact_consent
WHERE customer_id = $1 AND channel = $2 ORDER BY changed_at DESC, consent_id DESC LIMIT 1`, id, channel).
		Scan(&r.ID, &r.Customer, &r.Channel, &r.Status, &r.Source, &r.ChangedAt)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func consentUpdatedAt(t *testing.T, id string) time.Time {
	t.Helper()
	var ts time.Time
	if err := pool.QueryRow(t.Context(), `SELECT consent_updated_at FROM dwh.dim_customer WHERE customer_id = $1`, id).Scan(&ts); err != nil {
		t.Fatal(err)
	}
	return ts
}

// TestOptOutAndBackIn: an SMS opt-out writes the next CNS row, flips the flag,
// advances consent_updated_at, keeps the seed invariant and removes the customer
// from the SMS audience; opting back in restores all of it.
func TestOptOutAndBackIn(t *testing.T) {
	h := server(t)
	// reachable by email and SMS, so an SMS opt-out moves the customer from `both` to `emailOnly`
	both := pick(t, "c.account_status = 'active' AND c.sms_opt_in AND c.phone_e164 IS NOT NULL AND c.email_opt_in AND c.email IS NOT NULL AND NOT c.email_bounced")
	city := customerCity(t, both)
	countBefore, covBefore := cityCoverage(t, h, city)
	idBefore := maxConsentNumber(t)
	updatedBefore := consentUpdatedAt(t, both)
	rowsBefore := count(t, `SELECT count(*) FROM dwh.fact_consent`)
	requireInvariant(t)

	got := updateConsent(t, h, both, "sms", false)
	if want := (status{both, "sms", false, false}); got != want {
		t.Fatalf("updateConsent = %+v, want %+v", got, want)
	}

	wantID := consent.ConsentID(int64(idBefore + 1))
	if wantID != "CNS-002220" {
		t.Logf("first new consent id is %s (seed max was %d)", wantID, idBefore)
	}
	row := latestRow(t, both, "sms")
	if row.ID != wantID || row.Status != "opted_out" || row.Source != "admin_tool" || row.Customer != both {
		t.Errorf("new fact_consent row = %+v, want id %s opted_out admin_tool", row, wantID)
	}
	if n := count(t, `SELECT count(*) FROM dwh.fact_consent`); n != rowsBefore+1 {
		t.Errorf("fact_consent rows = %d, want %d", n, rowsBefore+1)
	}
	if !consentUpdatedAt(t, both).After(updatedBefore) {
		t.Errorf("consent_updated_at did not advance past %s", updatedBefore)
	}
	if count(t, `SELECT count(*) FROM dwh.dim_customer WHERE customer_id = $1 AND NOT sms_opt_in AND email_opt_in`, both) != 1 {
		t.Error("sms flag not false, or the email flag changed")
	}
	requireInvariant(t)

	// the other channel is untouched and the status endpoint agrees
	if s := consentStatus(t, h, []string{both}, "sms")[0]; s.OptedIn || s.Eligible {
		t.Errorf("sms status after opt-out = %+v", s)
	}
	if s := consentStatus(t, h, []string{both}, "email")[0]; !s.OptedIn || !s.Eligible {
		t.Errorf("email status after sms opt-out = %+v", s)
	}

	// the audience loses the SMS reach of exactly this customer
	countAfter, covAfter := cityCoverage(t, h, city)
	wantCov := covBefore
	wantCov.Both--
	wantCov.EmailOnly++
	if countAfter != countBefore || covAfter != wantCov {
		t.Errorf("audience for %s: count %d coverage %+v, want count %d coverage %+v", city, countAfter, covAfter, countBefore, wantCov)
	}

	// back in: next id again, flag restored, audience restored
	got = updateConsent(t, h, both, "sms", true)
	if want := (status{both, "sms", true, true}); got != want {
		t.Fatalf("opt back in = %+v, want %+v", got, want)
	}
	if row := latestRow(t, both, "sms"); row.ID != consent.ConsentID(int64(idBefore+2)) || row.Status != "opted_in" {
		t.Errorf("second row = %+v, want %s opted_in", row, consent.ConsentID(int64(idBefore+2)))
	}
	requireInvariant(t)
	if c, cov := cityCoverage(t, h, city); c != countBefore || cov != covBefore {
		t.Errorf("audience after opting back in: count %d coverage %+v, want %d %+v", c, cov, countBefore, covBefore)
	}
}

// TestSMSOnlyCustomerLeavesTheAudience: an SMS-only reachable customer opting
// out drops out of the audience count and becomes unreachable.
func TestSMSOnlyCustomerLeavesTheAudience(t *testing.T) {
	h := server(t)
	id := pick(t, "c.account_status = 'active' AND c.sms_opt_in AND c.phone_e164 IS NOT NULL AND NOT (c.email IS NOT NULL AND c.email_opt_in AND NOT c.email_bounced)")
	city := customerCity(t, id)
	countBefore, covBefore := cityCoverage(t, h, city)

	updateConsent(t, h, id, "sms", false)
	countAfter, covAfter := cityCoverage(t, h, city)
	want := covBefore
	want.SmsOnly--
	want.Unreachable++
	if countAfter != countBefore-1 || covAfter != want {
		t.Errorf("audience for %s: count %d coverage %+v, want count %d coverage %+v", city, countAfter, covAfter, countBefore-1, want)
	}
	requireInvariant(t)
}

func TestEmailOptOut(t *testing.T) {
	h := server(t)
	id := pick(t, "c.account_status = 'active' AND c.email_opt_in AND c.email IS NOT NULL AND NOT c.email_bounced")
	if got := updateConsent(t, h, id, "email", false); got != (status{id, "email", false, false}) {
		t.Errorf("updateConsent = %+v", got)
	}
	if row := latestRow(t, id, "email"); row.Status != "opted_out" || row.Channel != "email" {
		t.Errorf("row = %+v", row)
	}
	requireInvariant(t)
}

// TestRepeatedUpdateWritesNothing: asking for the state the customer already
// has returns the status and leaves fact_consent and consent_updated_at alone.
func TestRepeatedUpdateWritesNothing(t *testing.T) {
	h := server(t)
	id := pick(t, "c.account_status = 'active' AND c.sms_opt_in AND c.phone_e164 IS NOT NULL")
	updateConsent(t, h, id, "sms", false)
	rows, maxID, updated := count(t, `SELECT count(*) FROM dwh.fact_consent`), maxConsentNumber(t), consentUpdatedAt(t, id)

	for i := 0; i < 3; i++ {
		if got := updateConsent(t, h, id, "sms", false); got != (status{id, "sms", false, false}) {
			t.Fatalf("repeat %d = %+v", i, got)
		}
	}
	if n := count(t, `SELECT count(*) FROM dwh.fact_consent`); n != rows {
		t.Errorf("fact_consent rows = %d, want %d (no write)", n, rows)
	}
	if n := maxConsentNumber(t); n != maxID {
		t.Errorf("max consent number = %d, want %d", n, maxID)
	}
	if got := consentUpdatedAt(t, id); !got.Equal(updated) {
		t.Errorf("consent_updated_at changed from %s to %s", updated, got)
	}

	// a customer that is already opted in: opting in again is also a no-op
	opted := pick(t, "c.account_status = 'active' AND c.email_opt_in")
	before := count(t, `SELECT count(*) FROM dwh.fact_consent`)
	updateConsent(t, h, opted, "email", true)
	if n := count(t, `SELECT count(*) FROM dwh.fact_consent`); n != before {
		t.Errorf("opt-in of an opted-in customer wrote %d rows", n-before)
	}
	requireInvariant(t)
}

func TestUpdateErrors(t *testing.T) {
	h := server(t)
	active := pick(t, "c.account_status = 'active'")
	deleted := pick(t, "c.account_status = 'deleted'")
	rowsBefore := count(t, `SELECT count(*) FROM dwh.fact_consent`)

	tests := []struct {
		name string
		vars map[string]any
		code string
	}{
		{"unknown customer", updateVars("CUS-999999", "sms", false, "admin_tool"), graph.CodeNotFound},
		{"deleted customer", updateVars(deleted, "sms", true, "admin_tool"), graph.CodeBadUserInput},
		{"deleted customer opt-out", updateVars(deleted, "email", false, "admin_tool"), graph.CodeBadUserInput},
		{"unknown channel", updateVars(active, "push", false, "admin_tool"), graph.CodeBadUserInput},
		{"free text source", updateVars(active, "sms", false, "because"), graph.CodeBadUserInput},
		{"seed only source", updateVars(active, "sms", false, "signup_form"), graph.CodeBadUserInput},
		{"empty source", updateVars(active, "sms", false, ""), graph.CodeBadUserInput},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantCode(t, post(t, h, adminToken, updateMutation, tt.vars), tt.code)
		})
	}
	if n := count(t, `SELECT count(*) FROM dwh.fact_consent`); n != rowsBefore {
		t.Errorf("rejected updates wrote %d rows", n-rowsBefore)
	}
	requireInvariant(t)

	// every allowed source is accepted and recorded
	for _, src := range consent.Sources {
		id := pick(t, "c.account_status = 'active' AND c.sms_opt_in AND c.phone_e164 IS NOT NULL AND NOT EXISTS (SELECT 1 FROM dwh.fact_consent f WHERE f.customer_id = c.customer_id AND f.source = $1)", src)
		resp := post(t, h, adminToken, updateMutation, updateVars(id, "sms", false, src))
		if len(resp.Errors) > 0 {
			t.Fatalf("source %s: %+v", src, resp.Errors)
		}
		if row := latestRow(t, id, "sms"); row.Source != src {
			t.Errorf("source %s recorded as %q", src, row.Source)
		}
	}
	requireInvariant(t)
}

// TestConcurrentUpdatesGetDistinctConsecutiveIDs: updates of different
// customers run in parallel and still allocate max+1, max+2, ... without a
// duplicate key.
func TestConcurrentUpdatesGetDistinctConsecutiveIDs(t *testing.T) {
	h := server(t)
	const n = 8
	rows, err := pool.Query(t.Context(), `
SELECT customer_id FROM dwh.dim_customer
WHERE account_status = 'active' AND email_opt_in AND email IS NOT NULL AND NOT email_bounced
ORDER BY customer_id DESC LIMIT $1`, n)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) != n {
		t.Fatalf("found %d customers, want %d", len(ids), n)
	}
	maxBefore := maxConsentNumber(t)

	var wg sync.WaitGroup
	start := make(chan struct{})
	for _, id := range ids {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			resp := post(t, h, adminToken, updateMutation, updateVars(id, "email", false, "admin_tool"))
			if len(resp.Errors) > 0 {
				t.Errorf("%s: %+v", id, resp.Errors)
			}
		}()
	}
	close(start)
	wg.Wait()

	got := map[string]bool{}
	for _, id := range ids {
		got[latestRow(t, id, "email").ID] = true
	}
	for i := 1; i <= n; i++ {
		if want := consent.ConsentID(int64(maxBefore + i)); !got[want] {
			t.Errorf("id %s missing; got %v", want, got)
		}
	}
	if len(got) != n {
		t.Errorf("distinct ids = %d, want %d", len(got), n)
	}
	requireInvariant(t)
}

// TestConcurrentUpdatesOfOneCustomerWriteOneRow: the row lock makes the second
// request see the first one's result and skip the write.
func TestConcurrentUpdatesOfOneCustomerWriteOneRow(t *testing.T) {
	h := server(t)
	id := pick(t, "c.account_status = 'active' AND c.email_opt_in AND c.email IS NOT NULL AND NOT c.email_bounced")
	before := count(t, `SELECT count(*) FROM dwh.fact_consent`)
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := post(t, h, adminToken, updateMutation, updateVars(id, "email", false, "admin_tool"))
			if len(resp.Errors) > 0 {
				t.Errorf("errors: %+v", resp.Errors)
			}
		}()
	}
	wg.Wait()
	if n := count(t, `SELECT count(*) FROM dwh.fact_consent`); n != before+1 {
		t.Errorf("rows written = %d, want 1", n-before)
	}
	requireInvariant(t)
}
