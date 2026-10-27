// Package auth identifies the callers of the Customer GraphQL endpoint and
// gives each a fixed set of scopes (D-043, D-063). Tokens come from the
// configuration; which caller holds which scope is fixed here, so a wrong
// environment cannot widen a scope.
package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/viper"

	"github.com/furknozg/coldbru/internal/platform/config"
)

// Scopes named in the @hasScope directives of the schema (D-063).
const (
	ScopeAudienceRead = "audience:read"
	ScopeConsentRead  = "consent:read"
	ScopeConsentWrite = "consent:write"
)

// Config keys holding the token of each caller (D-063).
const (
	KeyTokenProposal     = "CUSTOMER_TOKEN_PROPOSAL"
	KeyTokenNotification = "CUSTOMER_TOKEN_NOTIFICATION"
	KeyTokenAdmin        = "CUSTOMER_TOKEN_ADMIN"
)

// Caller names.
const (
	CallerProposal     = "proposal"
	CallerNotification = "notification"
	CallerAdmin        = "admin"
)

// callerDef binds a caller to its config key and fixed scopes. The order is
// the order tokens are checked and reported in.
type callerDef struct {
	name   string
	key    string
	scopes []string
}

// The admin token also reads consent so a change can be verified with consentStatus (D-063).
var callerDefs = []callerDef{
	{CallerProposal, KeyTokenProposal, []string{ScopeAudienceRead}},
	{CallerNotification, KeyTokenNotification, []string{ScopeConsentRead}},
	{CallerAdmin, KeyTokenAdmin, []string{ScopeConsentWrite, ScopeConsentRead}},
}

// Caller is an authenticated caller of the GraphQL endpoint.
type Caller struct {
	Name   string
	scopes []string
}

// HasScope reports whether the caller holds scope.
func (c Caller) HasScope(scope string) bool { return slices.Contains(c.scopes, scope) }

// Scopes returns a copy of the caller's scopes.
func (c Caller) Scopes() []string { return slices.Clone(c.scopes) }

type ctxKey struct{}

// WithCaller returns ctx carrying c.
func WithCaller(ctx context.Context, c Caller) context.Context {
	return context.WithValue(ctx, ctxKey{}, c)
}

// FromContext returns the caller the middleware stored in ctx.
func FromContext(ctx context.Context) (Caller, bool) {
	c, ok := ctx.Value(ctxKey{}).(Caller)
	return c, ok
}

type entry struct {
	caller Caller
	digest [sha256.Size]byte
}

// TokenSet maps bearer tokens to callers. Only SHA-256 digests are kept.
type TokenSet struct {
	entries []entry
}

// Keys returns every config key a TokenSet needs.
func Keys() []string {
	keys := make([]string, 0, len(callerDefs))
	for _, d := range callerDefs {
		keys = append(keys, d.key)
	}
	return keys
}

// LoadTokenSet reads the three token keys from v. It fails when one is missing
// or empty, or when two callers share a token (D-063).
func LoadTokenSet(v *viper.Viper) (*TokenSet, error) {
	if err := config.Require(v, Keys()...); err != nil {
		return nil, err
	}
	tokens := make(map[string]string, len(callerDefs))
	for _, d := range callerDefs {
		tokens[d.name] = v.GetString(d.key)
	}
	return NewTokenSet(tokens)
}

// NewTokenSet builds a TokenSet from tokens keyed by caller name (CallerProposal,
// CallerNotification, CallerAdmin). Every caller needs a non-empty token and no
// two tokens may be equal. Error messages name config keys, never token values.
func NewTokenSet(tokens map[string]string) (*TokenSet, error) {
	var missing []string
	seen := make(map[string]string, len(callerDefs)) // token -> config key
	var dupes []string
	set := &TokenSet{}
	for _, d := range callerDefs {
		tok := strings.TrimSpace(tokens[d.name])
		if tok == "" {
			missing = append(missing, d.key)
			continue
		}
		if first, ok := seen[tok]; ok {
			dupes = append(dupes, first+" and "+d.key)
			continue
		}
		seen[tok] = d.key
		set.entries = append(set.entries, entry{
			caller: Caller{Name: d.name, scopes: slices.Clone(d.scopes)},
			digest: sha256.Sum256([]byte(tok)),
		})
	}
	var errs []error
	if len(missing) > 0 {
		sort.Strings(missing)
		errs = append(errs, fmt.Errorf("missing required config: %s", strings.Join(missing, ", ")))
	}
	if len(dupes) > 0 {
		errs = append(errs, fmt.Errorf("caller tokens must differ: %s share a token", strings.Join(dupes, "; ")))
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return set, nil
}

// lookup returns the caller whose token equals token. It compares against every
// entry in constant time (on SHA-256 digests, so token length does not leak)
// and does not stop at the first match.
func (s *TokenSet) lookup(token string) (Caller, bool) {
	digest := sha256.Sum256([]byte(token))
	var found Caller
	matched := 0
	for _, e := range s.entries {
		if subtle.ConstantTimeCompare(digest[:], e.digest[:]) == 1 {
			found = e.caller
			matched = 1
		}
	}
	return found, matched == 1
}

// bearerToken extracts the token of an "Authorization: Bearer <token>" header.
func bearerToken(r *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}

// Middleware authenticates every request with a bearer token. A missing,
// malformed or unknown token gets 401 and the request never reaches next; on
// success the caller is available through FromContext.
func (s *TokenSet) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := bearerToken(r)
		if ok {
			var caller Caller
			if caller, ok = s.lookup(token); ok {
				next.ServeHTTP(w, r.WithContext(WithCaller(r.Context(), caller)))
				return
			}
		}
		w.Header().Set("WWW-Authenticate", "Bearer")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	})
}
