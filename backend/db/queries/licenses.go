package queries

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/dbutil"
)

// LicenseListFilter controls pagination and optional filtering for admin license listing.
type LicenseListFilter struct {
	Page      int
	PerPage   int
	AppID     string
	KeyPrefix string
}

// LicenseListItem represents a single license row in the admin list.
type LicenseListItem struct {
	ID              string    `json:"id"`
	Key             string    `json:"key"`
	OrderID         string    `json:"order_id"`
	AppID           string    `json:"app_id"`
	AppName         string    `json:"app_name"`
	Revoked         bool      `json:"revoked"`
	MaxActivations  *int      `json:"max_activations,omitempty"`
	ActivationCount int       `json:"activation_count"`
	CreatedAt       time.Time `json:"created_at"`
}

// LicenseListResult is the paginated response for admin license listing.
type LicenseListResult struct {
	Licenses []LicenseListItem `json:"licenses"`
	Total    int               `json:"total"`
}

// ListAllLicenses returns a paginated, optionally filtered list of all licenses for admin use.
func ListAllLicenses(ctx context.Context, db *sql.DB, defaultLocale string, filter LicenseListFilter) (*LicenseListResult, error) {
	filter.Page, filter.PerPage = clampPagination(filter.Page, filter.PerPage)

	var where []string
	var args []any

	if filter.AppID != "" {
		where = append(where, "l.app_id = ?")
		args = append(args, filter.AppID)
	}
	if filter.KeyPrefix != "" {
		where = append(where, "l.key LIKE ?")
		args = append(args, filter.KeyPrefix+"%")
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	// Count and data queries use `?` placeholders; NumberPlaceholders rewrites
	// them to $1..$N matching args positionally. See orders.ListAllOrders for
	// the same pattern.
	countQuery := dbutil.NumberPlaceholders(
		fmt.Sprintf(`SELECT COUNT(*) FROM licenses l %s`, whereClause), 0)
	var total int
	if err := db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("ListAllLicenses count: %w", err)
	}

	offset := (filter.Page - 1) * filter.PerPage

	// activation_count is computed once via a grouped LEFT JOIN rather than
	// a per-row correlated subquery, so the page cost is constant in the
	// number of rows returned (instead of PerPage + 1 trips to activations).
	dataQuery := dbutil.NumberPlaceholders(fmt.Sprintf(`
		SELECT l.id, l.key, l.order_id, l.app_id, COALESCE(tn.value, ''),
		       l.revoked, l.max_activations,
		       COALESCE(act.activation_count, 0) AS activation_count,
		       l.created_at
		FROM licenses l
		LEFT JOIN apps a ON a.id = l.app_id
		LEFT JOIN entity_translations tn ON tn.entity_type = 'project' AND tn.entity_id = a.project_id AND tn.field = 'title' AND tn.locale = ?
		LEFT JOIN (
		    SELECT license_id, COUNT(*) AS activation_count
		    FROM activations
		    GROUP BY license_id
		) act ON act.license_id = l.id
		%s
		ORDER BY l.created_at DESC
		LIMIT ? OFFSET ?`, whereClause), 0)

	dataArgs := append([]any{defaultLocale}, args...)
	dataArgs = append(dataArgs, filter.PerPage, offset)
	rows, err := db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("ListAllLicenses query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var licenses []LicenseListItem
	for rows.Next() {
		var item LicenseListItem
		if err := rows.Scan(
			&item.ID, &item.Key, &item.OrderID, &item.AppID, &item.AppName,
			&item.Revoked, &item.MaxActivations, &item.ActivationCount, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("ListAllLicenses scan: %w", err)
		}
		licenses = append(licenses, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListAllLicenses rows: %w", err)
	}
	if licenses == nil {
		licenses = []LicenseListItem{}
	}

	return &LicenseListResult{Licenses: licenses, Total: total}, nil
}

func InsertLicense(ctx context.Context, tx *sql.Tx, key, orderID, appID string, maxActivations *int) (*models.License, error) {
	id := dbutil.NewID()
	row := tx.QueryRowContext(ctx, `
		INSERT INTO licenses (id, key, order_id, app_id, max_activations)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, key, order_id, app_id, revoked, max_activations, created_at`,
		id, key, orderID, appID, maxActivations)
	var l models.License
	if err := row.Scan(&l.ID, &l.Key, &l.OrderID, &l.AppID, &l.Revoked, &l.MaxActivations, &l.CreatedAt); err != nil {
		return nil, err
	}
	return &l, nil
}

// OwnershipStatus captures whether a user already has a license for an app.
type OwnershipStatus struct {
	HasLicense     bool
	LicenseID      string
	MaxActivations *int
}

// GetOwnershipStatus returns ownership facts for a (user, app) pair.
// Returns a zero-value OwnershipStatus (HasLicense=false) when no license exists.
func GetOwnershipStatus(ctx context.Context, db *sql.DB, appID, emailHash string) (OwnershipStatus, error) {
	row := db.QueryRowContext(ctx, `
		SELECT l.id, l.max_activations
		FROM licenses l
		JOIN orders o ON o.id = l.order_id
		WHERE l.app_id = $1 AND o.email = $2
		LIMIT 1`, appID, emailHash)

	var licenseID string
	var maxActivations *int
	err := row.Scan(&licenseID, &maxActivations)
	if err == sql.ErrNoRows {
		return OwnershipStatus{HasLicense: false}, nil
	}
	if err != nil {
		return OwnershipStatus{}, err
	}
	return OwnershipStatus{
		HasLicense:     true,
		LicenseID:      licenseID,
		MaxActivations: maxActivations,
	}, nil
}

// GetLicenseForAppAndEmail returns the existing license for a (app, user) pair
// inside a transaction. Used by fulfillOrder for install_plus mode.
// Returns (nil, nil) when no license exists.
func GetLicenseForAppAndEmail(ctx context.Context, tx *sql.Tx, appID, emailHash, defaultLocale string) (*models.License, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT l.id, l.key, l.order_id, l.app_id, COALESCE(a.bundle_id, '') AS app_bundle_id,
		       COALESCE(tn.value, '') AS app_name,
		       l.revoked, l.max_activations, l.created_at
		FROM licenses l
		JOIN orders o ON o.id = l.order_id
		LEFT JOIN apps a ON a.id = l.app_id
		LEFT JOIN entity_translations tn ON tn.entity_type = 'project' AND tn.entity_id = a.project_id AND tn.field = 'title' AND tn.locale = $1
		WHERE l.app_id = $2 AND o.email = $3
		LIMIT 1`, defaultLocale, appID, emailHash)

	var l models.License
	err := row.Scan(&l.ID, &l.Key, &l.OrderID, &l.AppID, &l.AppBundleID, &l.AppName, &l.Revoked, &l.MaxActivations, &l.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// BumpLicenseMaxActivations adds slotsToAdd to the license's max_activations.
// If max_activations is currently NULL, COALESCE treats it as 0 and adds from there.
func BumpLicenseMaxActivations(ctx context.Context, tx *sql.Tx, licenseID string, slotsToAdd int) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE licenses SET max_activations = COALESCE(max_activations, 0) + $1 WHERE id = $2`,
		slotsToAdd, licenseID)
	return err
}

func GetLicenseByKey(ctx context.Context, db *sql.DB, key, defaultLocale string) (*models.License, error) {
	row := db.QueryRowContext(ctx, `
		SELECT l.id, l.key, l.order_id, l.app_id, COALESCE(a.bundle_id, '') AS app_bundle_id,
		       COALESCE(tn.value, '') AS app_name,
		       l.revoked, l.max_activations, l.created_at
		FROM licenses l
		LEFT JOIN apps a ON a.id = l.app_id
		LEFT JOIN entity_translations tn ON tn.entity_type = 'project' AND tn.entity_id = a.project_id AND tn.field = 'title' AND tn.locale = $1
		WHERE l.key = $2`, defaultLocale, key)
	var l models.License
	err := row.Scan(&l.ID, &l.Key, &l.OrderID, &l.AppID, &l.AppBundleID, &l.AppName, &l.Revoked, &l.MaxActivations, &l.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &l, nil
}

func CountActivations(ctx context.Context, db *sql.DB, licenseID string) (int, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM activations WHERE license_id = $1`, licenseID).Scan(&count)
	return count, err
}

// LockLicenseForActivation takes a row-level write lock on the license so that
// concurrent activation transactions serialize on it. Without this, under the
// default READ COMMITTED isolation two parallel activations for distinct machine
// hashes could both read a count below the cap and both insert, exceeding the
// device limit (a COUNT(*) cannot itself be locked with FOR UPDATE). Callers must
// invoke this before CountActivationsForUpdate inside the same transaction.
func LockLicenseForActivation(ctx context.Context, tx *sql.Tx, licenseID string) error {
	_, err := tx.ExecContext(ctx,
		`SELECT id FROM licenses WHERE id = $1 FOR UPDATE`, licenseID)
	return err
}

// CountActivationsForUpdate counts activations inside a transaction. TOCTOU
// protection comes from the caller holding LockLicenseForActivation on the
// license row for the duration of the transaction.
func CountActivationsForUpdate(ctx context.Context, tx *sql.Tx, licenseID string) (int, error) {
	var count int
	err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM activations WHERE license_id = $1`, licenseID).Scan(&count)
	return count, err
}

// ActivationExistsForMachine checks if a machine is already activated for a license (inside tx).
func ActivationExistsForMachine(ctx context.Context, tx *sql.Tx, licenseID, machineHash string) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM activations WHERE license_id = $1 AND machine_hash = $2)`,
		licenseID, machineHash).Scan(&exists)
	return exists, err
}

// scanActivation scans a single Activation row from the standard 7-column select.
func scanActivation(scan func(...any) error) (models.Activation, error) {
	var a models.Activation
	var lastSeenAt sql.NullTime
	if err := scan(&a.ID, &a.LicenseID, &a.MachineHash, &a.DeviceLabel, &a.KeyID, &a.ActivatedAt, &lastSeenAt); err != nil {
		return a, err
	}
	if lastSeenAt.Valid {
		a.LastSeenAt = &lastSeenAt.Time
	}
	return a, nil
}

func UpsertActivation(ctx context.Context, tx *sql.Tx, licenseID, machineHash, deviceLabel string, keyID *string) (*models.Activation, error) {
	id := dbutil.NewID()
	row := tx.QueryRowContext(ctx, `
		INSERT INTO activations (id, license_id, machine_hash, device_label, key_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (license_id, machine_hash) DO UPDATE
			SET last_seen_at = NOW(),
			    device_label = COALESCE(NULLIF(EXCLUDED.device_label, ''), activations.device_label),
			    key_id = EXCLUDED.key_id
		RETURNING id, license_id, machine_hash, device_label, key_id, activated_at, last_seen_at`,
		id, licenseID, machineHash, deviceLabel, keyID)

	a, err := scanActivation(row.Scan)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func GetLicensesByEmail(ctx context.Context, db *sql.DB, email, defaultLocale string) ([]models.License, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT l.id, l.key, l.order_id, l.app_id, COALESCE(a.bundle_id, '') AS app_bundle_id,
		       COALESCE(tn.value, '') AS app_name,
		       l.revoked, l.max_activations, l.created_at
		FROM licenses l
		JOIN orders o ON o.id = l.order_id
		LEFT JOIN apps a ON a.id = l.app_id
		LEFT JOIN entity_translations tn ON tn.entity_type = 'project' AND tn.entity_id = a.project_id AND tn.field = 'title' AND tn.locale = $1
		WHERE o.email = $2
		ORDER BY l.created_at DESC`, defaultLocale, email)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []models.License
	for rows.Next() {
		var l models.License
		if err := rows.Scan(&l.ID, &l.Key, &l.OrderID, &l.AppID, &l.AppBundleID, &l.AppName, &l.Revoked, &l.MaxActivations, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

func RevokeLicense(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx, `UPDATE licenses SET revoked = TRUE WHERE id = $1`, id)
	return err
}

func UnrevokeLicense(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx, `UPDATE licenses SET revoked = FALSE WHERE id = $1`, id)
	return err
}

func RevokeLicenseByOrderID(ctx context.Context, db *sql.DB, orderID string) error {
	_, err := db.ExecContext(ctx, `UPDATE licenses SET revoked = TRUE WHERE order_id = $1`, orderID)
	return err
}

func GetActivationsByLicenseID(ctx context.Context, db *sql.DB, licenseID string) ([]models.Activation, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, license_id, machine_hash, device_label, key_id, activated_at, last_seen_at
		FROM activations WHERE license_id = $1`, licenseID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []models.Activation
	for rows.Next() {
		a, err := scanActivation(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func GetActivationByIDAndEmail(ctx context.Context, db *sql.DB, activationID, email string) (*models.Activation, error) {
	row := db.QueryRowContext(ctx, `
		SELECT a.id, a.license_id, a.machine_hash, a.device_label, a.key_id, a.activated_at, a.last_seen_at
		FROM activations a
		JOIN licenses l ON l.id = a.license_id
		JOIN orders o   ON o.id = l.order_id
		WHERE a.id = $1 AND o.email = $2`, activationID, email)
	a, err := scanActivation(row.Scan)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &a, nil
}

func UpdateDeviceLabel(ctx context.Context, db *sql.DB, activationID, label string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE activations SET device_label = $1 WHERE id = $2`, label, activationID)
	return err
}

func DeleteActivation(ctx context.Context, db *sql.DB, activationID string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM activations WHERE id = $1`, activationID)
	return err
}

// DeleteActivationByLicenseAndMachine removes the activation for a
// (license_id, machine_hash) pair. Returns true if a row was deleted.
func DeleteActivationByLicenseAndMachine(ctx context.Context, db *sql.DB, licenseID, machineHash string) (bool, error) {
	res, err := db.ExecContext(ctx,
		`DELETE FROM activations WHERE license_id = $1 AND machine_hash = $2`,
		licenseID, machineHash)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
