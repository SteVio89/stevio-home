package settings

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"testing/fstest"

	"github.com/SteVio89/stevio-home/testutil"
)

// testSettingsMigration is an embedded in-test migration that creates the
// settings table this suite operates on. Using testutil.SetupTestDB gives
// per-test schema isolation against the shared Postgres instance.
var testSettingsMigration = fstest.MapFS{
	"migrations/001_settings.sql": &fstest.MapFile{
		Data: []byte(`CREATE TABLE test_settings (key TEXT PRIMARY KEY, value TEXT)`),
	},
}

func setupSettingsDB(t *testing.T) *sql.DB {
	t.Helper()
	return testutil.SetupTestDB(t, "settings", testSettingsMigration)
}

func mustNewStore(t *testing.T, db *sql.DB, table string) *Store {
	t.Helper()
	s, err := NewStore(db, table)
	if err != nil {
		t.Fatalf("NewStore(%q): %v", table, err)
	}
	return s
}

func TestGetUpsertRoundTrip(t *testing.T) {
	db := setupSettingsDB(t)
	s := mustNewStore(t, db, "test_settings")
	ctx := context.Background()

	if err := s.Upsert(ctx, "color", "blue"); err != nil {
		t.Fatal(err)
	}

	val, err := s.Get(ctx, "color")
	if err != nil {
		t.Fatal(err)
	}
	if val != "blue" {
		t.Errorf("Get(color) = %q, want %q", val, "blue")
	}

	// Upsert overwrites.
	if err := s.Upsert(ctx, "color", "red"); err != nil {
		t.Fatal(err)
	}
	val, err = s.Get(ctx, "color")
	if err != nil {
		t.Fatal(err)
	}
	if val != "red" {
		t.Errorf("Get(color) after update = %q, want %q", val, "red")
	}
}

func TestGetNotFound(t *testing.T) {
	db := setupSettingsDB(t)
	s := mustNewStore(t, db, "test_settings")
	ctx := context.Background()

	_, err := s.Get(ctx, "nonexistent")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Get(nonexistent) error = %v, want ErrNotFound", err)
	}
}

func TestGetInt(t *testing.T) {
	db := setupSettingsDB(t)
	s := mustNewStore(t, db, "test_settings")
	ctx := context.Background()

	tests := []struct {
		name     string
		key      string
		value    string // empty means don't insert
		fallback int
		want     int
	}{
		{"valid int", "count", "42", 0, 42},
		{"zero", "zero", "0", 99, 0},
		{"negative", "neg", "-5", 0, -5},
		{"non-numeric", "bad", "abc", 10, 10},
		{"missing key", "missing", "", 7, 7},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != "" {
				_ = s.Upsert(ctx, tt.key, tt.value)
			}
			got := s.GetInt(ctx, tt.key, tt.fallback)
			if got != tt.want {
				t.Errorf("GetInt(%q, %d) = %d, want %d", tt.key, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestGetAll(t *testing.T) {
	db := setupSettingsDB(t)
	s := mustNewStore(t, db, "test_settings")
	ctx := context.Background()

	_ = s.Upsert(ctx, "a", "1")
	_ = s.Upsert(ctx, "b", "2")

	all, err := s.GetAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("GetAll returned %d entries, want 2", len(all))
	}
	if all["a"] != "1" || all["b"] != "2" {
		t.Errorf("GetAll = %v, want map[a:1 b:2]", all)
	}
}

func TestNewStore_InvalidTableName(t *testing.T) {
	tests := []struct {
		name  string
		table string
	}{
		{"SQL injection", "settings; DROP TABLE users--"},
		{"spaces", "my table"},
		{"empty", ""},
		{"starts with number", "1settings"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewStore(nil, tt.table)
			if err == nil {
				t.Error("expected error for invalid table name")
			}
		})
	}
}
