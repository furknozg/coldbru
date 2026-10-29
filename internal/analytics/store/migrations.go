// Package store is the Analytics service's database layer: its goose migrations (D-083) and the
// sqlc-generated queries (D-020, D-051).
package store

import (
	"embed"
	"io/fs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrations returns the service's migration set, the directory that directly holds the numbered
// .sql files, in the form migrate.Up expects. It uses goose's default version table, separate
// from the platform set's goose_platform_version (D-054).
func Migrations() fs.FS {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		// The directory is embedded at compile time; this cannot fail.
		panic("store: embedded migrations directory missing: " + err.Error())
	}
	return sub
}
