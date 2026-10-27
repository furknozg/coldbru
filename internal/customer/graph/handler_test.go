package graph_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/furknozg/coldbru/internal/customer/audience"
	"github.com/furknozg/coldbru/internal/customer/graph"
)

// response is the GraphQL response envelope.
type response struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message    string         `json:"message"`
		Path       []any          `json:"path"`
		Extensions map[string]any `json:"extensions"`
	} `json:"errors"`
}

// failingDB is a database that always fails to begin a transaction. A request
// that reaches it has passed GraphQL validation and the filter compiler, which
// is what the validation tests below need to show; real queries run in the
// integration tests.
type failingDB struct {
	err   error
	calls atomic.Int32
}

func (d *failingDB) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	d.calls.Add(1)
	return nil, d.err
}

var errDatabaseDown = errors.New("database down")

func newServer(t *testing.T) http.Handler {
	t.Helper()
	h, _ := newServerDB(t, errDatabaseDown)
	return h
}

// newServerDB returns the handler and the database stand-in; dbErr is what
// BeginTx fails with.
func newServerDB(t *testing.T, dbErr error) (http.Handler, *failingDB) {
	t.Helper()
	cities, err := audience.NewCities([]string{"İstanbul", "Ankara", "Eskişehir"})
	if err != nil {
		t.Fatal(err)
	}
	db := &failingDB{err: dbErr}
	svc := audience.NewService(db, cities)
	return graph.NewHandler(graph.NewResolver(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))), db
}

// post sends a GraphQL POST and decodes the response.
func post(t *testing.T, h http.Handler, query string, variables map[string]any) (int, response) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	var resp response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	t.Logf("status=%d body=%s", rec.Code, strings.TrimSpace(rec.Body.String()))
	return rec.Code, resp
}

const previewQuery = `query($f: CustomerFilter!) { audiencePreview(filter: $f) { count } }`

// wantReachedDatabase fails unless the request passed validation and the
// compiler and then hit the (failing) database: an INTERNAL error that does not
// leak the cause.
func wantReachedDatabase(t *testing.T, resp response) {
	t.Helper()
	wantCode(t, resp, graph.CodeInternal)
	if got := resp.Errors[0].Message; got != "internal error" {
		t.Errorf("message = %q, want %q (no internal details)", got, "internal error")
	}
}

// wantCode fails unless resp holds exactly one error with extensions.code code.
func wantCode(t *testing.T, resp response, code string) {
	t.Helper()
	if len(resp.Errors) != 1 {
		t.Fatalf("errors = %+v, want exactly one with code %s", resp.Errors, code)
	}
	if got := resp.Errors[0].Extensions["code"]; got != code {
		t.Fatalf("extensions.code = %v, want %s (message %q)", got, code, resp.Errors[0].Message)
	}
}

// sampleFilter is the docs/coldbru-gqdsl.md section 8.2 normalized filter.
func sampleFilter() map[string]any {
	return map[string]any{"and": []any{
		map[string]any{"city": map[string]any{"eq": "İstanbul"}},
		map[string]any{"cartItems": map[string]any{"some": map[string]any{
			"addedAt": map[string]any{"gte": map[string]any{"ago": "30d"}},
		}}},
		map[string]any{"orders": map[string]any{"none": map[string]any{
			"orderDate": map[string]any{"gte": map[string]any{"ago": "30d"}},
		}}},
	}}
}

func TestSampleFilterPassesValidation(t *testing.T) {
	t.Parallel()
	status, resp := post(t, newServer(t), previewQuery, map[string]any{"f": sampleFilter()})
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	wantReachedDatabase(t, resp)
}

func TestAllFilterOperatorsPassValidation(t *testing.T) {
	t.Parallel()
	filter := map[string]any{
		"or": []any{
			map[string]any{"not": map[string]any{"loyaltyTier": map[string]any{"in": []any{"Gold", "Platinum"}}}},
			map[string]any{
				"totalSpendGbp":      map[string]any{"gte": "1000.50", "lt": "5000"},
				"sessions90d":        map[string]any{"ne": 0},
				"hasOpenCart":        map[string]any{"eq": true},
				"signupDate":         map[string]any{"eq": map[string]any{"date": "2026-01-01"}},
				"lastCampaignSentAt": map[string]any{"lte": map[string]any{"at": "2026-10-27T09:30:00Z"}},
				"events":             map[string]any{"every": map[string]any{"valueGbp": map[string]any{"gt": "0.01"}}},
			},
		},
	}
	_, resp := post(t, newServer(t), previewQuery, map[string]any{"f": filter})
	wantReachedDatabase(t, resp)
}

func TestTimeValueRequiresExactlyOneField(t *testing.T) {
	t.Parallel()
	h := newServer(t)

	t.Run("literal with two fields is rejected by validation", func(t *testing.T) {
		t.Parallel()
		status, resp := post(t, h, `{ audiencePreview(filter: {signupDate: {gte: {ago: "1d", date: "2026-01-01"}}}) { count } }`, nil)
		if status != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, want 422", status)
		}
		wantCode(t, resp, "GRAPHQL_VALIDATION_FAILED")
		if !strings.Contains(resp.Errors[0].Message, "exactly one key") {
			t.Errorf("message = %q, want OneOf message", resp.Errors[0].Message)
		}
	})
	t.Run("variable with two fields is rejected by the OneOf directive", func(t *testing.T) {
		t.Parallel()
		f := map[string]any{"signupDate": map[string]any{"gte": map[string]any{"ago": "30d", "date": "2026-01-01"}}}
		_, resp := post(t, h, previewQuery, map[string]any{"f": f})
		wantCode(t, resp, graph.CodeBadUserInput)
		if !strings.Contains(resp.Errors[0].Message, "exactly one") {
			t.Errorf("message = %q, want OneOf message", resp.Errors[0].Message)
		}
	})
	t.Run("empty literal is rejected", func(t *testing.T) {
		t.Parallel()
		_, resp := post(t, h, `{ audiencePreview(filter: {signupDate: {gte: {}}}) { count } }`, nil)
		wantCode(t, resp, "GRAPHQL_VALIDATION_FAILED")
	})
	t.Run("empty variable is rejected", func(t *testing.T) {
		t.Parallel()
		f := map[string]any{"signupDate": map[string]any{"gte": map[string]any{}}}
		_, resp := post(t, h, previewQuery, map[string]any{"f": f})
		wantCode(t, resp, graph.CodeBadUserInput)
	})
	t.Run("null beside the value counts as unset", func(t *testing.T) {
		t.Parallel()
		f := map[string]any{"signupDate": map[string]any{"gte": map[string]any{"ago": "30d", "date": nil}}}
		_, resp := post(t, h, previewQuery, map[string]any{"f": f})
		// null counts as unset, so this is a valid single-field TimeValue.
		wantReachedDatabase(t, resp)
	})
}

// TestCatalogExcludesPersonalFields proves the filter input has no personal,
// consent or account fields (D-042, docs/coldbru-gqdsl.md section 9.2).
func TestCatalogExcludesPersonalFields(t *testing.T) {
	t.Parallel()
	h := newServer(t)
	for _, field := range []string{
		"email", "phoneE164", "firstName", "lastName", "dateOfBirth", "employerCompany",
		"addressStreet", "emailOptIn", "smsOptIn", "emailBounced", "accountStatus", "deletedAt", "consentUpdatedAt",
	} {
		t.Run(field+" literal", func(t *testing.T) {
			t.Parallel()
			query := `{ audiencePreview(filter: {` + field + `: {eq: "x"}}) { count } }`
			status, resp := post(t, h, query, nil)
			if status != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422", status)
			}
			wantCode(t, resp, "GRAPHQL_VALIDATION_FAILED")
		})
		t.Run(field+" variable", func(t *testing.T) {
			t.Parallel()
			_, resp := post(t, h, previewQuery, map[string]any{"f": map[string]any{field: map[string]any{"eq": "x"}}})
			wantCode(t, resp, "GRAPHQL_VALIDATION_FAILED")
		})
	}
	t.Run("nested relation field", func(t *testing.T) {
		t.Parallel()
		f := map[string]any{"orders": map[string]any{"some": map[string]any{"customerId": map[string]any{"eq": "c1"}}}}
		_, resp := post(t, h, previewQuery, map[string]any{"f": f})
		wantCode(t, resp, "GRAPHQL_VALIDATION_FAILED")
	})
}

func TestDecimalMustBeAString(t *testing.T) {
	t.Parallel()
	h := newServer(t)
	for name, value := range map[string]any{"json number": 12.5, "three decimals": "12.555", "text": "abc"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := map[string]any{"totalSpendGbp": map[string]any{"gt": value}}
			_, resp := post(t, h, previewQuery, map[string]any{"f": f})
			if len(resp.Errors) != 1 || strings.Contains(resp.Errors[0].Message, "not implemented") {
				t.Fatalf("errors = %+v, want one decimal error", resp.Errors)
			}
		})
	}
}

func TestConsentResolversReturnNotImplemented(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		query string
		path  string
	}{
		{"consentStatus", `{ consentStatus(customerIds: ["c1"], channel: "sms") { eligible } }`, "consentStatus"},
		{"updateConsent", `mutation { updateConsent(input: {customerId: "c1", channel: "email", optedIn: false, source: "test"}) { optedIn eligible } }`, "updateConsent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			status, resp := post(t, newServer(t), tt.query, nil)
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200", status)
			}
			wantCode(t, resp, graph.CodeNotImplemented)
			if got := resp.Errors[0].Path; len(got) != 1 || got[0] != tt.path {
				t.Errorf("path = %v, want [%s]", got, tt.path)
			}
		})
	}
}

// TestIntrospectionUnavailable documents the observed behaviour: handler.New
// without the Introspection extension answers __schema and __type with an
// "introspection disabled" error and null, HTTP 200. Proposal builds its
// catalog from the build-time schema instead (D-044).
func TestIntrospectionUnavailable(t *testing.T) {
	t.Parallel()
	h := newServer(t)
	for _, query := range []string{
		`{ __schema { types { name } } }`,
		`{ __type(name: "CustomerFilter") { name } }`,
	} {
		status, resp := post(t, h, query, nil)
		if status != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", query, status)
		}
		if len(resp.Errors) != 1 || resp.Errors[0].Message != "introspection disabled" {
			t.Errorf("%s: errors = %+v, want \"introspection disabled\"", query, resp.Errors)
		}
		if strings.Contains(string(resp.Data), "name") {
			t.Errorf("%s: data leaks schema: %s", query, resp.Data)
		}
	}
}

func TestOnlyPostIsServed(t *testing.T) {
	t.Parallel()
	h := newServer(t)
	req := httptest.NewRequest(http.MethodGet, "/graphql?query=%7B__typename%7D", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		t.Errorf("GET status = %d, want an error status; body %s", rec.Code, rec.Body.String())
	}
}
