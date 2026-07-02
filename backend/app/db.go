package app

import (
	"context"
	"database/sql"

	"github.com/SteVio89/stevio-home/dbutil"
)

// DB wraps *sql.DB. Handlers obtain it via c.DB(). The embedded *sql.DB is
// accessible as DB.DB for packages that require *sql.DB directly.
type DB struct {
	*sql.DB
}

// newDB creates a DB wrapper around an existing *sql.DB.
func newDB(sqlDB *sql.DB) *DB {
	return &DB{DB: sqlDB}
}

// WithTx runs fn inside a transaction using sql.LevelDefault. Rolls back on
// panic, commits on nil-error return. Postgres handles higher isolation
// levels if you need them — use dbutil.WithTx directly for those.
func (d *DB) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	return dbutil.WithTx(ctx, d.DB, fn)
}
