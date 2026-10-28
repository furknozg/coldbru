package customer

import (
	"os"
	"testing"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// schemaPath is Customer's schema, the single source of truth for the contract (D-044).
const schemaPath = "../../customer/graph/schema.graphqls"

// validateQuery fails the test when query is not valid against Customer's schema, so that schema
// drift breaks the build instead of production (D-067).
func validateQuery(t *testing.T, query string) {
	t.Helper()
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	schema, err := gqlparser.LoadSchema(&ast.Source{Name: "schema.graphqls", Input: string(raw)})
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	if _, errs := gqlparser.LoadQuery(schema, query); len(errs) > 0 {
		t.Errorf("query does not match the Customer schema: %v\n%s", errs, query)
	}
}

func TestValidateQueryCatchesDrift(t *testing.T) {
	// Guard for the guard: a query with a field the schema lacks must fail validation.
	raw, _ := os.ReadFile(schemaPath)
	schema, err := gqlparser.LoadSchema(&ast.Source{Name: "schema.graphqls", Input: string(raw)})
	if err != nil {
		t.Fatal(err)
	}
	bad := `query($filter: CustomerFilter!) { audiencePreview(filter: $filter) { count nope } }`
	if _, errs := gqlparser.LoadQuery(schema, bad); len(errs) == 0 {
		t.Error("a query with an unknown field passed validation")
	}
	bad = `query($filter: CustomerFilter!) { audienceRecipients(filter: $filter) { nodes { customerId } } }`
	if _, errs := gqlparser.LoadQuery(schema, bad); len(errs) == 0 {
		t.Error("a query without the required first argument passed validation")
	}
}
