package graph_test

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/furknozg/coldbru/internal/customer/graph"
)

const recipientsQuery = `query($f: CustomerFilter!, $first: Int!, $after: String) {
  audienceRecipients(filter: $f, first: $first, after: $after) { nodes { customerId } pageInfo { hasNextPage endCursor } }
}`

func TestUnknownValueError(t *testing.T) {
	t.Parallel()
	h, db := newServerDB(t, errDatabaseDown)

	t.Run("city", func(t *testing.T) {
		t.Parallel()
		f := map[string]any{"city": map[string]any{"eq": "Istnbul"}}
		status, resp := post(t, h, previewQuery, map[string]any{"f": f})
		if status != http.StatusOK {
			t.Errorf("status = %d, want 200", status)
		}
		wantCode(t, resp, graph.CodeUnknownValue)
		ext := resp.Errors[0].Extensions
		if ext["field"] != "city" || ext["value"] != "Istnbul" {
			t.Errorf("extensions = %v, want field city, value Istnbul", ext)
		}
		if got := ext["suggestions"]; !reflect.DeepEqual(got, []any{"İstanbul"}) {
			t.Errorf("suggestions = %#v, want [İstanbul]", got)
		}
		if got := ext["filterPath"]; !reflect.DeepEqual(got, []any{"filter", "city", "eq"}) {
			t.Errorf("filterPath = %#v, want [filter city eq]", got)
		}
		if got := resp.Errors[0].Path; !reflect.DeepEqual(got, []any{"audiencePreview"}) {
			t.Errorf("path = %v, want [audiencePreview]", got)
		}
	})
	t.Run("loyalty tier in a nested relation filter path", func(t *testing.T) {
		t.Parallel()
		f := map[string]any{"and": []any{
			map[string]any{"loyaltyTier": map[string]any{"in": []any{"Gold", "gold"}}},
		}}
		_, resp := post(t, h, previewQuery, map[string]any{"f": f})
		wantCode(t, resp, graph.CodeUnknownValue)
		ext := resp.Errors[0].Extensions
		if got := ext["suggestions"]; !reflect.DeepEqual(got, []any{"Gold"}) {
			t.Errorf("suggestions = %#v, want [Gold]", got)
		}
		if got := ext["filterPath"]; !reflect.DeepEqual(got, []any{"filter", "and", float64(0), "loyaltyTier", "in", float64(1)}) {
			t.Errorf("filterPath = %#v", got)
		}
	})
	if db.calls.Load() != 0 {
		t.Error("the database was reached for an invalid filter")
	}
}

func TestFilterInputErrors(t *testing.T) {
	t.Parallel()
	ago := func(v string) map[string]any {
		return map[string]any{"signupDate": map[string]any{"gte": map[string]any{"ago": v}}}
	}
	many := make([]any, 101)
	for i := range many {
		many[i] = "x"
	}
	tests := []struct {
		name     string
		filter   map[string]any
		wantCode string
	}{
		{"ago 30x", ago("30x"), graph.CodeBadUserInput},
		{"ago 0d", ago("0d"), graph.CodeBadUserInput},
		{"empty in", map[string]any{"city": map[string]any{"in": []any{}}}, graph.CodeBadUserInput},
		{"empty field filter", map[string]any{"city": map[string]any{}}, graph.CodeBadUserInput},
		{"101 values in in", map[string]any{"timezone": map[string]any{"in": many}}, graph.CodeComplexityLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h, db := newServerDB(t, errDatabaseDown)
			_, resp := post(t, h, previewQuery, map[string]any{"f": tt.filter})
			wantCode(t, resp, tt.wantCode)
			if db.calls.Load() != 0 {
				t.Error("the database was reached")
			}
		})
	}
}

func TestRecipientsArguments(t *testing.T) {
	t.Parallel()
	f := map[string]any{"city": map[string]any{"eq": "Ankara"}}
	tests := []struct {
		name  string
		first int
		after any
	}{
		{"first zero", 0, nil},
		{"first too large", 1001, nil},
		{"cursor garbage", 10, "***"},
		{"cursor empty", 10, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h, db := newServerDB(t, errDatabaseDown)
			_, resp := post(t, h, recipientsQuery, map[string]any{"f": f, "first": tt.first, "after": tt.after})
			wantCode(t, resp, graph.CodeBadUserInput)
			if got := resp.Errors[0].Path; !reflect.DeepEqual(got, []any{"audienceRecipients"}) {
				t.Errorf("path = %v, want [audienceRecipients]", got)
			}
			if db.calls.Load() != 0 {
				t.Error("the database was reached")
			}
		})
	}

	t.Run("valid arguments reach the database", func(t *testing.T) {
		t.Parallel()
		h, db := newServerDB(t, errDatabaseDown)
		_, resp := post(t, h, recipientsQuery, map[string]any{"f": f, "first": 1000, "after": "Q1VTLTAwMDAwMQ"})
		wantReachedDatabase(t, resp)
		if db.calls.Load() != 1 {
			t.Errorf("database calls = %d, want 1", db.calls.Load())
		}
	})
}

func TestStatementTimeoutHasItsOwnCode(t *testing.T) {
	t.Parallel()
	h, _ := newServerDB(t, &pgconn.PgError{Code: "57014", Message: "canceling statement due to statement timeout"})
	_, resp := post(t, h, previewQuery, map[string]any{"f": map[string]any{}})
	wantCode(t, resp, graph.CodeQueryTimeout)
}
