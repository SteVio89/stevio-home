package dbutil

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// pgUniqueViolation is Postgres' SQLSTATE for "unique_violation".
// https://www.postgresql.org/docs/current/errcodes-appendix.html
const pgUniqueViolation = "23505"

// IsUniqueViolation reports whether err is a Postgres UNIQUE constraint
// violation. Match on the SQLSTATE code rather than the error message, which
// is locale-dependent.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}
