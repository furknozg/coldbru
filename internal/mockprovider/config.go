package mockprovider

import (
	"fmt"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Configuration keys (D-081).
const (
	KeyErrorRate      = "MOCK_ERROR_RATE"
	KeyRate429        = "MOCK_RATE_429"
	KeyTimeoutRate    = "MOCK_TIMEOUT_RATE"
	KeyTimeoutDelay   = "MOCK_TIMEOUT_DELAY"
	KeyLatencyMin     = "MOCK_LATENCY_MIN"
	KeyLatencyMax     = "MOCK_LATENCY_MAX"
	KeyRateLimit      = "MOCK_RATE_LIMIT"
	KeySeed           = "MOCK_SEED"
	KeyIdempotencyTTL = "MOCK_IDEMPOTENCY_TTL"
	KeySMTPAddr       = "MAILPIT_SMTP_ADDR"
	KeyEmailFrom      = "MOCK_EMAIL_FROM"
	KeyMockServerURL  = "MOCKSERVER_URL"
)

// Defaults of the configuration.
const (
	DefaultErrorRate      = 0.05
	DefaultRate429        = 0.02
	DefaultTimeoutRate    = 0.01
	DefaultTimeoutDelay   = 12 * time.Second
	DefaultLatencyMin     = 50 * time.Millisecond
	DefaultLatencyMax     = 300 * time.Millisecond
	DefaultRateLimit      = 110
	DefaultIdempotencyTTL = 24 * time.Hour
	DefaultSMTPAddr       = "mailpit:1025"
	DefaultEmailFrom      = "campaigns@coldbru.test"
	DefaultMockServerURL  = "http://mockserver:1080"
)

// Burst is the burst of the per-channel token bucket: Notification's own limiter allows a burst of 10.
const Burst = 10

// Config configures the mock provider. A zero Config is not valid; use LoadConfig or DefaultConfig.
type Config struct {
	// ErrorRate, Rate429 and TimeoutRate are the probabilities of one request being answered with
	// a 502/503, a spontaneous 429 or a lost answer. They are exclusive and add up to at most 1.
	ErrorRate, Rate429, TimeoutRate float64
	// TimeoutDelay is how long the answer of a "timeout" request is held back.
	TimeoutDelay time.Duration
	// LatencyMin and LatencyMax bound the uniform latency of a request.
	LatencyMin, LatencyMax time.Duration
	// RateLimit is the requests per second of one channel; Burst is its bucket size.
	RateLimit float64
	Burst     int
	// Seed makes the random choices deterministic when HasSeed is set.
	Seed    uint64
	HasSeed bool
	// IdempotencyTTL is how long an answer is kept under its key.
	IdempotencyTTL time.Duration
	// SMTPAddr is the host:port of Mailpit's SMTP server; EmailFrom the sender address.
	SMTPAddr  string
	EmailFrom string
	// MockServerURL is the base URL of MockServer, the inbox of the SMS.
	MockServerURL string
}

// DefaultConfig returns the configuration with every default of D-081.
func DefaultConfig() Config {
	return Config{
		ErrorRate: DefaultErrorRate, Rate429: DefaultRate429, TimeoutRate: DefaultTimeoutRate,
		TimeoutDelay: DefaultTimeoutDelay,
		LatencyMin:   DefaultLatencyMin, LatencyMax: DefaultLatencyMax,
		RateLimit: DefaultRateLimit, Burst: Burst,
		IdempotencyTTL: DefaultIdempotencyTTL,
		SMTPAddr:       DefaultSMTPAddr, EmailFrom: DefaultEmailFrom, MockServerURL: DefaultMockServerURL,
	}
}

// LoadConfig reads the configuration from v. An unset or empty key takes its default; a malformed or
// out-of-range value is an error that names the key.
func LoadConfig(v *viper.Viper) (Config, error) {
	c := DefaultConfig()
	var err error
	if c.ErrorRate, err = floatKey(v, KeyErrorRate, c.ErrorRate); err != nil {
		return Config{}, err
	}
	if c.Rate429, err = floatKey(v, KeyRate429, c.Rate429); err != nil {
		return Config{}, err
	}
	if c.TimeoutRate, err = floatKey(v, KeyTimeoutRate, c.TimeoutRate); err != nil {
		return Config{}, err
	}
	if c.TimeoutDelay, err = durationKey(v, KeyTimeoutDelay, c.TimeoutDelay); err != nil {
		return Config{}, err
	}
	if c.LatencyMin, err = durationKey(v, KeyLatencyMin, c.LatencyMin); err != nil {
		return Config{}, err
	}
	if c.LatencyMax, err = durationKey(v, KeyLatencyMax, c.LatencyMax); err != nil {
		return Config{}, err
	}
	if c.RateLimit, err = floatKey(v, KeyRateLimit, c.RateLimit); err != nil {
		return Config{}, err
	}
	if c.IdempotencyTTL, err = durationKey(v, KeyIdempotencyTTL, c.IdempotencyTTL); err != nil {
		return Config{}, err
	}
	if s := strings.TrimSpace(v.GetString(KeySeed)); s != "" {
		seed, perr := strconv.ParseUint(s, 10, 64)
		if perr != nil {
			return Config{}, fmt.Errorf("%s: %q is not an unsigned integer", KeySeed, s)
		}
		c.Seed, c.HasSeed = seed, true
	}
	c.SMTPAddr = stringKey(v, KeySMTPAddr, c.SMTPAddr)
	c.EmailFrom = stringKey(v, KeyEmailFrom, c.EmailFrom)
	c.MockServerURL = stringKey(v, KeyMockServerURL, c.MockServerURL)
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

// Validate reports the first invalid field.
func (c Config) Validate() error {
	for _, r := range []struct {
		key string
		v   float64
	}{{KeyErrorRate, c.ErrorRate}, {KeyRate429, c.Rate429}, {KeyTimeoutRate, c.TimeoutRate}} {
		if r.v < 0 || r.v > 1 {
			return fmt.Errorf("%s: %v is not between 0 and 1", r.key, r.v)
		}
	}
	if sum := c.ErrorRate + c.Rate429 + c.TimeoutRate; sum > 1 {
		return fmt.Errorf("%s + %s + %s add up to %v, more than 1", KeyErrorRate, KeyRate429, KeyTimeoutRate, sum)
	}
	if c.TimeoutDelay < 0 {
		return fmt.Errorf("%s: must not be negative", KeyTimeoutDelay)
	}
	if c.LatencyMin < 0 {
		return fmt.Errorf("%s: must not be negative", KeyLatencyMin)
	}
	if c.LatencyMax < c.LatencyMin {
		return fmt.Errorf("%s (%s) is below %s (%s)", KeyLatencyMax, c.LatencyMax, KeyLatencyMin, c.LatencyMin)
	}
	if c.RateLimit <= 0 {
		return fmt.Errorf("%s: must be above 0", KeyRateLimit)
	}
	if c.Burst < 1 {
		return fmt.Errorf("burst must be at least 1")
	}
	if c.IdempotencyTTL <= 0 {
		return fmt.Errorf("%s: must be above 0", KeyIdempotencyTTL)
	}
	if host, port, ok := strings.Cut(c.SMTPAddr, ":"); !ok || host == "" || port == "" || strings.Contains(port, ":") {
		return fmt.Errorf("%s: %q is not host:port", KeySMTPAddr, c.SMTPAddr)
	}
	if a, err := mail.ParseAddress(c.EmailFrom); err != nil || a.Address != c.EmailFrom {
		return fmt.Errorf("%s: %q is not a bare email address", KeyEmailFrom, c.EmailFrom)
	}
	if u, err := url.Parse(c.MockServerURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("%s: %q is not an absolute http or https URL", KeyMockServerURL, c.MockServerURL)
	}
	return nil
}

// LogAttrs describes the configuration for the start-up log line.
func (c Config) LogAttrs() []any {
	seed := "random"
	if c.HasSeed {
		seed = strconv.FormatUint(c.Seed, 10)
	}
	return []any{
		"error_rate", c.ErrorRate, "rate_429", c.Rate429, "timeout_rate", c.TimeoutRate,
		"timeout_delay", c.TimeoutDelay.String(),
		"latency_min", c.LatencyMin.String(), "latency_max", c.LatencyMax.String(),
		"rate_limit_per_channel", c.RateLimit, "burst", c.Burst, "seed", seed,
		"idempotency_ttl", c.IdempotencyTTL.String(),
		"smtp_addr", c.SMTPAddr, "email_from", c.EmailFrom, "mockserver_url", c.MockServerURL,
	}
}

func stringKey(v *viper.Viper, key, def string) string {
	if s := strings.TrimSpace(v.GetString(key)); s != "" {
		return s
	}
	return def
}

func floatKey(v *viper.Viper, key string, def float64) (float64, error) {
	s := strings.TrimSpace(v.GetString(key))
	if s == "" {
		return def, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a number", key, s)
	}
	return f, nil
}

func durationKey(v *viper.Viper, key string, def time.Duration) (time.Duration, error) {
	s := strings.TrimSpace(v.GetString(key))
	if s == "" {
		return def, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("%s: %q is not a duration such as 250ms", key, s)
	}
	return d, nil
}
