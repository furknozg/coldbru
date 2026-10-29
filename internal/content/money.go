package content

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Money is an amount of US dollars in micro-dollars (1e-6 USD), an integer, so that prices,
// products and sums are exact and never drift the way float64 does (D-071). It is shown rounded
// half up to four decimals, the precision of the stored estimates; prices may have up to six.
type Money int64

var moneyRE = regexp.MustCompile(`^(\d+)(?:\.(\d{1,6}))?$`)

// ParseMoney reads a non-negative decimal such as "0.04" or "0.0330" (up to six decimals).
func ParseMoney(s string) (Money, error) {
	m := moneyRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, fmt.Errorf("content: %q is not a non-negative amount with at most 6 decimals", s)
	}
	whole, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil || whole > (1<<62)/1_000_000 {
		return 0, fmt.Errorf("content: amount %q is too large", s)
	}
	frac := int64(0)
	if m[2] != "" {
		frac, _ = strconv.ParseInt((m[2] + "000000")[:6], 10, 64)
	}
	return Money(whole*1_000_000 + frac), nil
}

// String is the amount in dollars with four decimals, rounded half up: Money(33000) is "0.0330".
func (m Money) String() string {
	if m < 0 {
		return "-" + (-m).String()
	}
	u := (int64(m) + 50) / 100 // units of 1e-4
	return fmt.Sprintf("%d.%04d", u/10_000, u%10_000)
}

// Times returns m multiplied by the non-negative count n.
func (m Money) Times(n int) Money {
	if n < 0 {
		n = 0
	}
	return m * Money(n)
}

// Prices are what one SMS segment and one email cost (D-071). They are demo values, not the rates
// of a real provider.
type Prices struct {
	SMSSegment Money
	Email      Money
}

// EstimateSMS is segments x recipients x segment price.
func (p Prices) EstimateSMS(segments, recipients int) Money {
	return p.SMSSegment.Times(segments).Times(recipients)
}

// EstimateEmail is recipients x email price.
func (p Prices) EstimateEmail(recipients int) Money { return p.Email.Times(recipients) }

// SumMoney adds amounts given as decimal strings; an empty string counts as zero and a malformed
// one is an error.
func SumMoney(amounts ...string) (Money, error) {
	var sum Money
	for _, a := range amounts {
		if strings.TrimSpace(a) == "" {
			continue
		}
		m, err := ParseMoney(a)
		if err != nil {
			return 0, err
		}
		sum += m
	}
	return sum, nil
}
