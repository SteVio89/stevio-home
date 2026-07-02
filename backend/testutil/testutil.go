// Package testutil provides test helpers for Postgres-backed integration tests.
//
// Tests opt in by setting TEST_DATABASE_URL, e.g.:
//
//	export TEST_DATABASE_URL="postgres://stevio:stevio-dev-password@localhost:5432/stevio?sslmode=disable"
//	go test ./...
//
// If TEST_DATABASE_URL is unset, tests that use SetupTestDB are skipped so
// `go test ./...` stays green on a developer's laptop without a running
// Postgres. CI pipelines should always set it.
//
// Each call to SetupTestDB creates a fresh Postgres schema named test_<random>
// and sets search_path to that schema, so tests run in isolation and can run
// in parallel. The schema is dropped via t.Cleanup.
//
// testutil intentionally does NOT import auth/i18n/users/db — those packages
// import testutil for their own tests, and circular imports are not allowed
// within a single Go module.
package testutil

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/SteVio89/stevio-home/migrate"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const testDBEnvVar = "TEST_DATABASE_URL"

// SetupTestDB creates a fresh Postgres schema, applies the given migrations,
// and returns a *sql.DB scoped to that schema via search_path.
//
// Requires TEST_DATABASE_URL. Skips the test if unset.
//
// The schema is dropped and the connection closed via t.Cleanup.
func SetupTestDB(t *testing.T, prefix string, files fs.ReadFileFS) *sql.DB {
	t.Helper()

	baseDSN := os.Getenv(testDBEnvVar)
	if baseDSN == "" {
		t.Skipf("%s not set — skipping Postgres integration test", testDBEnvVar)
	}

	schema := newSchemaName()

	// 1. Create the schema on the admin connection.
	admin, err := sql.Open("pgx", baseDSN)
	if err != nil {
		t.Fatalf("testutil: open admin: %v", err)
	}
	if _, err := admin.Exec(fmt.Sprintf("CREATE SCHEMA %q", schema)); err != nil {
		_ = admin.Close()
		t.Fatalf("testutil: create schema %s: %v", schema, err)
	}
	_ = admin.Close()

	// 2. Open a fresh pool with search_path pinned to the test schema.
	//    Using the DSN query parameter means every connection in the pool
	//    inherits the setting, which matters because database/sql pools
	//    don't guarantee any single connection for multiple queries.
	testDSN := appendSearchPath(baseDSN, schema)
	db, err := sql.Open("pgx", testDSN)
	if err != nil {
		cleanupSchema(baseDSN, schema)
		t.Fatalf("testutil: open test: %v", err)
	}

	// 3. Apply migrations into the isolated schema.
	if err := migrate.RunMigrations(db, prefix, files, nil); err != nil {
		_ = db.Close()
		cleanupSchema(baseDSN, schema)
		t.Fatalf("testutil: run migrations for %s: %v", prefix, err)
	}

	t.Cleanup(func() {
		_ = db.Close()
		cleanupSchema(baseDSN, schema)
	})

	return db
}

// SeedLocale inserts a locale into the test DB.
func SeedLocale(t *testing.T, db *sql.DB, code, name string, isDefault bool) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO locales (code, name, is_default, enabled) VALUES ($1, $2, $3, TRUE)`,
		code, name, isDefault)
	if err != nil {
		t.Fatalf("seed locale %s: %v", code, err)
	}
}

// SeedEntityTranslation inserts an entity translation for testing.
func SeedEntityTranslation(t *testing.T, db *sql.DB, entityType, entityID, locale, field, value string) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO entity_translations (entity_type, entity_id, locale, field, value) VALUES ($1, $2, $3, $4, $5)`,
		entityType, entityID, locale, field, value)
	if err != nil {
		t.Fatalf("seed entity translation: %v", err)
	}
}

// newSchemaName returns a collision-resistant per-test schema name.
// Prefixed with "test_" so it's easy to spot and manually drop stragglers.
func newSchemaName() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("testutil: rand failed: " + err.Error())
	}
	return "test_" + hex.EncodeToString(b[:])
}

// appendSearchPath adds `options=-c search_path=<schema>` to a Postgres DSN.
// Preserves any existing query parameters.
func appendSearchPath(dsn, schema string) string {
	opt := "-c search_path=" + schema
	u, err := url.Parse(dsn)
	if err != nil {
		// Fall back to simple concatenation if the DSN isn't URL-shaped.
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		return dsn + sep + "options=" + url.QueryEscape(opt)
	}
	q := u.Query()
	q.Set("options", opt)
	u.RawQuery = q.Encode()
	return u.String()
}

// cleanupSchema drops the test schema. Best-effort — errors are ignored
// because t.Cleanup runs after the test has already passed/failed.
func cleanupSchema(baseDSN, schema string) {
	admin, err := sql.Open("pgx", baseDSN)
	if err != nil {
		return
	}
	defer func() { _ = admin.Close() }()
	_, _ = admin.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS %q CASCADE", schema))
}
