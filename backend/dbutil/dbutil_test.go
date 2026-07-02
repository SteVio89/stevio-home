package dbutil

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"testing/fstest"

	"github.com/SteVio89/stevio-home/testutil"
)

var dbutilTestMigration = fstest.MapFS{
	"migrations/001_init.sql": &fstest.MapFile{
		Data: []byte(`CREATE TABLE tx_test (id TEXT PRIMARY KEY);
CREATE TABLE tx_test2 (id TEXT PRIMARY KEY);`),
	},
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return testutil.SetupTestDB(t, "dbutil", dbutilTestMigration)
}

func TestNewID(t *testing.T) {
	uuidRe := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

	id := NewID()
	if !uuidRe.MatchString(id) {
		t.Errorf("NewID() = %q, does not match UUID v4 pattern", id)
	}

	// Uniqueness: two calls should never collide.
	id2 := NewID()
	if id == id2 {
		t.Error("NewID() produced duplicate IDs")
	}
}

func TestClampPagination(t *testing.T) {
	tests := []struct {
		name             string
		page, perPage    int
		wantPage, wantPP int
	}{
		{"valid values", 2, 25, 2, 25},
		{"page below 1", 0, 25, 1, 25},
		{"negative page", -5, 25, 1, 25},
		{"perPage below 1", 1, 0, 1, 20},
		{"negative perPage", 1, -1, 1, 20},
		{"perPage over 100", 1, 200, 1, 100},
		{"both invalid", -1, -1, 1, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPage, gotPP := ClampPagination(tt.page, tt.perPage)
			if gotPage != tt.wantPage || gotPP != tt.wantPP {
				t.Errorf("ClampPagination(%d, %d) = (%d, %d), want (%d, %d)",
					tt.page, tt.perPage, gotPage, gotPP, tt.wantPage, tt.wantPP)
			}
		})
	}
}

func TestWithTx(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	t.Run("commit", func(t *testing.T) {
		err := WithTx(ctx, db, func(tx *sql.Tx) error {
			_, err := tx.Exec(`INSERT INTO tx_test (id) VALUES ('a')`)
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
		var id string
		if err := db.QueryRow(`SELECT id FROM tx_test WHERE id = 'a'`).Scan(&id); err != nil {
			t.Errorf("committed row not found: %v", err)
		}
	})

	t.Run("rollback", func(t *testing.T) {
		testErr := errors.New("rollback me")
		err := WithTx(ctx, db, func(tx *sql.Tx) error {
			_, _ = tx.Exec(`INSERT INTO tx_test (id) VALUES ('b')`)
			return testErr
		})
		if !errors.Is(err, testErr) {
			t.Errorf("expected testErr, got %v", err)
		}
		var count int
		_ = db.QueryRow(`SELECT COUNT(*) FROM tx_test WHERE id = 'b'`).Scan(&count)
		if count != 0 {
			t.Error("rolled-back row should not exist")
		}
	})
}

func TestWithTx_CommitFailureRollsBack(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	// After a failed transaction, the database should still be usable
	// (the connection should be released back to the pool).
	testErr := errors.New("fn error")
	_ = WithTx(ctx, db, func(tx *sql.Tx) error {
		return testErr
	})

	// Verify the DB is still functional (connection not stuck).
	_, err := db.ExecContext(ctx, `INSERT INTO tx_test2 (id) VALUES ('works')`)
	if err != nil {
		t.Fatalf("DB should still be usable after rollback, got: %v", err)
	}
}

func TestNumberPlaceholders(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		offset int
		want   string
	}{
		{"single", "SELECT ?", 0, "SELECT $1"},
		{"multiple", "WHERE a = ? AND b = ?", 0, "WHERE a = $1 AND b = $2"},
		{"with offset", "LIMIT ? OFFSET ?", 3, "LIMIT $4 OFFSET $5"},
		{"none", "SELECT * FROM t", 0, "SELECT * FROM t"},
		{"question mark in string literal left alone",
			"WHERE note = 'what?' AND id = ?", 0,
			"WHERE note = 'what?' AND id = $1"},
		{"escaped quote in string",
			"WHERE note = 'it''s ?' AND id = ?", 0,
			"WHERE note = 'it''s ?' AND id = $1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NumberPlaceholders(tt.in, tt.offset)
			if got != tt.want {
				t.Errorf("NumberPlaceholders(%q, %d) = %q, want %q",
					tt.in, tt.offset, got, tt.want)
			}
		})
	}
}

func TestInPlaceholders(t *testing.T) {
	tests := []struct {
		name   string
		count  int
		offset int
		want   string
	}{
		{"zero count", 0, 0, ""},
		{"one from start", 1, 0, "$1"},
		{"three from start", 3, 0, "$1,$2,$3"},
		{"two with offset", 2, 2, "$3,$4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InPlaceholders(tt.count, tt.offset)
			if got != tt.want {
				t.Errorf("InPlaceholders(%d, %d) = %q, want %q",
					tt.count, tt.offset, got, tt.want)
			}
		})
	}
}
