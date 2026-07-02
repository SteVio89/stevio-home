package queries

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/dbutil"
)

var ErrDiscountNotFound = errors.New("discount code not found or invalid")

// GetAppPriceCents returns the price in cents for an app by ID.
func GetAppPriceCents(ctx context.Context, db *sql.DB, appID string) (int, error) {
	var price int
	err := db.QueryRowContext(ctx, `SELECT price_cents FROM apps WHERE id = $1`, appID).Scan(&price)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrDiscountNotFound
	}
	return price, err
}

// ApplyDiscount computes the final price after applying a discount to originalCents.
func ApplyDiscount(discountType string, discountValue, originalCents int) int {
	var reduction int
	switch discountType {
	case "percent":
		reduction = originalCents * discountValue / 100
	case "fixed":
		reduction = discountValue
	}
	final := originalCents - reduction
	if final < 0 {
		return 0
	}
	return final
}

// discountCols is used in single-table queries (no alias).
const discountCols = `id, code, label, discount_type, discount_value, app_id, max_uses, uses, expires_at, active, stackable, created_at, deleted_at`

// discountColsAliased is used in JOIN queries where the table is aliased as dc.
const discountColsAliased = `dc.id, dc.code, dc.label, dc.discount_type, dc.discount_value, dc.app_id, dc.max_uses, dc.uses, dc.expires_at, dc.active, dc.stackable, dc.created_at, dc.deleted_at`

func scanDiscountCode(scan func(...any) error) (*models.DiscountCode, error) {
	var d models.DiscountCode
	var expiresAt, deletedAt sql.NullTime
	if err := scan(&d.ID, &d.Code, &d.Label, &d.DiscountType, &d.DiscountValue,
		&d.AppID, &d.MaxUses, &d.Uses, &expiresAt, &d.Active, &d.Stackable, &d.CreatedAt, &deletedAt); err != nil {
		return nil, err
	}
	if expiresAt.Valid {
		d.ExpiresAt = &expiresAt.Time
	}
	if deletedAt.Valid {
		d.DeletedAt = &deletedAt.Time
	}
	return &d, nil
}

// ListDiscountCodes returns all discount codes with per-code order stats (count + revenue)
// derived from a LEFT JOIN with the orders table.
func ListDiscountCodes(ctx context.Context, db *sql.DB) ([]models.DiscountCode, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT `+discountColsAliased+`,
		       COUNT(o.id)                             AS order_count,
		       COALESCE(SUM(o.price_paid_cents), 0)    AS revenue_cents
		FROM discount_codes dc
		LEFT JOIN orders o ON o.discount_code_id = dc.id
		GROUP BY dc.id
		ORDER BY dc.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []models.DiscountCode{}
	for rows.Next() {
		var orderCount int
		var revenueCents int
		d, err := scanDiscountCode(func(dest ...any) error {
			// Append the two extra columns (order_count, revenue_cents) to the scan list.
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

type InsertDiscountCodeParams struct {
	Code          string
	Label         string
	DiscountType  string
	DiscountValue int
	AppID         *string
	MaxUses       *int
	ExpiresAt     *time.Time
	Stackable     bool
}

func InsertDiscountCode(ctx context.Context, db *sql.DB, p InsertDiscountCodeParams) (*models.DiscountCode, error) {
	id := dbutil.NewID()
	row := db.QueryRowContext(ctx,
		`INSERT INTO discount_codes (id, code, label, discount_type, discount_value, app_id, max_uses, expires_at, stackable)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING `+discountCols,
		id, p.Code, p.Label, p.DiscountType, p.DiscountValue, p.AppID, p.MaxUses, p.ExpiresAt, p.Stackable)
	return scanDiscountCode(row.Scan)
}

type UpdateDiscountCodeParams struct {
	Label         string
	DiscountType  string
	DiscountValue int
	AppID         *string
	MaxUses       *int
	ExpiresAt     *time.Time
	Active        bool
	Stackable     bool
}

func UpdateDiscountCode(ctx context.Context, db *sql.DB, id string, p UpdateDiscountCodeParams) (*models.DiscountCode, error) {
	row := db.QueryRowContext(ctx,
		`UPDATE discount_codes
		 SET label=$1, discount_type=$2, discount_value=$3, app_id=$4, max_uses=$5, expires_at=$6, active=$7, stackable=$8
		 WHERE id=$9 AND deleted_at IS NULL
		 RETURNING `+discountCols,
		p.Label, p.DiscountType, p.DiscountValue, p.AppID, p.MaxUses, p.ExpiresAt, p.Active, p.Stackable, id)
	d, err := scanDiscountCode(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDiscountNotFound
	}
	return d, err
}

// SoftDeleteDiscountCode sets deleted_at on a discount code (idempotent).
func SoftDeleteDiscountCode(ctx context.Context, db *sql.DB, id string) error {
	res, err := db.ExecContext(ctx,
		`UPDATE discount_codes SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		var exists int
		if err := db.QueryRowContext(ctx, `SELECT 1 FROM discount_codes WHERE id = $1`, id).Scan(&exists); err != nil {
			return ErrDiscountNotFound
		}
		return nil // already soft-deleted, idempotent
	}
	return nil
}

// RestoreDiscountCode clears deleted_at on a soft-deleted discount code.
func RestoreDiscountCode(ctx context.Context, db *sql.DB, id string) error {
	res, err := db.ExecContext(ctx, `UPDATE discount_codes SET deleted_at = NULL WHERE id = $1`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrDiscountNotFound
	}
	return nil
}

// ValidateDiscountCode checks whether a code is active, non-deleted, non-expired,
// within its usage limit, and scoped to the given appID. Returns the discount row
// if valid. A max_uses of 0 means unlimited. appID must be the app being purchased
// — the code is valid if it is store-wide (app_id IS NULL) or scoped to that exact
// app. An exhausted code returns ErrDiscountNotFound so checkout rejects it up front.
func ValidateDiscountCode(ctx context.Context, db *sql.DB, code, appID string) (*models.DiscountCode, error) {
	row := db.QueryRowContext(ctx,
		`SELECT `+discountCols+`
		 FROM discount_codes
		 WHERE code = $1
		   AND active = TRUE
		   AND deleted_at IS NULL
		   AND (expires_at IS NULL OR expires_at > NOW())
		   AND (max_uses = 0 OR uses < max_uses)
		   AND (app_id IS NULL OR app_id = $2)`,
		code, appID)
	d, err := scanDiscountCode(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDiscountNotFound
	}
	return d, err
}

// IncrementDiscountUses increments the usage counter for a discount code.
// Called fire-and-forget after the fulfillment transaction commits (kept outside
// the tx by design — see fulfillOrder's MaxOpenConns=1 note). The max_uses guard
// in the WHERE clause makes the statement self-capping: a single UPDATE is atomic,
// so concurrent increments can never push uses past max_uses (0 = unlimited).
// Failures are logged but do not affect order processing.
func IncrementDiscountUses(ctx context.Context, db *sql.DB, code, appID string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE discount_codes SET uses = uses + 1
		 WHERE code = $1 AND (app_id IS NULL OR app_id = $2)
		   AND (max_uses = 0 OR uses < max_uses)`,
		code, appID)
	return err
}
