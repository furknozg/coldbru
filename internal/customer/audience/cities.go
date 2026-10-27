package audience

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/agnivade/levenshtein"
	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"
)

// Key is the canonical comparison key of a name (D-040, D-062): ı, İ and I
// become i, then the text is NFD-decomposed, combining marks are dropped and
// the rest is lower-cased. "İstanbul", "Istanbul", "ISTANBUL" and "ıstanbul"
// all give "istanbul"; "Eskişehir" and "ESKISEHIR" both give "eskisehir".
// The rule lives in code, not in database collation.
func Key(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case 'ı', 'İ', 'I':
			return 'i'
		}
		return r
	}, s)
	s = norm.NFD.String(s)
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) {
			return -1
		}
		return r
	}, s)
	return strings.ToLower(s)
}

// Cities is the in-memory city dictionary: canonical key to the spelling stored
// in dim_customer.city (D-062). It is read-only after construction, so it is
// safe for concurrent use.
type Cities struct {
	byKey map[string]string
	names []string
}

// NewCities builds the dictionary from the stored city names. Two different
// names with the same key make the dictionary ambiguous and are an error.
func NewCities(names []string) (*Cities, error) {
	c := &Cities{byKey: make(map[string]string, len(names))}
	for _, n := range names {
		k := Key(n)
		if prev, dup := c.byKey[k]; dup && prev != n {
			return nil, fmt.Errorf("cities %q and %q share the key %q", prev, n, k)
		} else if dup {
			continue
		}
		c.byKey[k] = n
		c.names = append(c.names, n)
	}
	slices.Sort(c.names)
	return c, nil
}

// Querier is the subset of *pgxpool.Pool that LoadCities needs.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// LoadCities reads the distinct cities of dwh.dim_customer once, at startup.
// A new city appears after the service restarts (D-062).
func LoadCities(ctx context.Context, q Querier) (*Cities, error) {
	rows, err := q.Query(ctx, `SELECT DISTINCT city FROM dwh.dim_customer`)
	if err != nil {
		return nil, fmt.Errorf("load cities: %w", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("load cities: scan: %w", err)
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("load cities: %w", err)
	}
	c, err := NewCities(names)
	if err != nil {
		return nil, fmt.Errorf("load cities: %w", err)
	}
	return c, nil
}

// Lookup returns the stored spelling of the city that s names, ignoring case,
// the Turkish dotted/dotless i and diacritics.
func (c *Cities) Lookup(s string) (string, bool) {
	n, ok := c.byKey[Key(s)]
	return n, ok
}

// Names returns the canonical city names in sorted order.
func (c *Cities) Names() []string { return slices.Clone(c.names) }

// Suggest returns up to three known cities close to s.
func (c *Cities) Suggest(s string) []string { return Suggest(s, c.names) }

const (
	maxSuggestions = 3
	maxEditDist    = 2
)

// Suggest returns up to three of candidates that are near value: the key is
// within Levenshtein distance 2 of the candidate's key, or is a prefix of it
// (D-062). Closest first; ties break alphabetically by key. The result is never nil.
func Suggest(value string, candidates []string) []string {
	k := Key(value)
	out := []string{}
	if k == "" {
		return out
	}
	type scored struct {
		name, key string
		score     int
	}
	var hits []scored
	for _, cand := range candidates {
		ck := Key(cand)
		d := levenshtein.ComputeDistance(k, ck)
		switch {
		case d <= maxEditDist:
			hits = append(hits, scored{cand, ck, d})
		case strings.HasPrefix(ck, k):
			hits = append(hits, scored{cand, ck, maxEditDist + 1})
		}
	}
	slices.SortFunc(hits, func(a, b scored) int {
		return cmp.Or(cmp.Compare(a.score, b.score), cmp.Compare(a.key, b.key), cmp.Compare(a.name, b.name))
	})
	for _, h := range hits[:min(len(hits), maxSuggestions)] {
		out = append(out, h.name)
	}
	return out
}
