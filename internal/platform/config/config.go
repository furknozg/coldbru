// Package config loads service configuration from defaults, an optional .env
// file in the working directory, and environment variables (D-050).
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

// Platform configuration keys shared by every service.
const (
	// KeyHTTPAddr is the listen address of the HTTP server.
	KeyHTTPAddr = "HTTP_ADDR"
	// KeyLogLevel is the slog level: debug, info, warn or error.
	KeyLogLevel = "LOG_LEVEL"
	// KeyShutdownTimeout is the graceful shutdown deadline as a duration string.
	KeyShutdownTimeout = "SHUTDOWN_TIMEOUT"
	// KeyAPIDocsEnabled turns on GET /docs (Scalar) and GET /openapi.json for
	// services that embed an OpenAPI spec (D-057).
	KeyAPIDocsEnabled = "API_DOCS_ENABLED"
)

// envFile is the optional dotenv file read from the working directory.
const envFile = ".env"

// PlatformDefaults returns the default values for the platform keys.
func PlatformDefaults() map[string]any {
	return map[string]any{
		KeyHTTPAddr:        ":8080",
		KeyLogLevel:        "info",
		KeyShutdownTimeout: "10s",
		KeyAPIDocsEnabled:  false,
	}
}

// New returns a fresh Viper instance (never the global one) holding defaults.
// It reads ".env" from the working directory when the file exists, and binds
// environment variables without a prefix. Precedence: environment, then
// ".env", then defaults. A missing ".env" is not an error; any other read
// error is.
func New(defaults map[string]any) (*viper.Viper, error) {
	v := viper.New()
	for k, val := range defaults {
		v.SetDefault(k, val)
	}

	v.SetConfigFile(envFile)
	v.SetConfigType("env")
	if err := v.ReadInConfig(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", envFile, err)
	}

	v.AutomaticEnv()
	return v, nil
}

// Require returns one error naming every key that is missing or empty in v.
// It returns nil when all keys have a non-empty value.
func Require(v *viper.Viper, keys ...string) error {
	var missing []string
	for _, k := range keys {
		if strings.TrimSpace(v.GetString(k)) == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	sort.Strings(missing)
	return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
}
