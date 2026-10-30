package api

import (
	"net/url"
	"strconv"
	"time"
)

// Buckets of the series endpoints, and the longest range each accepts (D-097).
const (
	BucketHour = "hour"
	BucketDay  = "day"

	// MaxHourRange bounds a bucket=hour series to 168 buckets.
	MaxHourRange = 7 * 24 * time.Hour
	// MaxDayRange bounds a bucket=day series to 400 buckets.
	MaxDayRange = 400 * 24 * time.Hour
)

// parseTime reads an RFC 3339 instant and returns it in UTC.
func parseTime(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

// parseRange reads the optional from and to parameters. Both may be omitted (an open end); from
// must be before to when both are given. The problems are the individual messages for a 422.
func parseRange(q url.Values) (Range, []string) {
	var rg Range
	var problems []string
	for _, p := range []struct {
		name string
		dst  **time.Time
	}{{"from", &rg.From}, {"to", &rg.To}} {
		s := q.Get(p.name)
		if s == "" {
			continue
		}
		t, ok := parseTime(s)
		if !ok {
			problems = append(problems, p.name+": must be an RFC 3339 timestamp, like 2026-10-01T00:00:00Z")
			continue
		}
		*p.dst = &t
	}
	if len(problems) == 0 && rg.From != nil && rg.To != nil && !rg.From.Before(*rg.To) {
		problems = append(problems, "from: must be before to")
	}
	return rg, problems
}

// parseSeries reads the required from and to and the bucket of a series request, and checks the
// range against the limit of the bucket.
func parseSeries(q url.Values) (SeriesQuery, []string) {
	var sq SeriesQuery
	var problems []string
	var haveFrom, haveTo bool
	for _, p := range []struct {
		name string
		dst  *time.Time
		have *bool
	}{{"from", &sq.From, &haveFrom}, {"to", &sq.To, &haveTo}} {
		s := q.Get(p.name)
		if s == "" {
			problems = append(problems, p.name+": required")
			continue
		}
		t, ok := parseTime(s)
		if !ok {
			problems = append(problems, p.name+": must be an RFC 3339 timestamp, like 2026-10-01T00:00:00Z")
			continue
		}
		*p.dst, *p.have = t, true
	}
	var limit time.Duration
	switch b := q.Get("bucket"); b {
	case BucketHour:
		sq.Bucket, limit = b, MaxHourRange
	case BucketDay:
		sq.Bucket, limit = b, MaxDayRange
	case "":
		problems = append(problems, "bucket: required, hour or day")
	default:
		problems = append(problems, "bucket: must be hour or day")
	}
	if haveFrom && haveTo {
		switch {
		case !sq.From.Before(sq.To):
			problems = append(problems, "from: must be before to")
		case limit > 0 && sq.To.Sub(sq.From) > limit:
			problems = append(problems, "to: with bucket="+sq.Bucket+" the range is at most "+strconv.Itoa(int(limit/(24*time.Hour)))+" days")
		}
	}
	return sq, problems
}
