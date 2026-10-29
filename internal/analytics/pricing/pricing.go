// Package pricing turns the token usage of a model call into US dollars (D-083). Prices are USD
// per million tokens, per model, kept as exact rationals so a cost is never off by float rounding.
package pricing

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

// KeyLLMPrices is the config key of the price overrides: a JSON object mapping a model name to its
// prices, for example
//
//	{"claude-sonnet-5-5":{"cache_read":"0.10"},"my-model":{"input":"1","output":"5"}}
//
// Each price is a non-negative decimal string (at most 6 decimals) in USD per million tokens. An
// entry changes only the prices it names; the others keep the default of a known model and are 0
// for a model without a default.
const KeyLLMPrices = "LLM_PRICES"

// CostDecimals is the number of decimals of a call's cost, the scale of llm_usage.cost_usd.
const CostDecimals = 8

// Price is what a model costs, in USD per million tokens.
type Price struct {
	Input         *big.Rat
	Output        *big.Rat
	CacheCreation *big.Rat // writing the prompt cache
	CacheRead     *big.Rat // reading the prompt cache
}

// Usage is the token usage of one call, as Proposal reports it.
type Usage struct {
	Input         int64
	Output        int64
	CacheRead     int64
	CacheCreation int64
}

// Table maps a model name to its price.
type Table map[string]Price

func rat(s string) *big.Rat {
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		panic("pricing: bad default price " + s)
	}
	return r
}

// Defaults are the prices of D-083, USD per million tokens. For Sonnet 5.5 cache reads the local
// document says 0.20 and the price page 0.10; the higher one is the default. The mock model is free.
func Defaults() Table {
	return Table{
		"claude-sonnet-5-5": {Input: rat("2"), Output: rat("10"), CacheCreation: rat("2.5"), CacheRead: rat("0.20")},
		"claude-haiku-5-5":  {Input: rat("0.10"), Output: rat("0.50"), CacheCreation: rat("0.125"), CacheRead: rat("0.01")},
		"mock":              {Input: rat("0"), Output: rat("0"), CacheCreation: rat("0"), CacheRead: rat("0")},
	}
}

var decimalRE = regexp.MustCompile(`^\d{1,9}(\.\d{1,6})?$`)

// overrideEntry is one model of LLM_PRICES; a nil field is not overridden.
type overrideEntry struct {
	Input         *string `json:"input"`
	Output        *string `json:"output"`
	CacheCreation *string `json:"cache_write"`
	CacheRead     *string `json:"cache_read"`
}

// Parse applies the JSON overrides to base and returns the result; base is not changed. An empty
// string means no overrides. An unknown field, a malformed price or a blank model name is an error.
func Parse(base Table, overrides string) (Table, error) {
	out := make(Table, len(base))
	for m, p := range base {
		out[m] = p
	}
	if strings.TrimSpace(overrides) == "" {
		return out, nil
	}
	var raw map[string]overrideEntry
	dec := json.NewDecoder(strings.NewReader(overrides))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("%s: %w", KeyLLMPrices, err)
	}
	for model, e := range raw {
		if strings.TrimSpace(model) == "" {
			return nil, fmt.Errorf("%s: blank model name", KeyLLMPrices)
		}
		p := out[model]
		for _, f := range []struct {
			name string
			in   *string
			dst  **big.Rat
		}{
			{"input", e.Input, &p.Input}, {"output", e.Output, &p.Output},
			{"cache_write", e.CacheCreation, &p.CacheCreation}, {"cache_read", e.CacheRead, &p.CacheRead},
		} {
			if f.in == nil {
				continue
			}
			s := strings.TrimSpace(*f.in)
			if !decimalRE.MatchString(s) {
				return nil, fmt.Errorf("%s: %s.%s %q is not a non-negative decimal with at most 6 decimals", KeyLLMPrices, model, f.name, *f.in)
			}
			*f.dst = rat(s)
		}
		out[model] = p.filled()
	}
	return out, nil
}

// filled replaces the prices a new model did not name by zero.
func (p Price) filled() Price {
	zero := func(r *big.Rat) *big.Rat {
		if r == nil {
			return new(big.Rat)
		}
		return r
	}
	return Price{Input: zero(p.Input), Output: zero(p.Output), CacheCreation: zero(p.CacheCreation), CacheRead: zero(p.CacheRead)}
}

// LoadConfig returns Defaults with the LLM_PRICES overrides of v applied.
func LoadConfig(v *viper.Viper) (Table, error) {
	return Parse(Defaults(), v.GetString(KeyLLMPrices))
}

// Cost is the price of usage on model as a decimal string with CostDecimals decimals, rounded half
// up. known is false for a model without a price: the cost is then "0.00000000" and the caller logs
// it.
func (t Table) Cost(model string, u Usage) (cost string, known bool) {
	p, ok := t[model]
	if !ok {
		return formatUSD(new(big.Rat)), false
	}
	sum := new(big.Rat)
	for _, c := range []struct {
		tokens int64
		price  *big.Rat
	}{{u.Input, p.Input}, {u.Output, p.Output}, {u.CacheRead, p.CacheRead}, {u.CacheCreation, p.CacheCreation}} {
		sum.Add(sum, new(big.Rat).Mul(big.NewRat(c.tokens, 1), c.price))
	}
	sum.Quo(sum, big.NewRat(1_000_000, 1)) // per million tokens
	return formatUSD(sum), true
}

// formatUSD renders r (non-negative) with CostDecimals decimals, rounded half up.
func formatUSD(r *big.Rat) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(CostDecimals), nil)
	// floor(r*scale + 1/2) = floor((2*num*scale + den) / (2*den))
	n := new(big.Int).Mul(r.Num(), scale)
	n.Mul(n, big.NewInt(2))
	n.Add(n, r.Denom())
	n.Quo(n, new(big.Int).Mul(r.Denom(), big.NewInt(2)))
	q, m := new(big.Int).QuoRem(n, scale, new(big.Int))
	return fmt.Sprintf("%s.%0*d", q.String(), CostDecimals, m.Int64())
}
