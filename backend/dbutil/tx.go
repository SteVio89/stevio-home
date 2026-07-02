package dbutil

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// Querier is the common interface implemented by both *sql.DB and *sql.Tx,
// enabling functions to work with either.
type Querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// WithTx runs fn inside a transaction using sql.LevelDefault. Rolls back on
// any error from fn; commits on nil. If the commit itself fails, the tx is
// rolled back to release the connection.
//
// Postgres supports higher isolation levels (LevelReadCommitted, LevelSerializable)
// if you need them — open the tx via db.BeginTx directly in that case.
func WithTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelDefault})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		_ = tx.Rollback()
		return err
	}
	return nil
}

// NumberPlaceholders rewrites positional "?" placeholders in sql to Postgres'
// "$1, $2, ..." style. Numbering starts at offset+1 and increments for each
// non-quoted "?" encountered.
//
// Pass offset=0 when the SQL string's first "?" should become "$1". Use a
// larger offset when stitching in additional args that come before the
// placeholders in the final args slice.
//
// "?" characters inside single-quoted SQL string literals are left alone.
// Escaped quotes (”) inside strings are handled.
func NumberPlaceholders(sql string, offset int) string {
	var b strings.Builder
	b.Grow(len(sql) + 8)
	n := offset
	inString := false
	for i := 0; i < len(sql); i++ {
		c := sql[i]
		switch c {
		case '\'':
			// Handle escaped '' inside a string literal: consume both and stay in string.
			if inString && i+1 < len(sql) && sql[i+1] == '\'' {
				b.WriteByte('\'')
				b.WriteByte('\'')
				i++
				continue
			}
			inString = !inString
			b.WriteByte(c)
		case '?':
			if inString {
				b.WriteByte(c)
				continue
			}
			n++
			b.WriteByte('$')
			b.WriteString(strconv.Itoa(n))
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// InPlaceholders returns a comma-separated list of numbered placeholders
// suitable for an SQL IN clause: `$1,$2,...,$N` where N = offset+count and
// numbering starts at offset+1.
//
// Example:
//
//	InPlaceholders(3, 2)  // "$3,$4,$5"
//
// Pass offset=0 for a standalone query with no preceding args.
func InPlaceholders(count, offset int) string {
	if count <= 0 {
		return ""
	}
	var b strings.Builder
	b.Grow(count * 4)
	for i := 0; i < count; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('$')
		b.WriteString(strconv.Itoa(offset + i + 1))
	}
	return b.String()
}
