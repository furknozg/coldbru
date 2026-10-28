// Package customer is Proposal's client of the Customer GraphQL endpoint (D-041, D-043, D-067).
// It is hand-written over net/http and knows exactly two operations, audiencePreview and
// audienceRecipients, each with a fixed selection. Requests carry Proposal's bearer token (D-063),
// are retried only on transport failures and 502/503/504, and Customer's filter errors come back
// as *dsl.Error at the DSL position so that the repair loop can treat them like parser errors.
package customer

import (
	"fmt"
	"time"

	"github.com/spf13/viper"

	"github.com/furknozg/coldbru/internal/platform/config"
)

// Config keys read from the environment (D-067).
const (
	// KeyURL is the full URL of Customer's endpoint, for example http://customer:8080/graphql.
	KeyURL = "CUSTOMER_GRAPHQL_URL"
	// KeyToken is Proposal's bearer token, the value of Customer's CUSTOMER_TOKEN_PROPOSAL (D-063).
	KeyToken = "CUSTOMER_TOKEN"
)

// DefaultTimeout is the time limit of one attempt (D-067).
const DefaultTimeout = 10 * time.Second

// Config configures a Client.
type Config struct {
	URL   string
	Token string
	// Timeout limits one attempt, not the whole call. Zero means DefaultTimeout.
	Timeout time.Duration
}

// LoadConfig reads the Config from v, a Viper built by config.New. It fails naming every missing
// key, never the token.
func LoadConfig(v *viper.Viper) (Config, error) {
	if err := config.Require(v, KeyURL, KeyToken); err != nil {
		return Config{}, err
	}
	return Config{URL: v.GetString(KeyURL), Token: v.GetString(KeyToken)}, nil
}

// String omits the token so that a logged Config does not leak it.
func (c Config) String() string {
	return fmt.Sprintf("customer.Config{URL: %q, Timeout: %s}", c.URL, c.Timeout)
}
