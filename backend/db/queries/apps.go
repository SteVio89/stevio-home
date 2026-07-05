package queries

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/dbutil"
)

// ErrInvalidPurchaseMode is returned when a non-empty purchase_mode value
// is not one of the allowed enum values. Empty strings are coalesced to the
// default ("always_new_license") and never produce this error.
var ErrInvalidPurchaseMode = errors.New("queries: invalid purchase_mode")

// Slugify converts a name into a URL-safe slug (lowercase, hyphens, alphanumeric only).
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	var out []byte
	prev := byte('-')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'):
			out = append(out, c)
			prev = c
		case c == ' ' || c == '-' || c == '_':
			if prev != '-' {
				out = append(out, '-')
				prev = '-'
			}
		}
	}
	return strings.Trim(string(out), "-")
}

// scanApp scans the standard 8-column app row:
// id, project_id, bundle_id, price_cents, purchase_mode, tax_category, created_at, deleted_at.
// Display text fields (title/tagline/description/image) live on the parent project.
func scanApp(scan func(...any) error) (models.App, error) {
	var a models.App
	var projectID sql.NullString
	if err := scan(&a.ID, &projectID, &a.BundleID, &a.PriceCents, &a.PurchaseMode, &a.TaxCategory, &a.CreatedAt, &a.DeletedAt); err != nil {
		return a, err
	}
	if projectID.Valid {
		a.ProjectID = projectID.String
	}
	return a, nil
}

// GetAppByID looks up an app by its UUID. Returns (nil, nil) when not found.
// Soft-deleted apps are NOT excluded — caller decides.
func GetAppByID(ctx context.Context, db *sql.DB, id string) (*models.App, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, project_id, bundle_id, price_cents, purchase_mode, tax_category, created_at, deleted_at
		FROM apps WHERE id = $1
		LIMIT 1`, id)
	a, err := scanApp(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// GetAppByProjectID returns the (non-deleted) commerce row attached to a project,
// or (nil, nil) when none exists.
func GetAppByProjectID(ctx context.Context, db *sql.DB, projectID string) (*models.App, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, project_id, bundle_id, price_cents, purchase_mode, tax_category, created_at, deleted_at
		FROM apps WHERE project_id = $1 AND deleted_at IS NULL
		LIMIT 1`, projectID)
	a, err := scanApp(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

// GetAppIDByBundleID resolves a bundle_id to an internal app UUID.
// Returns "", nil when no matching app exists.
func GetAppIDByBundleID(ctx context.Context, db *sql.DB, bundleID string) (string, error) {
	var id string
	err := db.QueryRowContext(ctx,
		`SELECT id FROM apps WHERE bundle_id = $1 AND deleted_at IS NULL`, bundleID).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

// InsertAppTx creates a new commerce app attached to a project, inside a transaction.
// An empty taxCategory is coalesced to the default "standard" (Paddle's
// pre-approved "Standard digital goods" category for downloadable software).
func InsertAppTx(ctx context.Context, tx *sql.Tx, projectID, bundleID string, priceCents int, purchaseMode, taxCategory string) (*models.App, error) {
	if purchaseMode == "" {
		purchaseMode = "always_new_license"
	} else if !IsValidPurchaseMode(purchaseMode) {
		return nil, ErrInvalidPurchaseMode
	}
	if taxCategory == "" {
		taxCategory = "standard"
	} else if !IsValidTaxCategory(taxCategory) {
		return nil, ErrInvalidTaxCategory
	}
	id := dbutil.NewID()
	row := tx.QueryRowContext(ctx, `
		INSERT INTO apps (id, project_id, bundle_id, price_cents, purchase_mode, tax_category)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, project_id, bundle_id, price_cents, purchase_mode, tax_category, created_at, deleted_at`,
		id, projectID, bundleID, priceCents, purchaseMode, taxCategory)
	a, err := scanApp(row.Scan)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// UpdateApp updates pricing/purchase_mode/tax_category on a commerce app. project_id
// and bundle_id are immutable after create — change them via re-create + soft-delete.
// An empty taxCategory is coalesced to the default "standard" (Paddle's
// pre-approved "Standard digital goods" category for downloadable software).
func UpdateApp(ctx context.Context, db *sql.DB, id string, priceCents int, purchaseMode, taxCategory string) error {
	if purchaseMode == "" {
		purchaseMode = "always_new_license"
	} else if !IsValidPurchaseMode(purchaseMode) {
		return ErrInvalidPurchaseMode
	}
	if taxCategory == "" {
		taxCategory = "standard"
	} else if !IsValidTaxCategory(taxCategory) {
		return ErrInvalidTaxCategory
	}
	_, err := db.ExecContext(ctx,
		`UPDATE apps SET price_cents=$1, purchase_mode=$2, tax_category=$3 WHERE id=$4`,
		priceCents, purchaseMode, taxCategory, id)
	return err
}

// IsValidPurchaseMode checks if the given purchase mode is one of the allowed values.
func IsValidPurchaseMode(m string) bool {
	return m == "always_new_license" || m == "one_time_only" || m == "install_plus" || m == "coming_soon"
}

// ErrInvalidTaxCategory is returned when a non-empty tax_category value is not
// one of Paddle's supported categories. Empty strings are coalesced to the
// default ("digital-goods") and never produce this error.
var ErrInvalidTaxCategory = errors.New("queries: invalid tax_category")

// IsValidTaxCategory checks whether the given value is one of Paddle's nine
// supported tax categories. Callers should treat "" as the default
// "digital-goods" before calling this — an empty string is NOT valid here.
//
// See https://developer.paddle.com/api-reference/about/tax-categories
func IsValidTaxCategory(c string) bool {
	switch c {
	case "digital-goods",
		"ebooks",
		"implementation-services",
		"professional-services",
		"saas",
		"software-programming-services",
		"standard",
		"training-services",
		"website-hosting":
		return true
	}
	return false
}

func GetLatestVersion(ctx context.Context, db *sql.DB, appID string) (*models.AppVersion, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, app_id, version, download_url, file_path, published_at
		FROM app_versions
		WHERE app_id = $1
		ORDER BY published_at DESC
		LIMIT 1`, appID)
	var v models.AppVersion
	err := row.Scan(&v.ID, &v.AppID, &v.Version, &v.DownloadURL, &v.FilePath, &v.PublishedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &v, nil
}

// AdminAppListItem joins an app with its parent project for the admin table view.
type AdminAppListItem struct {
	models.App
	ProjectSlug  string `json:"project_slug"`
	ProjectTitle string `json:"project_title,omitempty"`
}

// ListAppsAdmin returns all commerce apps (including soft-deleted), each annotated
// with its parent project's slug. Title is left to the handler to populate from
// translations (entity_type='project').
func ListAppsAdmin(ctx context.Context, db *sql.DB) ([]AdminAppListItem, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT a.id, a.project_id, a.bundle_id, a.price_cents, a.purchase_mode, a.tax_category, a.created_at, a.deleted_at,
		       COALESCE(p.slug, '')
		FROM apps a
		LEFT JOIN projects p ON p.id = a.project_id
		ORDER BY a.created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []AdminAppListItem{}
	for rows.Next() {
		var item AdminAppListItem
		var projectID sql.NullString
		if err := rows.Scan(&item.ID, &projectID, &item.BundleID, &item.PriceCents, &item.PurchaseMode,
			&item.TaxCategory, &item.CreatedAt, &item.DeletedAt, &item.ProjectSlug); err != nil {
			return nil, err
		}
		if projectID.Valid {
			item.ProjectID = projectID.String
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// SoftDeleteApp sets deleted_at on an app (idempotent — only updates if not already deleted).
func SoftDeleteApp(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE apps SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	return err
}

// RestoreApp clears deleted_at on a soft-deleted app.
func RestoreApp(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx, `UPDATE apps SET deleted_at = NULL WHERE id = $1`, id)
	return err
}

// queryRower is implemented by both *sql.DB and *sql.Tx — used by other query
// helpers in this package.
type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}
