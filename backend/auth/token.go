package auth

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

// hashToken returns the hex-encoded SHA-256 hash of a token.
// Tokens are stored and looked up by hash — never in plaintext.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

var (
	ErrTokenNotFound = errors.New("token not found")
	ErrTokenUsed     = errors.New("token already used")
	ErrTokenExpired  = errors.New("token expired")
)

// InsertAuthToken stores a new magic-link token. Optional TokenOption values
// attach a user_type hint to the token. Callers that pass no options are
// unaffected — user_type defaults to NULL.
func InsertAuthToken(ctx context.Context, db *sql.DB, token, emailHash string, expiresAt time.Time, opts ...TokenOption) error {
	o := &TokenOptions{}
	for _, fn := range opts {
		fn(o)
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO auth_tokens (token, email, expires_at, user_type) VALUES ($1, $2, $3, $4)`,
		hashToken(token), emailHash, expiresAt, o.UserType)
	return err
}

// ConsumeAuthToken atomically validates and marks the token as used via a single
// UPDATE statement. Returns the email hash on success; returns ErrToken*
// sentinels on failure. Use ConsumeAuthTokenFull to also retrieve the user_type.
func ConsumeAuthToken(ctx context.Context, db *sql.DB, token string) (string, error) {
	emailHash, _, err := ConsumeAuthTokenFull(ctx, db, token)
	return emailHash, err
}

// ConsumeAuthTokenFull is like ConsumeAuthToken but also returns the
// optional user_type stored on the token (nil if not set).
func ConsumeAuthTokenFull(ctx context.Context, db *sql.DB, token string) (emailHash string, userType *string, err error) {
	tokenHash := hashToken(token)

	err = db.QueryRowContext(ctx,
		`UPDATE auth_tokens SET used = TRUE
		 WHERE token = $1 AND used = FALSE AND expires_at > NOW()
		 RETURNING email, user_type`,
		tokenHash).Scan(&emailHash, &userType)
	if err == nil {
		return emailHash, userType, nil
	}
	if err != sql.ErrNoRows {
		return "", nil, err
	}

	// Update matched nothing — determine the exact reason for the caller.
	var used bool
	err = db.QueryRowContext(ctx,
		`SELECT used FROM auth_tokens WHERE token = $1`, tokenHash).
		Scan(&used)
	if err == sql.ErrNoRows {
		return "", nil, ErrTokenNotFound
	}
	if err != nil {
		return "", nil, err
	}
	if used {
		return "", nil, ErrTokenUsed
	}
	return "", nil, ErrTokenExpired
}

// HasValidAuthToken returns true if a non-expired, unused token already exists for the given email hash.
func HasValidAuthToken(ctx context.Context, db *sql.DB, emailHash string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM auth_tokens WHERE email = $1 AND used = FALSE AND expires_at > NOW()`,
		emailHash).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// DeleteExpiredAuthTokens removes all used or expired auth tokens.
func DeleteExpiredAuthTokens(ctx context.Context, db *sql.DB) (int64, error) {
	res, err := db.ExecContext(ctx,
		`DELETE FROM auth_tokens WHERE used = TRUE OR expires_at <= NOW()`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
