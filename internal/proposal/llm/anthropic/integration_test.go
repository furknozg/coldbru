//go:build integration

package anthropic_test

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/furknozg/coldbru/internal/customer/audience"
	"github.com/furknozg/coldbru/internal/customer/auth"
	"github.com/furknozg/coldbru/internal/customer/consent"
	"github.com/furknozg/coldbru/internal/customer/graph"
	"github.com/furknozg/coldbru/internal/customer/pgtest"
	"github.com/furknozg/coldbru/internal/proposal/customer"
	"github.com/furknozg/coldbru/internal/proposal/llm"
	"github.com/furknozg/coldbru/internal/proposal/llm/llmtest"
)

const proposalToken = "proposal-it-token"

var (
	pgOnce sync.Once
	pgEnv  *pgtest.Env
	pgErr  error
)

// TestMain only closes the database; it is started by the first test that needs it, so the
// unit tests of this package do not pay for a container.
func TestMain(m *testing.M) {
	code := m.Run()
	if pgEnv != nil {
		pgEnv.Close()
	}
	os.Exit(code)
}

func pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pgOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		pgEnv, pgErr = pgtest.Start(ctx)
	})
	if pgErr != nil {
		t.Fatalf("start seeded postgres: %v", pgErr)
	}
	return pgEnv.Pool
}

// customerClient serves the real Customer GraphQL handler over the seeded database, behind the
// real bearer-token middleware, and returns Proposal's client for it (copied from the setup of
// internal/proposal/customer/integration_test.go).
func customerClient(t *testing.T) *customer.Client {
	t.Helper()
	p := pool(t)
	cities, err := audience.LoadCities(t.Context(), p)
	if err != nil {
		t.Fatalf("LoadCities: %v", err)
	}
	svc := audience.NewService(p, cities)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := graph.NewHandler(graph.NewResolver(svc, consent.NewService(p), logger))
	tokens, err := auth.NewTokenSet(map[string]string{
		auth.CallerProposal:     proposalToken,
		auth.CallerNotification: "notification-it-token",
		auth.CallerAdmin:        "admin-it-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(tokens.Middleware(h))
	t.Cleanup(srv.Close)
	c, err := customer.New(customer.Config{URL: srv.URL, Token: proposalToken})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// The scripted model misspells the city once, is told by the real Customer handler where and what
// to write instead, and then proposes the sample brief: the loop returns the count of D-062.
func TestSampleBriefRepairedAgainstTheRealCustomerService(t *testing.T) {
	const (
		rest      = ` && cartItems.some(c => c.addedAt >= ago("30d")) && orders.none(o => o.orderDate >= ago("30d"))`
		misspelt  = `city === "Istnbul"` + rest
		corrected = `city === "İstanbul"` + rest
	)
	srv := llmtest.NewMessagesServer(t,
		llmtest.JSON(msg(llmtest.Thinking("", "sig"), proposeBlock("toolu_1", misspelt))),
		llmtest.JSON(msg(proposeBlock("toolu_2", corrected))),
		summaryTurn(),
	)
	tools := llm.NewCustomerTools(customerClient(t))
	ctx, col := withRecorder(t)
	res, err := newClient(t, srv).ProposeSegment(ctx, brief, tools)
	if err != nil {
		t.Fatalf("ProposeSegment: %v", err)
	}
	if res.DSL != corrected || res.Preview.Count != 138 || res.ProposeAttempts != 2 || res.Turn != 2 {
		t.Errorf("result = dsl %q count %d attempts %d turn %d, want the corrected DSL with count 138 on attempt 2", res.DSL, res.Preview.Count, res.ProposeAttempts, res.Turn)
	}
	if want := (customer.Coverage{Both: 85, EmailOnly: 35, SmsOnly: 18, Unreachable: 17}); res.Preview.Coverage != want { // 3000-user seed (D-113)
		t.Errorf("coverage = %+v, want %+v", res.Preview.Coverage, want)
	}

	// The model was told where the problem is (the literal on line 1, column 10) and what to write.
	_, text, isErr := lastToolResult(t, srv.Requests()[1])
	for _, want := range []string{`"code":"UNKNOWN_VALUE"`, `"line":1`, `"column":10`, `"near":"\"Istnbul\""`, `"suggestions":["İstanbul"]`} {
		if !isErr || !strings.Contains(text, want) {
			t.Errorf("tool_result lacks %s (is_error %v):\n%s", want, isErr, text)
		}
	}
	if res.Summary != scriptedSummary {
		t.Errorf("summary = %q", res.Summary)
	}
	recs := col.Records()
	if len(recs) != 3 || recs[0].Outcome != llm.OutcomeValidationFailed || recs[1].Outcome != llm.OutcomeAccepted || recs[2].Outcome != llm.OutcomeAccepted {
		t.Errorf("records = %+v", recs)
	}
}

// An audience that really is empty in the seeded database comes back as EMPTY_AUDIENCE.
func TestEmptyAudienceAgainstTheRealCustomerService(t *testing.T) {
	tools := llm.NewCustomerTools(customerClient(t))
	_, _, errs, err := tools.Validate(t.Context(), `loyaltyTier === "Platinum" && ageYears < 10`)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if len(errs) != 1 || errs[0].Code != llm.CodeEmptyAudience {
		t.Fatalf("errs = %v, want one %s", errs, llm.CodeEmptyAudience)
	}
	p, err := tools.CountAudience(t.Context(), `loyaltyTier === "Platinum" && ageYears < 10`)
	if err != nil || p.Count != 0 {
		t.Errorf("CountAudience = %+v, %v; a zero count is an answer, not an error", p, err)
	}
}
