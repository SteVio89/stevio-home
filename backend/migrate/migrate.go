package migrate

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"sort"
)

// RunMigrations applies all unapplied migrations from the given FS.
// The FS must contain files at "migrations/*.sql".
// The prefix namespaces version keys in schema_migrations (e.g. "core/001.sql").
// Pass nil for logger to use the default logger.
//
// Postgres handles multi-statement Exec natively, so each migration file is
// executed in a single Exec call inside a transaction.
func RunMigrations(db *sql.DB, prefix string, files fs.ReadFileFS, logger *log.Logger) error {
	if logger == nil {
		logger = log.Default()
	}

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("migrations: create tracking table: %w", err)
	}

	entries, err := fs.ReadDir(files, "migrations")
	if err != nil {
		return fmt.Errorf("migrations: read dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		version := prefix + "/" + e.Name()

		// Check both prefixed key and legacy unprefixed key (backward compat).
		var exists bool
		_ = db.QueryRow(
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version IN ($1, $2))`,
			version, e.Name(),
		).Scan(&exists)
		if exists {
			continue
		}

		content, err := fs.ReadFile(files, "migrations/"+e.Name())
		if err != nil {
			return fmt.Errorf("migrations: read %s: %w", version, err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("migrations: begin %s: %w", version, err)
		}
		if _, err := tx.Exec(string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrations: execute %s: %w", version, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrations: record %s: %w", version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migrations: commit %s: %w", version, err)
		}

		logger.Printf("migrate: applied %s", version)
	}
	return nil
}
