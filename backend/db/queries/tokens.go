package queries

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var (
	ErrDownloadTokenUsed     = errors.New("download token already used")
	ErrDownloadTokenExpired  = errors.New("download token has expired")
	ErrDownloadTokenNotFound = errors.New("download token not found")
)

func InsertDownloadToken(ctx context.Context, db *sql.DB, token, licenseID, appID string, expiresAt time.Time) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO download_tokens (token, license_id, app_id, expires_at) VALUES ($1, $2, $3, $4)`,
		token, licenseID, appID, expiresAt)
	return err
}

// ConsumeDownloadToken atomically marks a token as used and returns the app_id.
// Returns ErrDownloadTokenUsed, ErrDownloadTokenExpired, or ErrDownloadTokenNotFound on failure.
func ConsumeDownloadToken(ctx context.Context, db *sql.DB, token string) (appID string, err error) {
	err = db.QueryRowContext(ctx,
		`UPDATE download_tokens SET used = TRUE
		 WHERE token=$1 AND used = FALSE AND expires_at>NOW()
		 RETURNING app_id`,
		token).Scan(&appID)
	if err == nil {
		return appID, nil
	}
	if err != sql.ErrNoRows {
		return "", err
	}

	// Determine specific failure reason
	var used bool
	err = db.QueryRowContext(ctx,
		`SELECT used FROM download_tokens WHERE token=$1`, token).
		Scan(&used)
	if err == sql.ErrNoRows {
		return "", ErrDownloadTokenNotFound
	}
	if err != nil {
		return "", err
	}
	if used {
		return "", ErrDownloadTokenUsed
	}
	return "", ErrDownloadTokenExpired
}
