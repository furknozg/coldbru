package graph_test

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/furknozg/coldbru/internal/customer/graph"
)

const (
	scopePreview = `query($f: CustomerFilter!) { audiencePreview(filter: $f) { count } }`
	scopeConsent = `{ consentStatus(customerIds: ["c1"], channel: "sms") { eligible } }`
	scopeUpdate  = `mutation { updateConsent(input: {customerId: "c1", channel: "email", optedIn: false, source: "admin_tool"}) { optedIn } }`
)

// TestScopesThroughTheHandler runs real requests through the middleware, the
// GraphQL handler and the @hasScope directive (D-063). A field that is allowed
// reaches its resolver: audiencePreview hits the failing database (INTERNAL),
// the consent fields too (the failing database answers INTERNAL).
func TestScopesThroughTheHandler(t *testing.T) {
	t.Parallel()
	h := newServer(t)
	vars := map[string]any{"f": sampleFilter()}
	tests := []struct {
		name     string
		token    string
		query    string
		vars     map[string]any
		wantCode string
		wantPath string
	}{
		{"proposal audiencePreview reaches the resolver", proposalToken, scopePreview, vars, graph.CodeInternal, "audiencePreview"},
		{"proposal consentStatus forbidden", proposalToken, scopeConsent, nil, graph.CodeForbidden, "consentStatus"},
		{"proposal updateConsent forbidden", proposalToken, scopeUpdate, nil, graph.CodeForbidden, "updateConsent"},
		{"notification consentStatus reaches the resolver", notificationToken, scopeConsent, nil, graph.CodeInternal, "consentStatus"},
		{"notification audiencePreview forbidden", notificationToken, scopePreview, vars, graph.CodeForbidden, "audiencePreview"},
		{"notification updateConsent forbidden", notificationToken, scopeUpdate, nil, graph.CodeForbidden, "updateConsent"},
		{"admin updateConsent reaches the resolver", adminToken, scopeUpdate, nil, graph.CodeInternal, "updateConsent"},
		{"admin consentStatus reaches the resolver", adminToken, scopeConsent, nil, graph.CodeInternal, "consentStatus"},
		{"admin audiencePreview forbidden", adminToken, scopePreview, vars, graph.CodeForbidden, "audiencePreview"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			status, resp := postAs(t, h, tt.token, tt.query, tt.vars)
			if status != http.StatusOK {
				t.Fatalf("status = %d, want 200", status)
			}
			wantCode(t, resp, tt.wantCode)
			if got := resp.Errors[0].Path; !reflect.DeepEqual(got, []any{tt.wantPath}) {
				t.Errorf("path = %v, want [%s]", got, tt.wantPath)
			}
			if tt.wantCode == graph.CodeForbidden && resp.Errors[0].Message != "forbidden" {
				t.Errorf("message = %q, want forbidden", resp.Errors[0].Message)
			}
		})
	}
}

func TestForbiddenFieldDoesNotRunItsResolver(t *testing.T) {
	t.Parallel()
	h, db := newServerDB(t, errDatabaseDown)
	_, resp := postAs(t, h, notificationToken, scopePreview, map[string]any{"f": sampleFilter()})
	wantCode(t, resp, graph.CodeForbidden)
	if n := db.calls.Load(); n != 0 {
		t.Errorf("database calls = %d, want 0", n)
	}
}

// TestPartialDataWithOneForbiddenField asks a Notification token for an
// allowed and a forbidden field in one operation: the allowed one still runs.
func TestPartialDataWithOneForbiddenField(t *testing.T) {
	t.Parallel()
	query := `query($f: CustomerFilter!) {
  consentStatus(customerIds: ["c1"], channel: "sms") { eligible }
  audiencePreview(filter: $f) { count }
}`
	_, resp := postAs(t, newServer(t), notificationToken, query, map[string]any{"f": sampleFilter()})
	if len(resp.Errors) != 2 {
		t.Fatalf("errors = %+v, want 2 (INTERNAL and FORBIDDEN)", resp.Errors)
	}
	codes := map[any]any{}
	for _, e := range resp.Errors {
		codes[e.Extensions["code"]] = e.Path[0]
	}
	want := map[any]any{graph.CodeInternal: "consentStatus", graph.CodeForbidden: "audiencePreview"}
	if !reflect.DeepEqual(codes, want) {
		t.Errorf("code -> path = %v, want %v", codes, want)
	}
}

// TestAllowedAndForbiddenFieldTogether: an unguarded field next to a forbidden
// one yields exactly one error, FORBIDDEN, at the forbidden field's path. The
// guarded root fields are non-null in the schema, so GraphQL propagates the
// null to the whole `data` (not partial data); making them nullable is a
// schema decision (D-063 follow-up).
func TestAllowedAndForbiddenFieldTogether(t *testing.T) {
	t.Parallel()
	query := `{ __typename consentStatus(customerIds: ["c1"], channel: "sms") { eligible } }`
	_, resp := postAs(t, newServer(t), proposalToken, query, nil)
	wantCode(t, resp, graph.CodeForbidden)
	if got := resp.Errors[0].Path; !reflect.DeepEqual(got, []any{"consentStatus"}) {
		t.Errorf("path = %v, want [consentStatus]", got)
	}
	t.Logf("data = %s", resp.Data)
}

func TestMissingTokenNeverReachesGraphQL(t *testing.T) {
	t.Parallel()
	h, db := newServerDB(t, errDatabaseDown)
	status, _ := postAs(t, h, "", scopePreview, map[string]any{"f": sampleFilter()})
	if status != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", status)
	}
	if n := db.calls.Load(); n != 0 {
		t.Errorf("database calls = %d, want 0", n)
	}
}

// TestHandlerWithoutMiddlewareDeniesEverything: no caller in the context means
// every guarded field is FORBIDDEN, never open.
func TestHandlerWithoutMiddlewareDeniesEverything(t *testing.T) {
	t.Parallel()
	h := newBareHandler(t)
	for _, q := range []string{scopeConsent, scopeUpdate} {
		_, resp := postAs(t, h, adminToken, q, nil)
		wantCode(t, resp, graph.CodeForbidden)
	}
}
