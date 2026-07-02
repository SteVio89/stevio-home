package queries

import (
	"context"
	"database/sql"

	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/dbutil"
)

// projectColumns is the canonical column list for project SELECTs (8 cols).
const projectColumns = `id, slug, image_url, external_url, position, has_detail_page, created_at, deleted_at`

// scanProject scans the standard 8-column project row in the order from
// projectColumns. external_url uses sql.NullString to keep the *string semantics
// (empty string and NULL both map to nil pointer).
func scanProject(scan func(...any) error) (models.Project, error) {
	var p models.Project
	var externalURL sql.NullString
	if err := scan(&p.ID, &p.Slug, &p.ImageURL, &externalURL, &p.Position, &p.HasDetailPage, &p.CreatedAt, &p.DeletedAt); err != nil {
		return p, err
	}
	if externalURL.Valid && externalURL.String != "" {
		s := externalURL.String
		p.ExternalURL = &s
	}
	return p, nil
}

// InsertProjectTx creates a new project inside a transaction.
// slug may be empty (the partial unique index allows multiple rows with ”).
// externalURL nil/empty leaves the column at its ” default. hasDetailPage is
// the admin-controlled toggle; commerce projects override this at the handler level.
func InsertProjectTx(ctx context.Context, tx *sql.Tx, slug, imageURL string, externalURL *string, position int, hasDetailPage bool) (models.Project, error) {
	id := dbutil.NewID()
	externalURLVal := ""
	if externalURL != nil {
		externalURLVal = *externalURL
	}
	row := tx.QueryRowContext(ctx, `
		INSERT INTO projects (id, slug, image_url, external_url, position, has_detail_page)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+projectColumns,
		id, slug, imageURL, externalURLVal, position, hasDetailPage)
	return scanProject(row.Scan)
}

// GetProjectByID looks up a project by UUID. Returns (nil, nil) when not found.
// No soft-delete filter — admin needs to see deleted projects too.
func GetProjectByID(ctx context.Context, db *sql.DB, id string) (*models.Project, error) {
	row := db.QueryRowContext(ctx, `SELECT `+projectColumns+` FROM projects WHERE id = $1`, id)
	p, err := scanProject(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

// GetProjectBySlug looks up a non-deleted project by slug. Returns (nil, nil)
// when not found. Empty slugs are never returned (partial unique index).
func GetProjectBySlug(ctx context.Context, db *sql.DB, slug string) (*models.Project, error) {
	if slug == "" {
		return nil, nil
	}
	row := db.QueryRowContext(ctx,
		`SELECT `+projectColumns+` FROM projects WHERE slug = $1 AND deleted_at IS NULL`, slug)
	p, err := scanProject(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

// ListProjectsPublic returns non-deleted projects ordered by position ASC,
// each with an attached commerce row (if any). Translation overlay is done at
// the handler level.
func ListProjectsPublic(ctx context.Context, db *sql.DB) ([]models.Project, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT p.id, p.slug, p.image_url, p.external_url, p.position, p.has_detail_page,
		       p.created_at, p.deleted_at,
		       a.id, a.bundle_id, a.price_cents, a.purchase_mode, a.created_at, a.deleted_at
		FROM projects p
		LEFT JOIN apps a ON a.project_id = p.id AND a.deleted_at IS NULL
		WHERE p.deleted_at IS NULL
		ORDER BY p.position ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []models.Project{}
	for rows.Next() {
		p, a, err := scanProjectWithCommerce(rows)
		if err != nil {
			return nil, err
		}
		if a != nil {
			p.Commerce = a
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ListProjectsAdmin returns all projects (including soft-deleted), each annotated
// with its commerce row (if any). Used by the admin dashboard.
func ListProjectsAdmin(ctx context.Context, db *sql.DB) ([]models.Project, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT p.id, p.slug, p.image_url, p.external_url, p.position, p.has_detail_page,
		       p.created_at, p.deleted_at,
		       a.id, a.bundle_id, a.price_cents, a.purchase_mode, a.created_at, a.deleted_at
		FROM projects p
		LEFT JOIN apps a ON a.project_id = p.id AND a.deleted_at IS NULL
		ORDER BY p.position ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []models.Project{}
	for rows.Next() {
		p, a, err := scanProjectWithCommerce(rows)
		if err != nil {
			return nil, err
		}
		if a != nil {
			p.Commerce = a
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// scanProjectWithCommerce scans a row that includes the project columns plus
// the LEFT-joined app columns (a.id, a.bundle_id, a.price_cents, a.purchase_mode,
// a.created_at, a.deleted_at). Commerce is nil when the LEFT JOIN matched nothing.
func scanProjectWithCommerce(rows *sql.Rows) (models.Project, *models.App, error) {
	var p models.Project
	var externalURL sql.NullString
	var appID, bundleID, purchaseMode sql.NullString
	var priceCents sql.NullInt64
	var appCreatedAt, appDeletedAt sql.NullTime

	if err := rows.Scan(
		&p.ID, &p.Slug, &p.ImageURL, &externalURL, &p.Position, &p.HasDetailPage,
		&p.CreatedAt, &p.DeletedAt,
		&appID, &bundleID, &priceCents, &purchaseMode, &appCreatedAt, &appDeletedAt,
	); err != nil {
		return p, nil, err
	}
	if externalURL.Valid && externalURL.String != "" {
		s := externalURL.String
		p.ExternalURL = &s
	}
	var commerce *models.App
	if appID.Valid {
		a := &models.App{
			ID:           appID.String,
			ProjectID:    p.ID,
			BundleID:     bundleID.String,
			PriceCents:   int(priceCents.Int64),
			PurchaseMode: purchaseMode.String,
		}
		if appCreatedAt.Valid {
			a.CreatedAt = appCreatedAt.Time
		}
		if appDeletedAt.Valid {
			t := appDeletedAt.Time
			a.DeletedAt = &t
		}
		commerce = a
	}
	return p, commerce, nil
}

// UpdateProjectTx updates a project's editable fields within a transaction.
// Only updates non-deleted projects.
func UpdateProjectTx(ctx context.Context, tx *sql.Tx, id, slug, imageURL string, externalURL *string, hasDetailPage bool) error {
	externalURLVal := ""
	if externalURL != nil {
		externalURLVal = *externalURL
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE projects
		   SET slug = $1, image_url = $2, external_url = $3, has_detail_page = $4
		 WHERE id = $5 AND deleted_at IS NULL`,
		slug, imageURL, externalURLVal, hasDetailPage, id)
	return err
}

// SoftDeleteProject sets deleted_at on a non-deleted project.
func SoftDeleteProject(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE projects SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`, id)
	return err
}

// RestoreProject clears deleted_at on a soft-deleted project.
func RestoreProject(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE projects SET deleted_at = NULL
		WHERE id = $1 AND deleted_at IS NOT NULL`, id)
	return err
}

// UpdateProjectPosition sets the position of a project within a transaction.
func UpdateProjectPosition(ctx context.Context, tx *sql.Tx, id string, position int) error {
	_, err := tx.ExecContext(ctx, `UPDATE projects SET position = $1 WHERE id = $2`, position, id)
	return err
}

// UpdateProjectImageURL updates only the image_url of a non-deleted project.
func UpdateProjectImageURL(ctx context.Context, db *sql.DB, id, imageURL string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE projects SET image_url = $1
		WHERE id = $2 AND deleted_at IS NULL`, imageURL, id)
	return err
}

// MaxProjectPosition returns the current maximum position among non-deleted projects.
// Returns -1 when no projects exist (so next position = 0).
func MaxProjectPosition(ctx context.Context, db *sql.DB) (int, error) {
	var maxPos int
	err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), -1) FROM projects WHERE deleted_at IS NULL`).Scan(&maxPos)
	return maxPos, err
}

// ── Project images (gallery) ────────────────────────────────────────────────

// InsertProjectImageTx creates a project_images row inside an existing transaction.
func InsertProjectImageTx(ctx context.Context, tx *sql.Tx, projectID, url, filePath string, position int) (*models.ProjectImage, error) {
	id := dbutil.NewID()
	row := tx.QueryRowContext(ctx, `
		INSERT INTO project_images (id, project_id, url, file_path, position)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, project_id, url, file_path, position, created_at`,
		id, projectID, url, filePath, position)
	var img models.ProjectImage
	if err := row.Scan(&img.ID, &img.ProjectID, &img.URL, &img.FilePath, &img.Position, &img.CreatedAt); err != nil {
		return nil, err
	}
	return &img, nil
}

// ListProjectImages returns all images attached to a project, ordered by position ASC.
func ListProjectImages(ctx context.Context, db *sql.DB, projectID string) ([]models.ProjectImage, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, project_id, url, file_path, position, created_at
		FROM project_images WHERE project_id = $1 ORDER BY position ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []models.ProjectImage{}
	for rows.Next() {
		var img models.ProjectImage
		if err := rows.Scan(&img.ID, &img.ProjectID, &img.URL, &img.FilePath, &img.Position, &img.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, img)
	}
	return out, rows.Err()
}

// GetProjectImageMaxPosition returns the max position among images of a project.
// Returns -1 when no images exist (so next position = 0).
func GetProjectImageMaxPosition(ctx context.Context, db *sql.DB, projectID string) (int, error) {
	var pos int
	err := db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(position), -1) FROM project_images WHERE project_id = $1`, projectID).Scan(&pos)
	return pos, err
}

// UpdateProjectImagePositionForProject updates an image's position only when
// it belongs to the given project. Returns sql.ErrNoRows when the image is not
// found / not owned.
func UpdateProjectImagePositionForProject(ctx context.Context, tx *sql.Tx, id, projectID string, position int) error {
	res, err := tx.ExecContext(ctx,
		`UPDATE project_images SET position=$1 WHERE id=$2 AND project_id=$3`,
		position, id, projectID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DeleteProjectImageForProject removes an image only when it belongs to the
// given project. Returns the file_path of the deleted row (so callers can unlink
// the file on disk), or "" when nothing was deleted.
func DeleteProjectImageForProject(ctx context.Context, db *sql.DB, id, projectID string) (filePath string, err error) {
	err = db.QueryRowContext(ctx,
		`DELETE FROM project_images WHERE id=$1 AND project_id=$2 RETURNING file_path`,
		id, projectID).Scan(&filePath)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return filePath, err
}
