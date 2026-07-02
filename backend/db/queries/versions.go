package queries

import (
	"context"
	"database/sql"

	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/dbutil"
)

func InsertAppVersion(ctx context.Context, db *sql.DB, appID, version, downloadURL, filePath string) (*models.AppVersion, error) {
	return insertAppVersion(ctx, db, appID, version, downloadURL, filePath)
}

// InsertAppVersionTx creates a version inside an existing transaction.
func InsertAppVersionTx(ctx context.Context, tx *sql.Tx, appID, version, downloadURL, filePath string) (*models.AppVersion, error) {
	return insertAppVersion(ctx, tx, appID, version, downloadURL, filePath)
}

// queryRower is defined in apps.go — both *sql.DB and *sql.Tx implement it.

func insertAppVersion(ctx context.Context, q queryRower, appID, version, downloadURL, filePath string) (*models.AppVersion, error) {
	id := dbutil.NewID()
	row := q.QueryRowContext(ctx, `
		INSERT INTO app_versions (id, app_id, version, download_url, file_path)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, app_id, version, download_url, file_path, published_at`,
		id, appID, version, downloadURL, filePath)
	var v models.AppVersion
	if err := row.Scan(&v.ID, &v.AppID, &v.Version, &v.DownloadURL, &v.FilePath, &v.PublishedAt); err != nil {
		return nil, err
	}
	return &v, nil
}

func UpdateAppVersionFilePath(ctx context.Context, db *sql.DB, versionID, filePath, downloadURL string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE app_versions SET file_path=$1, download_url=$2 WHERE id=$3`,
		filePath, downloadURL, versionID)
	return err
}

func ListVersionsByAppID(ctx context.Context, db *sql.DB, appID string) ([]models.AppVersion, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, app_id, version, download_url, file_path, published_at
		FROM app_versions WHERE app_id=$1 ORDER BY published_at DESC`, appID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []models.AppVersion{}
	for rows.Next() {
		var v models.AppVersion
		if err := rows.Scan(&v.ID, &v.AppID, &v.Version, &v.DownloadURL, &v.FilePath, &v.PublishedAt); err != nil {
			return nil, err
		}
		// ReleaseNotes is populated by entity_translations overlay in the handler,
		// not by a DB column (translation-first architecture).
		out = append(out, v)
	}
	return out, rows.Err()
}
