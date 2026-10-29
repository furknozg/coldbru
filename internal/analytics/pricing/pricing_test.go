package pricing

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestCost(t *testing.T) {
	tests := []struct {
		name  string
		model string
		usage Usage
		want  string
	}{
		{"sonnet input and output", "claude-sonnet-5-5", Usage{Input: 1_000_000, Output: 1_000_000}, "12.00000000"},
		{"sonnet all four kinds", "claude-sonnet-5-5", Usage{Input: 1000, Output: 500, CacheRead: 20_000, CacheCreation: 4000},
			// 1000*2 + 500*10 + 20000*0.20 + 4000*2.5 = 2000 + 5000 + 4000 + 10000 = 21000 per 1e6
			"0.02100000"},
		{"haiku cache write and read", "claude-haiku-5-5", Usage{CacheRead: 1_000_000, CacheCreation: 1_000_000}, "0.13500000"},
		{"one token is exact at eight decimals", "claude-haiku-5-5", Usage{CacheRead: 1}, "0.00000001"},
		{"one sonnet cache read token", "claude-sonnet-5-5", Usage{CacheRead: 1}, "0.00000020"},
		{"mock is free", "mock", Usage{Input: 5000, Output: 5000, CacheRead: 5000, CacheCreation: 5000}, "0.00000000"},
		{"no tokens", "claude-sonnet-5-5", Usage{}, "0.00000000"},
		{"large call", "claude-sonnet-5-5", Usage{Input: 2_000_000_000}, "4000.00000000"},
	}
	table := Defaults()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, known := table.Cost(tt.model, tt.usage)
			if !known || got != tt.want {
				t.Errorf("Cost() = %q, %v, want %q, true", got, known, tt.want)
			}
		})
	}
}

func TestCostRoundsHalfUp(t *testing.T) {
	// 1 token at 0.05 USD/MTok = 5e-8 exactly; at 0.005 it is 5e-9, half of the last decimal.
	table, err := Parse(Defaults(), `{"m":{"input":"0.005","output":"0.004"}}`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		usage Usage
		want  string
	}{
		{Usage{Input: 1}, "0.00000001"},  // 0.5e-8 rounds up
		{Usage{Output: 1}, "0.00000000"}, // 0.4e-8 rounds down
		{Usage{Input: 3}, "0.00000002"},  // 1.5e-8 rounds up
		{Usage{Input: 1, Output: 1}, "0.00000001"},
	} {
		if got, _ := table.Cost("m", tt.usage); got != tt.want {
			t.Errorf("Cost(%+v) = %s, want %s", tt.usage, got, tt.want)
		}
	}
}

func TestCostUnknownModelIsZeroAndNotKnown(t *testing.T) {
	got, known := Defaults().Cost("claude-future-9", Usage{Input: 1_000_000})
	if known || got != "0.00000000" {
		t.Errorf("Cost() = %q, %v, want 0.00000000, false", got, known)
	}
}

func TestParseOverrides(t *testing.T) {
	base := Defaults()
	got, err := Parse(base, `{"claude-sonnet-5-5":{"cache_read":"0.10"},"new-model":{"input":"1","output":"5"}}`)
	if err != nil {
		t.Fatal(err)
	}
	if c, _ := got.Cost("claude-sonnet-5-5", Usage{CacheRead: 1_000_000}); c != "0.10000000" {
		t.Errorf("overridden cache_read = %s", c)
	}
	if c, _ := got.Cost("claude-sonnet-5-5", Usage{Input: 1_000_000}); c != "2.00000000" {
		t.Errorf("an entry changed a price it did not name: input = %s", c)
	}
	if c, known := got.Cost("new-model", Usage{Input: 1_000_000, Output: 1_000_000, CacheRead: 1_000_000}); !known || c != "6.00000000" {
		t.Errorf("new model = %s, %v: unnamed prices must be 0", c, known)
	}
	if c, _ := base.Cost("claude-sonnet-5-5", Usage{CacheRead: 1_000_000}); c != "0.20000000" {
		t.Errorf("Parse changed the base table: cache_read = %s", c)
	}
	if empty, err := Parse(base, "  "); err != nil || len(empty) != len(base) {
		t.Errorf("Parse(blank) = %v, %v", empty, err)
	}
}

func TestParseRejects(t *testing.T) {
	for name, in := range map[string]string{
		"not json":         `nope`,
		"array":            `[]`,
		"unknown field":    `{"m":{"inputs":"1"}}`,
		"negative":         `{"m":{"input":"-1"}}`,
		"number not text":  `{"m":{"input":1}}`,
		"seven decimals":   `{"m":{"input":"0.1234567"}}`,
		"exponent":         `{"m":{"input":"1e3"}}`,
		"fraction":         `{"m":{"input":"1/3"}}`,
		"empty price":      `{"m":{"input":""}}`,
		"blank model name": `{" ":{"input":"1"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(Defaults(), in)
			if err == nil || !strings.Contains(err.Error(), KeyLLMPrices) {
				t.Errorf("Parse(%s) error = %v, want one naming %s", in, err, KeyLLMPrices)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	v := viper.New()
	got, err := LoadConfig(v)
	if err != nil || len(got) != 3 {
		t.Fatalf("LoadConfig() = %v, %v, want the 3 defaults", got, err)
	}
	v.Set(KeyLLMPrices, `{"mock":{"input":"1"}}`)
	got, err = LoadConfig(v)
	if err != nil {
		t.Fatal(err)
	}
	if c, _ := got.Cost("mock", Usage{Input: 1_000_000}); c != "1.00000000" {
		t.Errorf("mock input = %s", c)
	}
}
