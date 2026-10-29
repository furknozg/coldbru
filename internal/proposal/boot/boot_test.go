package boot_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/viper"

	"github.com/furknozg/coldbru/internal/platform/app"
	"github.com/furknozg/coldbru/internal/platform/health"
	"github.com/furknozg/coldbru/internal/platform/httpserver"
	"github.com/furknozg/coldbru/internal/platform/metrics"
	"github.com/furknozg/coldbru/internal/proposal/api"
	"github.com/furknozg/coldbru/internal/proposal/boot"
	"github.com/furknozg/coldbru/internal/proposal/llm/anthropic"
	"github.com/furknozg/coldbru/internal/proposal/llm/mock"
)

// secret is a stand-in API key; no test ever sees a real one.
const secret = "sk-test-not-a-real-key"

func config(kv ...string) *viper.Viper {
	v := viper.New()
	for i := 0; i < len(kv); i += 2 {
		v.Set(kv[i], kv[i+1])
	}
	return v
}

func TestParseMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in      string
		want    boot.Mode
		wantErr bool
	}{
		{"", boot.ModeAuto, false}, {"  ", boot.ModeAuto, false}, {"auto", boot.ModeAuto, false},
		{"anthropic", boot.ModeAnthropic, false}, {"mock", boot.ModeMock, false},
		{"MOCK", boot.ModeMock, false}, {" Anthropic ", boot.ModeAnthropic, false},
		{"claude", "", true}, {"real", "", true}, {"true", "", true},
	}
	for _, tt := range tests {
		got, err := boot.ParseMode(tt.in)
		if got != tt.want || (err != nil) != tt.wantErr {
			t.Errorf("ParseMode(%q) = %q, %v; want %q, error %v", tt.in, got, err, tt.want, tt.wantErr)
		}
		if err != nil && !strings.Contains(err.Error(), "LLM_MODE") {
			t.Errorf("ParseMode(%q) error %q does not name the key", tt.in, err)
		}
	}
}

func TestSelectLLM(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		cfg      *viper.Viper
		wantMode boot.Mode
		wantMock bool
		wantErr  string
	}{
		{"auto without a key is the mock", config(), boot.ModeMock, true, ""},
		{"auto with an empty key is the mock", config(anthropic.KeyAPIKey, ""), boot.ModeMock, true, ""},
		{"auto with a blank key is the mock", config(anthropic.KeyAPIKey, "  \t"), boot.ModeMock, true, ""},
		{"auto with a key is the real model", config(anthropic.KeyAPIKey, secret), boot.ModeAnthropic, false, ""},
		{"explicit auto", config(boot.KeyLLMMode, "auto", anthropic.KeyAPIKey, secret), boot.ModeAnthropic, false, ""},
		{"anthropic with a key", config(boot.KeyLLMMode, "anthropic", anthropic.KeyAPIKey, secret), boot.ModeAnthropic, false, ""},
		{"anthropic without a key fails", config(boot.KeyLLMMode, "anthropic"), "", false, "ANTHROPIC_API_KEY"},
		{"anthropic with a blank key fails", config(boot.KeyLLMMode, "anthropic", anthropic.KeyAPIKey, " "), "", false, "ANTHROPIC_API_KEY"},
		{"mock ignores the key", config(boot.KeyLLMMode, "mock", anthropic.KeyAPIKey, secret), boot.ModeMock, true, ""},
		{"mock without a key", config(boot.KeyLLMMode, "mock"), boot.ModeMock, true, ""},
		{"unknown mode fails", config(boot.KeyLLMMode, "gpt"), "", false, "LLM_MODE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			l, mode, err := boot.SelectLLM(tt.cfg)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want one naming %s", err, tt.wantErr)
				}
				if l != nil || strings.Contains(err.Error(), secret) {
					t.Errorf("an error returned a model or leaked the key: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if mode != tt.wantMode {
				t.Errorf("mode = %q, want %q", mode, tt.wantMode)
			}
			_, isMock := l.(*mock.LLM)
			_, isReal := l.(*anthropic.Client)
			if isMock != tt.wantMock || isReal == tt.wantMock {
				t.Errorf("model is %T, want mock = %v", l, tt.wantMock)
			}
		})
	}
}

// ---- Setup ----

type rig struct {
	deps *app.Deps
	logs *bytes.Buffer
}

// newRig builds the Deps the platform skeleton would hand to Setup, with a pool that never connects
// (nothing listens on port 1; pgxpool connects lazily).
func newRig(t *testing.T, cfg *viper.Viper) *rig {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), "postgres://nobody@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	logs := &bytes.Buffer{}
	logger := slog.New(slog.NewTextHandler(logs, nil))
	reg := health.NewRegistry()
	return &rig{logs: logs, deps: &app.Deps{
		Router: httpserver.NewRouter(logger, metrics.New(), reg), Config: cfg, Logger: logger, Health: reg, Pool: pool,
	}}
}

func validConfig(kv ...string) *viper.Viper {
	return config(append([]string{"CUSTOMER_GRAPHQL_URL", "http://customer:8080/graphql", "CUSTOMER_TOKEN", "proposal-dev-token"}, kv...)...)
}

func (r *rig) get(path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.deps.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestSetupInMockMode(t *testing.T) {
	t.Parallel()
	r := newRig(t, validConfig())
	if err := boot.Setup(t.Context(), r.deps); err != nil {
		t.Fatalf("Setup: %v", err)
	}

	// The routes of the API are mounted next to the platform ones.
	rec := r.get("/campaigns/not-a-uuid")
	if rec.Code != http.StatusBadRequest || rec.Header().Get("Content-Type") != api.ProblemContentType {
		t.Errorf("GET /campaigns/not-a-uuid = %d %q: the API routes are not mounted", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec := r.get("/healthz"); rec.Code != http.StatusOK {
		t.Errorf("/healthz = %d", rec.Code)
	}

	// /readyz names the mode; the postgres check does not exist in this rig, so it only has the llm one.
	rec = r.get("/readyz")
	var rep struct {
		Status string
		Checks map[string]struct{ Status, Detail string }
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || rep.Checks["llm"].Status != "ok" || rep.Checks["llm"].Detail != "mode=mock" {
		t.Errorf("/readyz = %d %s", rec.Code, rec.Body.String())
	}
	if out := r.logs.String(); !strings.Contains(out, "mode=mock") {
		t.Errorf("the chosen mode is not logged: %s", out)
	}
}

func TestSetupReportsTheRealModeWithoutCallingIt(t *testing.T) {
	t.Parallel()
	r := newRig(t, validConfig(anthropic.KeyAPIKey, secret, "ANTHROPIC_BASE_URL", "http://127.0.0.1:1"))
	if err := boot.Setup(t.Context(), r.deps); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	rec := r.get("/readyz")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"detail":"mode=anthropic"`) {
		t.Errorf("/readyz = %d %s: the real mode is ok without a network call", rec.Code, rec.Body.String())
	}
	if strings.Contains(r.logs.String(), secret) || strings.Contains(rec.Body.String(), secret) {
		t.Error("the API key leaked into the log or the readiness report")
	}
}

func TestSetupFailsOnInvalidConfiguration(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		cfg  *viper.Viper
		want []string
	}{
		{"no customer settings", config(), []string{"CUSTOMER_GRAPHQL_URL", "CUSTOMER_TOKEN"}},
		{"no customer token", config("CUSTOMER_GRAPHQL_URL", "http://customer:8080/graphql"), []string{"CUSTOMER_TOKEN"}},
		{"bad cta url", validConfig("CAMPAIGN_CTA_URL", "ftp://x"), []string{"CAMPAIGN_CTA_URL"}},
		{"bad sms price", validConfig("SMS_SEGMENT_PRICE_USD", "cheap"), []string{"SMS_SEGMENT_PRICE_USD"}},
		{"bad email price", validConfig("EMAIL_PRICE_USD", "-1"), []string{"EMAIL_PRICE_USD"}},
		{"bad llm mode", validConfig(boot.KeyLLMMode, "gpt"), []string{"LLM_MODE"}},
		{"anthropic without a key", validConfig(boot.KeyLLMMode, "anthropic"), []string{"ANTHROPIC_API_KEY"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := boot.Setup(t.Context(), newRig(t, tt.cfg).deps)
			if err == nil {
				t.Fatal("Setup succeeded, want an error")
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Errorf("error %q does not name %s", err, w)
				}
			}
			if strings.Contains(err.Error(), "proposal-dev-token") {
				t.Errorf("error %q leaks the token", err)
			}
		})
	}
}
