package queries

import (
	"context"
	"database/sql"
	"errors"

	"github.com/SteVio89/stevio-home/dbutil"
)

// OrderWithLicense holds the data needed by the success page after checkout.
type OrderWithLicense struct {
	OrderID    string
	AppID      string
	BundleID   string
	AppName    string
	LicenseKey string
}

// GetOrderAndLicenseByPaymentSession returns the order + license + app name
// for a given payment session ID. Returns nil, nil when no order exists yet
// (webhook not yet processed).
//
// For install_plus re-purchases, no new license row is linked to the order.
// In that case, the query falls back to finding the existing license for the
// same app and user (via the order's email hash).
func GetOrderAndLicenseByPaymentSession(ctx context.Context, db *sql.DB, sessionID, defaultLocale string) (*OrderWithLicense, error) {
	row := db.QueryRowContext(ctx, `
		SELECT o.id, o.app_id, COALESCE(a.bundle_id, '') AS bundle_id,
		       COALESCE(tn.value, '') AS name,
		       COALESCE(l.key, fl.key, '') AS license_key
		FROM orders o
		LEFT JOIN apps a ON a.id = o.app_id
		LEFT JOIN licenses l ON l.order_id = o.id
		LEFT JOIN entity_translations tn ON tn.entity_type = 'project' AND tn.entity_id = a.project_id AND tn.field = 'title' AND tn.locale = $1
		LEFT JOIN licenses fl ON fl.app_id = o.app_id
		    AND fl.order_id IN (SELECT o2.id FROM orders o2 WHERE o2.email = o.email)
		    AND l.id IS NULL
		WHERE o.payment_session = $2
		LIMIT 1`,
		defaultLocale, sessionID)
	var r OrderWithLicense
	err := row.Scan(&r.OrderID, &r.AppID, &r.BundleID, &r.AppName, &r.LicenseKey)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// NewLicenseKey generates a new UUID v4 string suitable for use as a license key.
func NewLicenseKey() string {
	return dbutil.NewID()
}
