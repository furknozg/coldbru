package customer

import (
	"strings"
	"testing"

	"github.com/furknozg/coldbru/internal/platform/config"
)

func TestLoadConfig(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(KeyURL, "http://customer:8080/graphql")
	t.Setenv(KeyToken, "tok")
	v, err := config.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(v)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.URL != "http://customer:8080/graphql" || cfg.Token != "tok" || cfg.Timeout != 0 {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestLoadConfigNamesMissingKeysNotValues(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv(KeyURL, "")
	t.Setenv(KeyToken, "")
	v, _ := config.New(nil)
	_, err := LoadConfig(v)
	if err == nil || !strings.Contains(err.Error(), KeyURL) || !strings.Contains(err.Error(), KeyToken) {
		t.Errorf("err = %v, want both keys named", err)
	}
}
