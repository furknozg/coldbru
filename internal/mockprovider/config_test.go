package mockprovider

import (
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func vip(kv map[string]string) *viper.Viper {
	v := viper.New()
	for k, val := range kv {
		v.Set(k, val)
	}
	return v
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Parallel()
	c, err := LoadConfig(vip(nil))
	if err != nil {
		t.Fatal(err)
	}
	want := DefaultConfig()
	if c != want {
		t.Errorf("config = %+v, want %+v", c, want)
	}
	if c.ErrorRate != 0.05 || c.Rate429 != 0.02 || c.TimeoutRate != 0.01 || c.TimeoutDelay != 12*time.Second ||
		c.LatencyMin != 50*time.Millisecond || c.LatencyMax != 300*time.Millisecond || c.RateLimit != 110 ||
		c.IdempotencyTTL != 24*time.Hour || c.SMTPAddr != "mailpit:1025" || c.EmailFrom != "campaigns@coldbru.test" ||
		c.MockServerURL != "http://mockserver:1080" || c.HasSeed {
		t.Errorf("defaults differ from D-081: %+v", c)
	}
}

func TestLoadConfigValues(t *testing.T) {
	t.Parallel()
	c, err := LoadConfig(vip(map[string]string{
		KeyErrorRate: "0.5", KeyRate429: "0.25", KeyTimeoutRate: "0", KeyTimeoutDelay: "300ms",
		KeyLatencyMin: "1ms", KeyLatencyMax: "2ms", KeyRateLimit: "50", KeySeed: "7",
		KeyIdempotencyTTL: "1h", KeySMTPAddr: "localhost:2525", KeyEmailFrom: "a@b.test", KeyMockServerURL: "http://localhost:1080",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if c.ErrorRate != 0.5 || c.Rate429 != 0.25 || c.TimeoutRate != 0 || c.TimeoutDelay != 300*time.Millisecond ||
		c.RateLimit != 50 || !c.HasSeed || c.Seed != 7 || c.IdempotencyTTL != time.Hour || c.SMTPAddr != "localhost:2525" {
		t.Errorf("config = %+v", c)
	}
	if got := len(c.LogAttrs()); got%2 != 0 || got == 0 {
		t.Errorf("LogAttrs has %d items", got)
	}
}

func TestLoadConfigRejectsBadValues(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		kv   map[string]string
		want string
	}{
		{"rate above 1", map[string]string{KeyErrorRate: "1.5"}, KeyErrorRate},
		{"negative rate", map[string]string{KeyRate429: "-0.1"}, KeyRate429},
		{"rate not a number", map[string]string{KeyTimeoutRate: "often"}, KeyTimeoutRate},
		{"rates above 1 together", map[string]string{KeyErrorRate: "0.6", KeyRate429: "0.3", KeyTimeoutRate: "0.2"}, "add up"},
		{"latency inverted", map[string]string{KeyLatencyMin: "400ms", KeyLatencyMax: "100ms"}, KeyLatencyMax},
		{"bad duration", map[string]string{KeyTimeoutDelay: "12"}, KeyTimeoutDelay},
		{"rate limit zero", map[string]string{KeyRateLimit: "0"}, KeyRateLimit},
		{"bad seed", map[string]string{KeySeed: "-1"}, KeySeed},
		{"ttl zero", map[string]string{KeyIdempotencyTTL: "0s"}, KeyIdempotencyTTL},
		{"smtp without port", map[string]string{KeySMTPAddr: "mailpit"}, KeySMTPAddr},
		{"from not an address", map[string]string{KeyEmailFrom: "Campaigns <c@x.test>"}, KeyEmailFrom},
		{"mockserver not a url", map[string]string{KeyMockServerURL: "mockserver:1080"}, KeyMockServerURL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadConfig(vip(tc.kv))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want one naming %q", err, tc.want)
			}
		})
	}
}

func TestNewHandlerNeedsAForwarder(t *testing.T) {
	t.Parallel()
	if _, err := NewHandler(HandlerConfig{Config: DefaultConfig()}); err == nil {
		t.Error("NewHandler() without a Forwarder succeeded")
	}
	bad := DefaultConfig()
	bad.RateLimit = 0
	if _, err := NewHandler(HandlerConfig{Config: bad, Forwarder: &fakeForwarder{}}); err == nil {
		t.Error("NewHandler() with an invalid config succeeded")
	}
}
