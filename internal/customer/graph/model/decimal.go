// Package model holds the GraphQL types of the Customer service: the generated
// input and output models (models_gen.go) and the hand-written custom scalars.
package model

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
)

// decimalPattern is the accepted Decimal text (D-058): optional minus, integer
// part, and at most two fraction digits (money columns are numeric(12,2)).
var decimalPattern = regexp.MustCompile(`^-?[0-9]+(\.[0-9]{1,2})?$`)

// ErrInvalidDecimal reports a Decimal value that is not a string of the
// accepted form.
var ErrInvalidDecimal = errors.New("invalid decimal")

// Decimal is an exact decimal number carried as a string such as "1234.50"
// (D-058). Keeping it a string means no float rounding can enter between the
// client and the numeric SQL parameter.
type Decimal string

// ParseDecimal validates s and returns it as a Decimal.
func ParseDecimal(s string) (Decimal, error) {
	if !decimalPattern.MatchString(s) {
		return "", fmt.Errorf("%w: %q must match %s", ErrInvalidDecimal, s, decimalPattern)
	}
	return Decimal(s), nil
}

// String returns the decimal text.
func (d Decimal) String() string { return string(d) }

// MarshalGQL writes d as a JSON string.
func (d Decimal) MarshalGQL(w io.Writer) {
	_, _ = io.WriteString(w, strconv.Quote(string(d)))
}

// UnmarshalGQL reads a Decimal from a GraphQL input value. Only strings are
// accepted: JSON numbers are rejected so a float can never stand in for money.
func (d *Decimal) UnmarshalGQL(v any) error {
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("%w: expected a string such as \"1234.50\", got %T", ErrInvalidDecimal, v)
	}
	parsed, err := ParseDecimal(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
