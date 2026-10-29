package progress

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// nonNilPool is a pool that never connects; the tests that take it do not use the database.
func nonNilPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	p, err := pgxpool.New(t.Context(), "postgres://nobody@127.0.0.1:1/none?sslmode=disable&connect_timeout=1")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}
