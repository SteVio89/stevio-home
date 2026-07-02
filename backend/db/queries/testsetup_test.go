package queries_test

import (
	"database/sql"
	"testing"

	"github.com/SteVio89/stevio-home/auth"
	"github.com/SteVio89/stevio-home/db"
	"github.com/SteVio89/stevio-home/i18n"
	"github.com/SteVio89/stevio-home/migrate"
	"github.com/SteVio89/stevio-home/testutil"
)

// setupAppDB creates a fresh Postgres schema with all migrations applied
// (auth, users, i18n, app). The schema is dropped via t.Cleanup. Skips the
// test if TEST_DATABASE_URL is unset (see backend/testutil for details).
//
// This lives here rather than in testutil to avoid an import cycle: testutil
// is imported by auth/i18n/users for their own unit tests, so testutil can't
// import those packages. Integration tests that need the full schema compose
// what they need directly — that's what this helper does.
func setupAppDB(t *testing.T) *sql.DB {
	t.Helper()

	// Core auth migrations first.
	database := testutil.SetupTestDB(t, "auth", auth.MigrationFiles)

	// Users table: create directly since the userstore migration is normally
	// applied via app.New() in production. Tests don't need the full framework
	// bootstrap.
	if _, err := database.Exec(`CREATE TABLE IF NOT EXISTS users (
		id          TEXT PRIMARY KEY,
		email_hash  TEXT NOT NULL UNIQUE,
		role        TEXT NOT NULL DEFAULT 'member',
		created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`); err != nil {
		t.Fatalf("setupAppDB: create users: %v", err)
	}
	if _, err := database.Exec(`CREATE INDEX IF NOT EXISTS idx_users_email_hash ON users(email_hash)`); err != nil {
		t.Fatalf("setupAppDB: create users index: %v", err)
	}

	// i18n migrations (creates locales, entity_translations, etc.).
	if err := migrate.RunMigrations(database, "i18n", i18n.MigrationFiles, nil); err != nil {
		t.Fatalf("setupAppDB: i18n migrations: %v", err)
	}

	// App-specific migrations.
	if err := migrate.RunMigrations(database, "app", db.MigrationFiles, nil); err != nil {
		t.Fatalf("setupAppDB: app migrations: %v", err)
	}

	return database
}
