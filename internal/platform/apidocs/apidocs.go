// Package apidocs serves a service's embedded OpenAPI spec and a Scalar
// reference page for it (D-057).
package apidocs

import (
	"fmt"
	"net/http"
	"strings"

	scalargo "github.com/bdpiprava/scalar-go"
	"github.com/go-chi/chi/v5"
)

// Paths of the routes Mount registers.
const (
	DocsPath = "/docs"
	SpecPath = "/openapi.json"
)

// ScalarCDN is the pinned Scalar bundle the docs page loads in the browser.
const ScalarCDN = "https://cdn.jsdelivr.net/npm/@scalar/api-reference@1.74.0"

// specURL is the spec location the page uses. It is relative so the page keeps
// working when a reverse proxy serves the service under a path prefix.
const specURL = "openapi.json"

// placeholderURL stands in for specURL while scalar-go renders the page:
// scalar-go accepts only absolute http(s) spec URLs.
const placeholderURL = "https://placeholder.invalid/" + specURL

// Mount registers GET /openapi.json (spec, verbatim) and GET /docs (Scalar
// page titled title) on r. It registers nothing when spec is empty.
func Mount(r chi.Router, spec []byte, title string) error {
	if len(spec) == 0 {
		return nil
	}
	page, err := renderPage(title)
	if err != nil {
		return err
	}
	r.Get(SpecPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(spec)
	})
	r.Get(DocsPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
	return nil
}

func renderPage(title string) ([]byte, error) {
	html, err := scalargo.NewV2(
		scalargo.WithCDN(ScalarCDN),
		scalargo.WithSpecURL(placeholderURL),
		scalargo.WithMetaDataOpts(scalargo.WithTitle(title)),
	)
	if err != nil {
		return nil, fmt.Errorf("render scalar page: %w", err)
	}
	if !strings.Contains(html, placeholderURL) {
		return nil, fmt.Errorf("render scalar page: spec url %q not found in page", placeholderURL)
	}
	return []byte(strings.ReplaceAll(html, placeholderURL, specURL)), nil
}
