//go:build integration

package customer_test

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furknozg/coldbru/internal/customer/audience"
	"github.com/furknozg/coldbru/internal/customer/auth"
	"github.com/furknozg/coldbru/internal/customer/consent"
	"github.com/furknozg/coldbru/internal/customer/graph"
	"github.com/furknozg/coldbru/internal/customer/pgtest"
	"github.com/furknozg/coldbru/internal/proposal/compile"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/dsl"
)

const (
	proposalToken     = "proposal-it-token"
	notificationToken = "notification-it-token"
	adminToken        = "admin-it-token"

	// sampleBrief is the DSL of the sample brief (docs/coldbru-gqdsl.md §8.2).
	sampleBrief = `city === "İstanbul" &&
cartItems.some(c => c.addedAt >= ago("30d")) &&
orders.none(o => o.orderDate >= ago("30d"))`
)

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

// newClient serves the real Customer GraphQL handler over the seeded database, behind the real
// bearer-token middleware (as cmd/customer wires it), and returns a client with the given token.
func newClient(t *testing.T, token string) *customer.Client {
	t.Helper()
	cities, err := audience.LoadCities(t.Context(), pool)
	if err != nil {
		t.Fatalf("LoadCities: %v", err)
	}
	svc := audience.NewService(pool, cities)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := graph.NewHandler(graph.NewResolver(svc, consent.NewService(pool), logger))
	tokens, err := auth.NewTokenSet(map[string]string{
		auth.CallerProposal:     proposalToken,
		auth.CallerNotification: notificationToken,
		auth.CallerAdmin:        adminToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(tokens.Middleware(h))
	t.Cleanup(srv.Close)
	c, err := customer.New(customer.Config{URL: srv.URL, Token: token})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func compileOK(t *testing.T, src string) *compile.Filter {
	t.Helper()
	f, errs := compile.Compile(src)
	if len(errs) > 0 {
		t.Fatalf("Compile(%q): %v", src, errs)
	}
	return f
}

func TestSampleBriefPreview(t *testing.T) {
	c := newClient(t, proposalToken)
	p, err := c.Preview(t.Context(), compileOK(t, sampleBrief))
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	want := customer.Coverage{Both: 20, EmailOnly: 13, SmsOnly: 13, Unreachable: 3}
	if p.Count != 46 || p.Coverage != want {
		t.Errorf("count = %d, coverage = %+v; want 46 and %+v (D-062)", p.Count, p.Coverage, want)
	}
	var sum int
	for _, l := range p.ByLanguage {
		sum += l.Count
	}
	if sum != p.Count {
		t.Errorf("byLanguage counts add up to %d, want %d", sum, p.Count)
	}
}

func TestSampleBriefRecipientsOverAllPages(t *testing.T) {
	c := newClient(t, proposalToken)
	f := compileOK(t, sampleBrief)

	all, err := c.AllRecipients(t.Context(), f)
	if err != nil {
		t.Fatalf("AllRecipients: %v", err)
	}
	if len(all) != 46 {
		t.Fatalf("recipients = %d, want 46", len(all))
	}
	ids := make(map[string]bool, len(all))
	for _, r := range all {
		if ids[r.CustomerID] {
			t.Errorf("duplicate recipient %s", r.CustomerID)
		}
		ids[r.CustomerID] = true
		if r.EmailEligible != (r.Email != "") || r.SMSEligible != (r.Phone != "") {
			t.Errorf("%s: contact details do not follow eligibility: %+v", r.CustomerID, r)
		}
		if !r.EmailEligible && !r.SMSEligible {
			t.Errorf("%s is reachable on no channel", r.CustomerID)
		}
	}
	if !slices.IsSortedFunc(all, func(a, b customer.Recipient) int { return cmp.Compare(a.CustomerID, b.CustomerID) }) {
		t.Error("recipients are not ordered by customer id")
	}

	// Small pages walk the same audience: the cursor round-trips through the client.
	var paged []customer.Recipient
	after := ""
	for pages := 0; ; pages++ {
		if pages > 10 {
			t.Fatal("too many pages")
		}
		page, err := c.Recipients(t.Context(), f, 20, after)
		if err != nil {
			t.Fatalf("Recipients: %v", err)
		}
		paged = append(paged, page.Nodes...)
		if !page.HasNextPage {
			break
		}
		after = page.EndCursor
	}
	if !slices.Equal(paged, all) {
		t.Errorf("pages of 20 returned %d recipients, differing from the single-page walk (%d)", len(paged), len(all))
	}
}

func TestUnknownCityComesBackAtTheLiteral(t *testing.T) {
	src := `retired === false &&
city === "Istnbul"`
	c := newClient(t, proposalToken)
	// city is Customer-only: the catalog accepts any text, so the filter compiles (D-066).
	f := compileOK(t, src)
	_, err := c.Preview(t.Context(), f)
	if err == nil {
		t.Fatal("want an error for an unknown city")
	}
	var de *dsl.Error
	if !errors.As(err, &de) {
		t.Fatalf("err = %T %v, want *dsl.Error", err, err)
	}
	if de.Code != dsl.CodeUnknownValue {
		t.Errorf("code = %s", de.Code)
	}
	if de.Start.Line != 2 || de.Start.Col != 10 || de.End.Col != 19 || de.Near != `"Istnbul"` {
		t.Errorf("position = %d:%d-%d:%d near %q, want 2:10-2:19 near \"Istnbul\"", de.Start.Line, de.Start.Col, de.End.Line, de.End.Col, de.Near)
	}
	if !slices.Contains(de.Suggestions, "İstanbul") {
		t.Errorf("suggestions = %v, want İstanbul", de.Suggestions)
	}
	if got := customer.DSLErrors(err); len(got) != 1 {
		t.Errorf("DSLErrors = %v", got)
	}
}

func TestUnknownCityInListComesBackAtTheElement(t *testing.T) {
	c := newClient(t, proposalToken)
	src := `city in ["Ankara", "Izmr"]`
	_, err := c.Preview(t.Context(), compileOK(t, src))
	des := customer.DSLErrors(err)
	if len(des) != 1 || des[0].Near != `"Izmr"` || des[0].Start.Col != 20 {
		t.Fatalf("err = %v, DSLErrors = %+v", err, des)
	}
}

func TestWrongTokenIsUnauthorized(t *testing.T) {
	c := newClient(t, "not-a-token")
	_, err := c.Preview(t.Context(), compileOK(t, sampleBrief))
	if !customer.IsUnauthorized(err) {
		t.Fatalf("err = %v, want unauthorized", err)
	}
	var u *customer.UnauthorizedError
	if !errors.As(err, &u) {
		t.Errorf("err is %T, want *UnauthorizedError", err)
	}
}

func TestTokenWithoutAudienceScopeIsForbidden(t *testing.T) {
	c := newClient(t, notificationToken) // consent:read only (D-063)
	_, err := c.Preview(t.Context(), compileOK(t, sampleBrief))
	var ce *customer.Error
	if !errors.As(err, &ce) || ce.Code != customer.CodeForbidden {
		t.Fatalf("err = %v, want FORBIDDEN", err)
	}
}
