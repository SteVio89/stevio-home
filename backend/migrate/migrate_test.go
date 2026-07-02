package migrate

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// openTestDB opens a Postgres connection scoped to a fresh schema. Skips
// the test if TEST_DATABASE_URL is not set. The schema is dropped via cleanup.
//
// This package can't import testutil because testutil imports migrate — so
// the schema-setup helper is duplicated here (~30 lines). Keep in sync with
// backend/testutil/testutil.go.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()

	baseDSN := os.Getenv("TEST_DATABASE_URL")
	if baseDSN == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping Postgres integration test")
	}

	var b [8]byte
	rand.Read(b[:])
	schema := "test_" + hex.EncodeToString(b[:])

	admin, err := sql.Open("pgx", baseDSN)
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}
	if _, err := admin.Exec(fmt.Sprintf("CREATE SCHEMA %q", schema)); err != nil {
		_ = admin.Close()
		t.Fatalf("create schema: %v", err)
	}
	_ = admin.Close()

	opt := "-c search_path=" + schema
	u, _ := url.Parse(baseDSN)
	q := u.Query()
	q.Set("options", opt)
	u.RawQuery = q.Encode()

	db, err := sql.Open("pgx", u.String())
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
		a, err := sql.Open("pgx", baseDSN)
		if err == nil {
			_, _ = a.Exec(fmt.Sprintf("DROP SCHEMA IF EXISTS %q CASCADE", schema))
			_ = a.Close()
		}
	})

	return db
}

func TestRunMigrations_Basic(t *testing.T) {
	db := openTestDB(t)

	fs := fstest.MapFS{
		"migrations/001_create.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT)`),
		},
	}

	if err := RunMigrations(db, "test", fs, nil); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	if _, err := db.Exec(`INSERT INTO users (id, name) VALUES ('1', 'Alice')`); err != nil {
		t.Errorf("users table should exist: %v", err)
	}

	var version string
	err := db.QueryRow(`SELECT version FROM schema_migrations WHERE version = $1`, "test/001_create.sql").Scan(&version)
	if err != nil {
		t.Errorf("migration version not recorded: %v", err)
	}
}

func TestRunMigrations_Idempotent(t *testing.T) {
	db := openTestDB(t)

	fs := fstest.MapFS{
		"migrations/001_create.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE things (id TEXT PRIMARY KEY)`),
		},
	}

	if err := RunMigrations(db, "test", fs, nil); err != nil {
		t.Fatal(err)
	}
	if err := RunMigrations(db, "test", fs, nil); err != nil {
		t.Errorf("second run should be idempotent: %v", err)
	}
}

func TestRunMigrations_MultiStatement(t *testing.T) {
	// Postgres handles multi-statement Exec natively.
	db := openTestDB(t)

	fs := fstest.MapFS{
		"migrations/001.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE kv (key TEXT PRIMARY KEY, value TEXT);
INSERT INTO kv (key, value) VALUES ('greeting', 'hello; world');`),
		},
	}

	if err := RunMigrations(db, "test", fs, nil); err != nil {
		t.Fatalf("multi-statement migration failed: %v", err)
	}

	var val string
	if err := db.QueryRow(`SELECT value FROM kv WHERE key = 'greeting'`).Scan(&val); err != nil {
		t.Fatal(err)
	}
	if val != "hello; world" {
		t.Errorf("got %q, want %q", val, "hello; world")
	}
}

func TestRunMigrations_PrefixNamespacing(t *testing.T) {
	db := openTestDB(t)

	fs := fstest.MapFS{
		"migrations/001.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE ns_test (id TEXT PRIMARY KEY)`),
		},
	}

	if err := RunMigrations(db, "alpha", fs, nil); err != nil {
		t.Fatal(err)
	}

	// Apply the same file under prefix "beta" — should fail because the
	// table already exists. This confirms namespacing prefixes don't
	// suppress the second application.
	err := RunMigrations(db, "beta", fs, nil)
	if err == nil {
		t.Error("expected error when applying same SQL under different prefix")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestRunMigrations_RollbackOnError(t *testing.T) {
	db := openTestDB(t)

	fs := fstest.MapFS{
		"migrations/001.sql": &fstest.MapFile{
			Data: []byte(`CREATE TABLE rollback_test (id TEXT PRIMARY KEY);
THIS IS INVALID SQL`),
		},
	}

	if err := RunMigrations(db, "test", fs, nil); err == nil {
		t.Fatal("expected error from invalid SQL")
	}

	// Table should not exist due to rollback.
	if _, err := db.Exec(`INSERT INTO rollback_test (id) VALUES ('1')`); err == nil {
		t.Error("rollback_test table should not exist after rollback")
	}

	var count int
	_ = db.QueryRow(`SELECT COUNT(*) FROM schema_migrations WHERE version = 'test/001.sql'`).Scan(&count)
	if count != 0 {
		t.Error("failed migration should not be recorded")
	}
}
