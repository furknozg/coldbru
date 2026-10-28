package anthropic

import (
	"fmt"

	"github.com/spf13/viper"

	"github.com/furknozg/coldbru/internal/platform/config"
)

// Config keys read from the environment (D-047).
const (
	// KeyAPIKey is the Claude API key. It is read here, through the platform config, and nowhere else.
	KeyAPIKey = "ANTHROPIC_API_KEY"
	// KeyBaseURL optionally points the client at another endpoint, for example a local proxy.
	KeyBaseURL = "ANTHROPIC_BASE_URL"
)

// Config configures a Client.
type Config struct {
	APIKey string
	// BaseURL overrides the API endpoint; empty means the production API. Tests set it to an
	// httptest server (D-025).
	BaseURL string
}

// LoadConfig reads the Config from v, a Viper built by config.New (environment, then .env). It
// fails naming the key, never the value.
func LoadConfig(v *viper.Viper) (Config, error) {
	if err := config.Require(v, KeyAPIKey); err != nil {
		return Config{}, err
	}
	return Config{APIKey: v.GetString(KeyAPIKey), BaseURL: v.GetString(KeyBaseURL)}, nil
}

// String omits the key so that a logged Config does not leak it.
func (c Config) String() string {
	return fmt.Sprintf("anthropic.Config{BaseURL: %q}", c.BaseURL)
}

// GoString keeps %#v from printing the key too.
func (c Config) GoString() string { return c.String() }
