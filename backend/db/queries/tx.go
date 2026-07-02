package queries

import (
	"context"
	"database/sql"

	"github.com/SteVio89/stevio-home/dbutil"
)

// querier is implemented by both *sql.DB and *sql.Tx.
type querier = dbutil.Querier

// WithTx runs fn inside a transaction.
func WithTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	return dbutil.WithTx(ctx, db, fn)
}
