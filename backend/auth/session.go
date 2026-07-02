package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrSessionNotFound is returned when no session exists for a given ID.
var ErrSessionNotFound = errors.New("session not found")

// Session represents an authenticated user session.
type Session struct {
	ID        string    `json:"id"`
	EmailHash string    `json:"email_hash"`
	UserID    *string   `json:"user_id,omitempty"`
	UserType  *string   `json:"user_type,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CreateSession inserts a new session. Optional SessionOption values attach
// user_id and user_type to the session. Callers that pass no options are
// unaffected — both columns default to NULL.
func CreateSession(ctx context.Context, db *sql.DB, id, emailHash string, expiresAt time.Time, opts ...SessionOption) error {
	o := &SessionOptions{}
	for _, fn := range opts {
		fn(o)
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO sessions (id, email, expires_at, user_id, user_type) VALUES ($1, $2, $3, $4, $5)`,
		id, emailHash, expiresAt, o.UserID, o.UserType)
	return err
}

// GetSession returns the session for the given ID.
// Returns ErrSessionNotFound if no session exists or if the session has expired.
func GetSession(ctx context.Context, db *sql.DB, id string) (*Session, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, email, created_at, expires_at, user_id, user_type FROM sessions WHERE id = $1 AND expires_at > NOW()`, id)
	var s Session
	err := row.Scan(&s.ID, &s.EmailHash, &s.CreatedAt, &s.ExpiresAt, &s.UserID, &s.UserType)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return &s, nil
}

// DeleteSession removes a session by ID.
func DeleteSession(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, id)
	return err
}

// DeleteSessionsByEmailHash removes all sessions for the given email hash.
func DeleteSessionsByEmailHash(ctx context.Context, db *sql.DB, emailHash string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE email = $1`, emailHash)
	return err
}

// DeleteExpiredSessions removes all sessions whose expiry has passed.
func DeleteExpiredSessions(ctx context.Context, db *sql.DB) (int64, error) {
	res, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= NOW()`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
