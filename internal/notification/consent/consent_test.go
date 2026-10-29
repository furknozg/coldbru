package consent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// schemaPath is Customer's schema, the single source of truth for the contract (D-044).
const schemaPath = "../../customer/graph/schema.graphqls"

func loadSchema(t *testing.T) *ast.Schema {
	t.Helper()
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	schema, err := gqlparser.LoadSchema(&ast.Source{Name: "schema.graphqls", Input: string(raw)})
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	return schema
}

func TestQueryMatchesSchema(t *testing.T) {
	t.Parallel()
	if _, errs := gqlparser.LoadQuery(loadSchema(t), consentQuery); len(errs) > 0 {
		t.Errorf("query does not match the Customer schema: %v\n%s", errs, consentQuery)
	}
}

func TestValidateQueryCatchesDrift(t *testing.T) {
	t.Parallel()
	// Guard for the guard: a field the schema lacks must fail validation.
	bad := `query($ids: [ID!]!, $channel: String!) { consentStatus(customerIds: $ids, channel: $channel) { customerId nope } }`
	if _, errs := gqlparser.LoadQuery(loadSchema(t), bad); len(errs) == 0 {
		t.Error("a query with an unknown field passed validation")
	}
}

// fakeCustomer answers consentStatus. eligible lists the ids that are eligible; every other id is
// answered false, like Customer does for an unknown or opted-out customer.
type fakeCustomer struct {
	eligible map[string]bool
	calls    atomic.Int32
	lastAuth atomic.Value
	lastReq  atomic.Value
}

func (f *fakeCustomer) handler(w http.ResponseWriter, r *http.Request) {
	f.calls.Add(1)
	f.lastAuth.Store(r.Header.Get("Authorization"))
	var req struct {
		OperationName string `json:"operationName"`
		Query         string `json:"query"`
		Variables     struct {
			IDs     []string `json:"ids"`
			Channel string   `json:"channel"`
		} `json:"variables"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	f.lastReq.Store(req)
	type row struct {
		CustomerID string `json:"customerId"`
		Channel    string `json:"channel"`
		OptedIn    bool   `json:"optedIn"`
		Eligible   bool   `json:"eligible"`
	}
	rows := make([]row, 0, len(req.Variables.IDs))
	for _, id := range req.Variables.IDs {
		rows = append(rows, row{id, req.Variables.Channel, f.eligible[id], f.eligible[id]})
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"consentStatus": rows}})
}

func newClient(t *testing.T, h http.Handler, cfg Config) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	cfg.URL, cfg.Token = srv.URL, "notification-test-token"
	c, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEligibleSendsOperationAndToken(t *testing.T) {
	t.Parallel()
	f := &fakeCustomer{eligible: map[string]bool{"CUS-1": true}}
	c := newClient(t, http.HandlerFunc(f.handler), Config{})
	got, err := c.Eligible(t.Context(), "sms", []string{"CUS-1", "CUS-2"})
	if err != nil {
		t.Fatalf("Eligible() error = %v", err)
	}
	if !got["CUS-1"] || got["CUS-2"] || len(got) != 2 {
		t.Errorf("Eligible() = %v, want CUS-1 true and CUS-2 false", got)
	}
	if auth := f.lastAuth.Load(); auth != "Bearer notification-test-token" {
		t.Errorf("Authorization = %v", auth)
	}
	req := f.lastReq.Load().(struct {
		OperationName string `json:"operationName"`
		Query         string `json:"query"`
		Variables     struct {
			IDs     []string `json:"ids"`
			Channel string   `json:"channel"`
		} `json:"variables"`
	})
	if req.OperationName != "ConsentStatus" || req.Query != consentQuery || req.Variables.Channel != "sms" || len(req.Variables.IDs) != 2 {
		t.Errorf("request = %+v", req)
	}
}

func TestEligibleSplitsLongLists(t *testing.T) {
	t.Parallel()
	f := &fakeCustomer{eligible: map[string]bool{}}
	ids := make([]string, 2500)
	for i := range ids {
		ids[i] = fmt.Sprintf("CUS-%d", i)
		f.eligible[ids[i]] = i%2 == 0
	}
	c := newClient(t, http.HandlerFunc(f.handler), Config{})
	got, err := c.Eligible(t.Context(), "email", ids)
	if err != nil || len(got) != 2500 || f.calls.Load() != 3 {
		t.Fatalf("len = %d, calls = %d, error = %v; want 2500 answers in 3 calls", len(got), f.calls.Load(), err)
	}
	if !got["CUS-0"] || got["CUS-1"] {
		t.Error("answers are mixed up between chunks")
	}
}

func TestEligibleFailuresAreUnavailable(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"503", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(503) }},
		{"401", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(401) }},
		{"graphql error", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"errors":[{"message":"no scope","extensions":{"code":"FORBIDDEN"}}],"data":null}`))
		}},
		{"graphql error with a 4xx", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(422)
			_, _ = w.Write([]byte(`{"errors":[{"message":"bad","extensions":{"code":"BAD_USER_INPUT"}}]}`))
		}},
		{"not json", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>")) }},
		{"missing id", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"data":{"consentStatus":[{"customerId":"CUS-1","channel":"sms","optedIn":true,"eligible":true}]}}`))
		}},
		{"wrong channel", func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"data":{"consentStatus":[{"customerId":"CUS-1","channel":"email","optedIn":true,"eligible":true},{"customerId":"CUS-2","channel":"email","optedIn":true,"eligible":true}]}}`))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c := newClient(t, tt.handler, Config{})
			got, err := c.Eligible(t.Context(), "sms", []string{"CUS-1", "CUS-2"})
			if !errors.Is(err, ErrUnavailable) || got != nil {
				t.Errorf("Eligible() = %v, %v; want nil and ErrUnavailable", got, err)
			}
		})
	}
}

func TestEligibleTimeout(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }))
	t.Cleanup(func() { close(release); srv.Close() })
	c, err := New(Config{URL: srv.URL, Token: "t", Timeout: 50 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Eligible(t.Context(), "sms", []string{"CUS-1"}); !errors.Is(err, ErrUnavailable) {
		t.Errorf("error = %v, want ErrUnavailable", err)
	}
}

func TestBreakerOpensAfterFiveConsecutiveFailures(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}), Config{OpenFor: time.Minute})

	for i := 1; i <= 5; i++ {
		_, err := c.Eligible(t.Context(), "sms", []string{"CUS-1"})
		if !errors.Is(err, ErrUnavailable) || errors.Is(err, ErrBreakerOpen) {
			t.Fatalf("call %d: error = %v, want a plain failure (the breaker is still closed)", i, err)
		}
		wantState := "closed"
		if i == 5 {
			wantState = "open"
		}
		if got := c.State(); got != wantState {
			t.Fatalf("after failure %d the breaker is %s, want %s", i, got, wantState)
		}
	}
	// Open: rejected without contacting Customer.
	_, err := c.Eligible(t.Context(), "sms", []string{"CUS-1"})
	if !errors.Is(err, ErrBreakerOpen) || !errors.Is(err, ErrUnavailable) {
		t.Errorf("error = %v, want ErrBreakerOpen", err)
	}
	if hits.Load() != 5 {
		t.Errorf("Customer got %d requests, want 5 (the sixth was rejected by the breaker)", hits.Load())
	}
}

func TestBreakerCountsConsecutiveFailuresOnly(t *testing.T) {
	t.Parallel()
	var n atomic.Int32
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Every fifth request succeeds: failures never reach five in a row.
		if n.Add(1)%5 == 0 {
			(&fakeCustomer{eligible: map[string]bool{"CUS-1": true}}).handler(w, r)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}), Config{})
	for range 30 {
		_, _ = c.Eligible(t.Context(), "sms", []string{"CUS-1"})
	}
	if c.State() == "open" {
		t.Error("the breaker opened although a success came every fifth call")
	}
}

func TestBreakerHalfOpenTrialCloses(t *testing.T) {
	t.Parallel()
	f := &fakeCustomer{eligible: map[string]bool{"CUS-1": true}}
	var down atomic.Bool
	down.Store(true)
	var hits atomic.Int32
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if down.Load() {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		f.handler(w, r)
	}), Config{OpenFor: 80 * time.Millisecond})

	for range 5 {
		_, _ = c.Eligible(t.Context(), "sms", []string{"CUS-1"})
	}
	if c.State() != "open" {
		t.Fatalf("state = %s, want open", c.State())
	}
	down.Store(false)
	if _, err := c.Eligible(t.Context(), "sms", []string{"CUS-1"}); !errors.Is(err, ErrBreakerOpen) {
		t.Fatalf("error = %v, want the open breaker to reject while Customer is back", err)
	}
	time.Sleep(120 * time.Millisecond)
	if c.State() != "half-open" {
		t.Fatalf("state = %s after the open period, want half-open", c.State())
	}
	got, err := c.Eligible(t.Context(), "sms", []string{"CUS-1"})
	if err != nil || !got["CUS-1"] {
		t.Fatalf("trial call = %v, %v", got, err)
	}
	if c.State() != "closed" {
		t.Errorf("state = %s after a successful trial, want closed", c.State())
	}
	if hits.Load() != 6 {
		t.Errorf("Customer got %d requests, want 5 failures + 1 trial", hits.Load())
	}
}

func TestBreakerHalfOpenFailedTrialReopens(t *testing.T) {
	t.Parallel()
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}), Config{OpenFor: 60 * time.Millisecond})
	for range 5 {
		_, _ = c.Eligible(t.Context(), "sms", []string{"CUS-1"})
	}
	time.Sleep(100 * time.Millisecond)
	if _, err := c.Eligible(t.Context(), "sms", []string{"CUS-1"}); errors.Is(err, ErrBreakerOpen) {
		t.Fatal("the trial call was rejected, want it to reach Customer")
	}
	if c.State() != "open" {
		t.Errorf("state = %s after a failed trial, want open", c.State())
	}
}

func TestBreakerHalfOpenAllowsOneTrialAtATime(t *testing.T) {
	t.Parallel()
	entered := make(chan struct{})
	release := make(chan struct{})
	var down atomic.Bool
	down.Store(true)
	var hits atomic.Int32
	c := newClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if down.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		close(entered)
		<-release
		(&fakeCustomer{eligible: map[string]bool{"CUS-1": true}}).handler(w, r)
	}), Config{OpenFor: 50 * time.Millisecond})
	for range 5 {
		_, _ = c.Eligible(t.Context(), "sms", []string{"CUS-1"})
	}
	down.Store(false)
	time.Sleep(90 * time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(1)
	var trialErr error
	go func() {
		defer wg.Done()
		_, trialErr = c.Eligible(t.Context(), "sms", []string{"CUS-1"})
	}()
	<-entered // the trial is in flight
	if _, err := c.Eligible(t.Context(), "sms", []string{"CUS-1"}); !errors.Is(err, ErrBreakerOpen) {
		t.Errorf("second call during the trial: error = %v, want it rejected by the breaker", err)
	}
	close(release)
	wg.Wait()
	if trialErr != nil {
		t.Errorf("trial error = %v", trialErr)
	}
	if hits.Load() != 6 {
		t.Errorf("Customer got %d requests, want 6 (5 failures and a single trial)", hits.Load())
	}
}

func TestCancelledCallDoesNotCountAgainstTheBreaker(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	c := newClient(t, http.HandlerFunc(func(http.ResponseWriter, *http.Request) { <-release }), Config{})
	t.Cleanup(func() { close(release) })
	for range 8 {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, _ = c.Eligible(ctx, "sms", []string{"CUS-1"})
	}
	if c.State() != "closed" {
		t.Errorf("state = %s after cancelled calls, want closed", c.State())
	}
}

func TestNewRequiresURLAndToken(t *testing.T) {
	t.Parallel()
	_, err := New(Config{})
	if err == nil || !strings.Contains(err.Error(), KeyURL) || !strings.Contains(err.Error(), KeyToken) {
		t.Errorf("New() error = %v, want one naming both keys", err)
	}
}

func TestLoadConfigDoesNotLeakTheToken(t *testing.T) {
	t.Parallel()
	v := viper.New()
	if _, err := LoadConfig(v); err == nil {
		t.Error("LoadConfig() with nothing set succeeded")
	}
	v.Set(KeyURL, "http://customer:8080/graphql")
	v.Set(KeyToken, "super-secret")
	cfg, err := LoadConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cfg.String(), "super-secret") || strings.Contains(fmt.Sprint(cfg), "super-secret") {
		t.Error("Config prints the token")
	}
}
