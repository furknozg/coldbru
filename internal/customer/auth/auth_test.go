package auth_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/furknozg/coldbru/internal/customer/auth"
)

func validTokens() map[string]string {
	return map[string]string{
		auth.CallerProposal:     "tok-proposal",
		auth.CallerNotification: "tok-notification",
		auth.CallerAdmin:        "tok-admin",
	}
}

func mustSet(t *testing.T) *auth.TokenSet {
	t.Helper()
	set, err := auth.NewTokenSet(validTokens())
	if err != nil {
		t.Fatal(err)
	}
	return set
}

func TestMiddlewareRejectsBadCredentials(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		header string
		set    bool
	}{
		{"no header", "", false},
		{"wrong scheme", "Basic tok-proposal", true},
		{"scheme only", "Bearer", true},
		{"empty bearer", "Bearer ", true},
		{"unknown token", "Bearer nope", true},
		{"token without scheme", "tok-proposal", true},
		{"prefix of a token", "Bearer tok-propos", true},
		{"token with suffix", "Bearer tok-proposal-x", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			reached := false
			h := mustSet(t).Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
			req := httptest.NewRequest(http.MethodPost, "/graphql", nil)
			if tt.set {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if reached {
				t.Fatal("request reached the wrapped handler")
			}
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
			if got := rec.Header().Get("WWW-Authenticate"); got != "Bearer" {
				t.Errorf("WWW-Authenticate = %q, want Bearer", got)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			if got := rec.Body.String(); got != `{"error":"unauthorized"}` {
				t.Errorf("body = %q, want {\"error\":\"unauthorized\"}", got)
			}
		})
	}
}

func TestMiddlewarePutsCallerInContext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		token      string
		scheme     string
		wantName   string
		wantScopes []string
	}{
		{"tok-proposal", "Bearer", auth.CallerProposal, []string{auth.ScopeAudienceRead}},
		{"tok-notification", "Bearer", auth.CallerNotification, []string{auth.ScopeConsentRead}},
		{"tok-admin", "Bearer", auth.CallerAdmin, []string{auth.ScopeConsentWrite, auth.ScopeConsentRead}},
		{"tok-admin", "bearer", auth.CallerAdmin, []string{auth.ScopeConsentWrite, auth.ScopeConsentRead}},
	}
	for _, tt := range tests {
		t.Run(tt.wantName+"/"+tt.scheme, func(t *testing.T) {
			t.Parallel()
			var got auth.Caller
			var ok bool
			h := mustSet(t).Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got, ok = auth.FromContext(r.Context())
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(http.MethodPost, "/graphql", nil)
			req.Header.Set("Authorization", tt.scheme+" "+tt.token)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusNoContent || !ok {
				t.Fatalf("status = %d, caller in context = %v", rec.Code, ok)
			}
			if got.Name != tt.wantName {
				t.Errorf("caller = %q, want %q", got.Name, tt.wantName)
			}
			gotScopes := got.Scopes()
			slices.Sort(gotScopes)
			want := slices.Clone(tt.wantScopes)
			slices.Sort(want)
			if !reflect.DeepEqual(gotScopes, want) {
				t.Errorf("scopes = %v, want %v", gotScopes, want)
			}
			for _, s := range []string{auth.ScopeAudienceRead, auth.ScopeConsentRead, auth.ScopeConsentWrite} {
				if got.HasScope(s) != slices.Contains(want, s) {
					t.Errorf("HasScope(%q) = %v, want %v", s, got.HasScope(s), slices.Contains(want, s))
				}
			}
		})
	}
}

func TestFromContextWithoutMiddleware(t *testing.T) {
	t.Parallel()
	if _, ok := auth.FromContext(t.Context()); ok {
		t.Error("FromContext on a bare context reported a caller")
	}
}

func TestNewTokenSetValidation(t *testing.T) {
	t.Parallel()
	with := func(mut func(map[string]string)) map[string]string {
		m := validTokens()
		mut(m)
		return m
	}
	tests := []struct {
		name    string
		tokens  map[string]string
		wantErr []string // substrings; nil means success
	}{
		{"valid", validTokens(), nil},
		{"missing admin", with(func(m map[string]string) { delete(m, auth.CallerAdmin) }), []string{"missing required config", auth.KeyTokenAdmin}},
		{"blank notification", with(func(m map[string]string) { m[auth.CallerNotification] = "  " }), []string{auth.KeyTokenNotification}},
		{"all missing", map[string]string{}, []string{auth.KeyTokenProposal, auth.KeyTokenNotification, auth.KeyTokenAdmin}},
		{"duplicate", with(func(m map[string]string) { m[auth.CallerAdmin] = m[auth.CallerProposal] }),
			[]string{"must differ", auth.KeyTokenProposal, auth.KeyTokenAdmin}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			set, err := auth.NewTokenSet(tt.tokens)
			if tt.wantErr == nil {
				if err != nil || set == nil {
					t.Fatalf("NewTokenSet = %v, %v; want success", set, err)
				}
				return
			}
			if err == nil {
				t.Fatal("NewTokenSet succeeded, want error")
			}
			for _, sub := range tt.wantErr {
				if !strings.Contains(err.Error(), sub) {
					t.Errorf("error %q lacks %q", err, sub)
				}
			}
			for _, tok := range tt.tokens {
				if tok != "" && strings.Contains(err.Error(), tok) {
					t.Errorf("error %q leaks token %q", err, tok)
				}
			}
		})
	}
}

func TestLoadTokenSetFromConfig(t *testing.T) {
	t.Parallel()
	t.Run("all keys set", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		v.Set(auth.KeyTokenProposal, "a")
		v.Set(auth.KeyTokenNotification, "b")
		v.Set(auth.KeyTokenAdmin, "c")
		if _, err := auth.LoadTokenSet(v); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("missing key", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		v.Set(auth.KeyTokenProposal, "a")
		v.Set(auth.KeyTokenAdmin, "c")
		_, err := auth.LoadTokenSet(v)
		if err == nil || !strings.Contains(err.Error(), auth.KeyTokenNotification) {
			t.Fatalf("err = %v, want missing %s", err, auth.KeyTokenNotification)
		}
	})
	t.Run("duplicate tokens", func(t *testing.T) {
		t.Parallel()
		v := viper.New()
		v.Set(auth.KeyTokenProposal, "same")
		v.Set(auth.KeyTokenNotification, "same")
		v.Set(auth.KeyTokenAdmin, "c")
		_, err := auth.LoadTokenSet(v)
		if err == nil || !strings.Contains(err.Error(), "must differ") {
			t.Fatalf("err = %v, want duplicate-token error", err)
		}
	})
}
