package queries

import (
	"context"
	"database/sql"
	"time"

	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/dbutil"
)

// scanSocialLink scans 5 columns from a social_links row:
// id, platform, url, position, created_at
func scanSocialLink(scan func(...any) error) (models.SocialLink, error) {
	var s models.SocialLink
	if err := scan(&s.ID, &s.Platform, &s.URL, &s.Position, &s.CreatedAt); err != nil {
		return s, err
	}
	return s, nil
}

// InsertSocialLink creates a new social link and returns it.
func InsertSocialLink(ctx context.Context, db *sql.DB, platform, url string, position int) (models.SocialLink, error) {
	id := dbutil.NewID()
	now := time.Now().UTC()
	_, err := db.ExecContext(ctx, `
		INSERT INTO social_links (id, platform, url, position)
		VALUES ($1, $2, $3, $4)`,
		id, platform, url, position)
	if err != nil {
		return models.SocialLink{}, err
	}
	return models.SocialLink{
		ID:        id,
		Platform:  platform,
		URL:       url,
		Position:  position,
		CreatedAt: now,
	}, nil
}

// GetSocialLinkByID looks up a social link by UUID. Returns (nil, nil) when not found.
func GetSocialLinkByID(ctx context.Context, db *sql.DB, id string) (*models.SocialLink, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, platform, url, position, created_at
		FROM social_links WHERE id = $1`, id)
	s, err := scanSocialLink(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// ListSocialLinksPublic returns all social links ordered by position ASC.
func ListSocialLinksPublic(ctx context.Context, db *sql.DB) ([]models.SocialLink, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, platform, url, position, created_at
		FROM social_links ORDER BY position ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []models.SocialLink{}
	for rows.Next() {
		s, err := scanSocialLink(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListSocialLinksAdmin returns all social links ordered by position ASC.
// Same as public (no soft-delete), kept separate for API clarity.
func ListSocialLinksAdmin(ctx context.Context, db *sql.DB) ([]models.SocialLink, error) {
	return ListSocialLinksPublic(ctx, db)
}

// UpdateSocialLink updates a social link's platform and URL.
func UpdateSocialLink(ctx context.Context, db *sql.DB, id, platform, url string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE social_links SET platform = $1, url = $2
		WHERE id = $3`, platform, url, id)
	return err
}

// DeleteSocialLink hard-deletes a social link (no soft-delete per D-09).
func DeleteSocialLink(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM social_links WHERE id = $1`, id)
	return err
}

// UpdateSocialLinkPosition sets the position of a social link within a transaction.
func UpdateSocialLinkPosition(ctx context.Context, tx *sql.Tx, id string, position int) error {
	_, err := tx.ExecContext(ctx, `UPDATE social_links SET position = $1 WHERE id = $2`, position, id)
	return err
}

// MaxSocialLinkPosition returns the current maximum position among social links.
// Returns -1 when no social links exist (so next position = 0).
func MaxSocialLinkPosition(ctx context.Context, db *sql.DB) (int, error) {
	var max int
	err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(position), -1) FROM social_links`).Scan(&max)
	return max, err
}
