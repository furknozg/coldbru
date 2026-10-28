package anthropic_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/furknozg/coldbru/internal/platform/config"
	"github.com/furknozg/coldbru/internal/proposal/llm/anthropic"
)

func TestLoadConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(anthropic.KeyAPIKey, "sk-test-not-real")
	t.Setenv(anthropic.KeyBaseURL, "http://localhost:9999")
	v, err := config.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := anthropic.LoadConfig(v)
	if err != nil || cfg.APIKey != "sk-test-not-real" || cfg.BaseURL != "http://localhost:9999" {
		t.Fatalf("cfg = %+v, err = %v", cfg, err)
	}
}

func TestLoadConfigNamesTheMissingKeyNotTheValue(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(anthropic.KeyAPIKey, "")
	t.Setenv(anthropic.KeyBaseURL, "")
	v, _ := config.New(nil)
	_, err := anthropic.LoadConfig(v)
	if err == nil || !strings.Contains(err.Error(), anthropic.KeyAPIKey) {
		t.Errorf("err = %v", err)
	}
}

func TestConfigNeverPrintsTheKey(t *testing.T) {
	t.Parallel()
	cfg := anthropic.Config{APIKey: "sk-secret-value", BaseURL: "http://x"}
	for _, s := range []string{cfg.String(), fmt.Sprintf("%v", cfg), fmt.Sprintf("%+v", cfg), fmt.Sprintf("%#v", cfg)} {
		if strings.Contains(s, "sk-secret-value") {
			t.Errorf("formatted config leaks the key: %s", s)
		}
	}
}

func TestNewRequiresAKey(t *testing.T) {
	t.Parallel()
	if _, err := anthropic.New(anthropic.Config{APIKey: "  "}); err == nil || !strings.Contains(err.Error(), anthropic.KeyAPIKey) {
		t.Errorf("err = %v", err)
	}
	if _, err := anthropic.New(anthropic.Config{APIKey: "k"}); err != nil {
		t.Errorf("a key alone is enough (default base URL): %v", err)
	}
}
