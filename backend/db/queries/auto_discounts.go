package queries

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/dbutil"
)

var ErrAutoDiscountNotFound = errors.New("no active auto-discount found")

const autoDiscountCols = `id, label, discount_type, discount_value, app_id, valid_from, expires_at, active, created_at, deleted_at`
const autoDiscountColsAliased = `ad.id, ad.label, ad.discount_type, ad.discount_value, ad.app_id, ad.valid_from, ad.expires_at, ad.active, ad.created_at, ad.deleted_at`

func scanAutoDiscount(scan func(...any) error) (*models.AutoDiscount, error) {
	var d models.AutoDiscount
	var validFrom, expiresAt, deletedAt sql.NullTime
	if err := scan(&d.ID, &d.Label, &d.DiscountType, &d.DiscountValue,
		&d.AppID, &validFrom, &expiresAt, &d.Active, &d.CreatedAt, &deletedAt); err != nil {
		return nil, err
	}
	if validFrom.Valid {
		d.ValidFrom = &validFrom.Time
	}
	if expiresAt.Valid {
		d.ExpiresAt = &expiresAt.Time
	}
	if deletedAt.Valid {
		d.DeletedAt = &deletedAt.Time
	}
	return &d, nil
}

// ListAutoDiscounts returns all auto-discounts with per-discount order stats
// derived from a LEFT JOIN with the orders table.
func ListAutoDiscounts(ctx context.Context, db *sql.DB) ([]models.AutoDiscount, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT `+autoDiscountColsAliased+`,
		       COUNT(o.id)                          AS order_count,
		       COALESCE(SUM(o.price_paid_cents), 0) AS revenue_cents
		FROM auto_discounts ad
		LEFT JOIN orders o ON o.auto_discount_id = ad.id
		GROUP BY ad.id
		ORDER BY ad.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []models.AutoDiscount{}
	for rows.Next() {
		var orderCount int
		var revenueCents int
		d, err := scanAutoDiscount(func(dest ...any) error {
			return rows.Scan(append(dest, &orderCount, &revenueCents)...)
		})
		if err != nil {
			return nil, err
		}
		d.OrderCount = orderCount
		d.RevenueCents = revenueCents
		out = append(out, *d)
	}
	return out, rows.Err()
}

type InsertAutoDiscountParams struct {
	Label         string
	DiscountType  string
	DiscountValue int
	AppID         *string
	ValidFrom     *time.Time
	ExpiresAt     *time.Time
}

func InsertAutoDiscount(ctx context.Context, db *sql.DB, p InsertAutoDiscountParams) (*models.AutoDiscount, error) {
	id := dbutil.NewID()
	row := db.QueryRowContext(ctx,
		`INSERT INTO auto_discounts (id, label, discount_type, discount_value, app_id, valid_from, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING `+autoDiscountCols,
		id, p.Label, p.DiscountType, p.DiscountValue, p.AppID, p.ValidFrom, p.ExpiresAt)
	return scanAutoDiscount(row.Scan)
}

type UpdateAutoDiscountParams struct {
	Label         string
	DiscountType  string
	DiscountValue int
	AppID         *string
	ValidFrom     *time.Time
	ExpiresAt     *time.Time
	Active        bool
}

func UpdateAutoDiscount(ctx context.Context, db *sql.DB, id string, p UpdateAutoDiscountParams) (*models.AutoDiscount, error) {
	row := db.QueryRowContext(ctx,
		`UPDATE auto_discounts
		 SET label=$1, discount_type=$2, discount_value=$3, app_id=$4, valid_from=$5, expires_at=$6, active=$7
		 WHERE id=$8 AND deleted_at IS NULL
		 RETURNING `+autoDiscountCols,
		p.Label, p.DiscountType, p.DiscountValue, p.AppID, p.ValidFrom, p.ExpiresAt, p.Active, id)
	d, err := scanAutoDiscount(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAutoDiscountNotFound
	}
	return d, err
}

// SoftDeleteAutoDiscount sets deleted_at on an auto-discount (idempotent).
func SoftDeleteAutoDiscount(ctx context.Context, db *sql.DB, id string) error {
	res, err := db.ExecContext(ctx,
		`UPDATE auto_discounts SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var exists int
		if err := db.QueryRowContext(ctx, `SELECT 1 FROM auto_discounts WHERE id = $1`, id).Scan(&exists); err != nil {
			return ErrAutoDiscountNotFound
		}
		return nil
	}
	return nil
}

// RestoreAutoDiscount clears deleted_at on a soft-deleted auto-discount.
func RestoreAutoDiscount(ctx context.Context, db *sql.DB, id string) error {
	res, err := db.ExecContext(ctx, `UPDATE auto_discounts SET deleted_at = NULL WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrAutoDiscountNotFound
	}
	return nil
}

// GetAllActiveAutoDiscounts returns all currently active auto-discounts (used to batch-apply
// discounts to the app listing). Callers should pick the best match per app: prefer app-specific
// (AppID != nil) over store-wide (AppID == nil).
func GetAllActiveAutoDiscounts(ctx context.Context, db *sql.DB) ([]models.AutoDiscount, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT `+autoDiscountCols+`
		 FROM auto_discounts
		 WHERE active = TRUE
		   AND deleted_at IS NULL
		   AND (valid_from IS NULL OR valid_from <= NOW())
		   AND (expires_at IS NULL OR expires_at > NOW())`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []models.AutoDiscount{}
	for rows.Next() {
		d, err := scanAutoDiscount(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

// GetActiveAutoDiscount returns the best active auto-discount for the given appID.
// App-specific discounts take precedence over store-wide ones.
// Returns ErrAutoDiscountNotFound if none is currently active.
func GetActiveAutoDiscount(ctx context.Context, db *sql.DB, appID string) (*models.AutoDiscount, error) {
	row := db.QueryRowContext(ctx,
		`SELECT `+autoDiscountCols+`
		 FROM auto_discounts
		 WHERE active = TRUE
		   AND deleted_at IS NULL
		   AND (app_id IS NULL OR app_id = $1)
		   AND (valid_from IS NULL OR valid_from <= NOW())
		   AND (expires_at IS NULL OR expires_at > NOW())
		 ORDER BY (app_id IS NOT NULL) DESC
		 LIMIT 1`,
		appID)
	d, err := scanAutoDiscount(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAutoDiscountNotFound
	}
	return d, err
}
