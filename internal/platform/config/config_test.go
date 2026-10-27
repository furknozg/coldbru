package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/furknozg/coldbru/internal/platform/config"
)

// These tests change the working directory and environment, so they cannot
// run in parallel.

func chdirWithEnv(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	if content != "" {
		if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(content), 0o600); err != nil {
			t.Fatalf("write .env: %v", err)
		}
	}
	t.Chdir(dir)
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		envFile string
		env     map[string]string
		want    string
	}{
		{name: "default used", want: ":8080"},
		{name: ".env value used", envFile: "HTTP_ADDR=:9090\n", want: ":9090"},
		{name: "real env overrides .env", envFile: "HTTP_ADDR=:9090\n", env: map[string]string{"HTTP_ADDR": ":7070"}, want: ":7070"},
		{name: "real env overrides default", env: map[string]string{"HTTP_ADDR": ":6060"}, want: ":6060"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chdirWithEnv(t, tt.envFile)
			for k, val := range tt.env {
				t.Setenv(k, val)
			}
			v, err := config.New(config.PlatformDefaults())
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if got := v.GetString(config.KeyHTTPAddr); got != tt.want {
				t.Errorf("HTTP_ADDR = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewMissingEnvFileIsNotAnError(t *testing.T) {
	chdirWithEnv(t, "")
	if _, err := config.New(nil); err != nil {
		t.Fatalf("New without .env: %v", err)
	}
}

func TestNewUnreadableEnvFileIsAnError(t *testing.T) {
	dir := t.TempDir()
	// A directory named .env exists but cannot be read as a file.
	if err := os.Mkdir(filepath.Join(dir, ".env"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	if _, err := config.New(nil); err == nil {
		t.Fatal("New with unreadable .env: want error, got nil")
	}
}

func TestRequire(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		keys    []string
		wantErr []string
	}{
		{name: "all present", env: map[string]string{"A_KEY": "x", "B_KEY": "y"}, keys: []string{"A_KEY", "B_KEY"}},
		{name: "reports every missing key", keys: []string{"A_KEY", "B_KEY"}, wantErr: []string{"A_KEY", "B_KEY"}},
		{name: "empty counts as missing", env: map[string]string{"A_KEY": "", "B_KEY": "y"}, keys: []string{"A_KEY", "B_KEY"}, wantErr: []string{"A_KEY"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chdirWithEnv(t, "")
			for k, val := range tt.env {
				t.Setenv(k, val)
			}
			v, err := config.New(nil)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			err = config.Require(v, tt.keys...)
			if len(tt.wantErr) == 0 {
				if err != nil {
					t.Fatalf("Require: unexpected error %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("Require: want error, got nil")
			}
			for _, k := range tt.wantErr {
				if !strings.Contains(err.Error(), k) {
					t.Errorf("error %q does not mention %s", err, k)
				}
			}
		})
	}
}
