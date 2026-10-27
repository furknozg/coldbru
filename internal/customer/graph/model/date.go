package model

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"
)

// dateLayout is the Date wire format (D-058).
const dateLayout = time.DateOnly

// ErrInvalidDate reports a Date value that is not a YYYY-MM-DD string.
var ErrInvalidDate = errors.New("invalid date")

// Date is a calendar date, YYYY-MM-DD, without a time zone (D-058). The
// embedded Time is midnight UTC of that date.
type Date struct {
	time.Time
}

// ParseDate parses a strict YYYY-MM-DD string.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return Date{}, fmt.Errorf("%w: %q is not YYYY-MM-DD", ErrInvalidDate, s)
	}
	return Date{Time: t}, nil
}

// MarshalGQL writes d as a JSON string in YYYY-MM-DD form.
func (d Date) MarshalGQL(w io.Writer) {
	_, _ = io.WriteString(w, strconv.Quote(d.Time.UTC().Format(dateLayout)))
}

// UnmarshalGQL reads a Date from a GraphQL input value; only YYYY-MM-DD
// strings are accepted.
func (d *Date) UnmarshalGQL(v any) error {
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("%w: expected a string such as \"2026-01-31\", got %T", ErrInvalidDate, v)
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}
