package apidocs_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGeneratedSpecs checks the committed spec of every REST service. A failure
// usually means `make generate` was not run after changing annotations (D-057).
func TestGeneratedSpecs(t *testing.T) {
	t.Parallel()
	for _, svc := range []string{"proposal", "notification", "analytics", "mockprovider"} {
		t.Run(svc, func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(filepath.Join("..", "..", "..", "cmd", svc, "apidocs", "swagger.json"))
			if err != nil {
				t.Fatalf("read spec: %v", err)
			}
			var spec struct {
				OpenAPI string                     `json:"openapi"`
				Paths   map[string]json.RawMessage `json:"paths"`
			}
			if err := json.Unmarshal(raw, &spec); err != nil {
				t.Fatalf("spec is not valid JSON: %v", err)
			}
			if !strings.HasPrefix(spec.OpenAPI, "3") {
				t.Errorf("openapi = %q, want a 3.x version", spec.OpenAPI)
			}
			for _, p := range []string{"/healthz", "/readyz"} {
				if _, ok := spec.Paths[p]; !ok {
					t.Errorf("paths is missing %s", p)
				}
			}
			if _, ok := spec.Paths["/metrics"]; ok {
				t.Error("paths must not document /metrics")
			}
		})
	}
}
