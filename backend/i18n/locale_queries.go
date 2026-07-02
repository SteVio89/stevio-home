package i18n

import (
	"context"
	"database/sql"
	"errors"
)

// GetEnabledLocales returns enabled locales ordered by sort_order.
func GetEnabledLocales(ctx context.Context, db *sql.DB) ([]Locale, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT code, name, is_default, enabled, sort_order, created_at
		 FROM locales WHERE enabled = TRUE ORDER BY sort_order, code`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanLocales(rows)
}

// ListAllLocales returns all locales (enabled + disabled), default first.
func ListAllLocales(ctx context.Context, db *sql.DB) ([]Locale, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT code, name, is_default, enabled, sort_order, created_at
		 FROM locales ORDER BY is_default DESC, sort_order, code`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanLocales(rows)
}

// GetLocale returns a single locale by code. Returns (nil, nil) when not found.
func GetLocale(ctx context.Context, db *sql.DB, code string) (*Locale, error) {
	var l Locale
	err := db.QueryRowContext(ctx,
		`SELECT code, name, is_default, enabled, sort_order, created_at
		 FROM locales WHERE code = $1`, code).
		Scan(&l.Code, &l.Name, &l.IsDefault, &l.Enabled, &l.SortOrder, &l.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// UpsertLocale inserts or updates a locale.
func UpsertLocale(ctx context.Context, db *sql.DB, l Locale) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO locales (code, name, is_default, enabled, sort_order)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT(code) DO UPDATE SET
		   name = excluded.name,
		   is_default = excluded.is_default,
		   enabled = excluded.enabled,
		   sort_order = excluded.sort_order`,
		l.Code, l.Name, l.IsDefault, l.Enabled, l.SortOrder)
	return err
}

// SetDefaultLocale clears all defaults and sets one. Must run inside a transaction.
func SetDefaultLocale(ctx context.Context, tx *sql.Tx, code string) error {
	if _, err := tx.ExecContext(ctx, `UPDATE locales SET is_default = FALSE`); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE locales SET is_default = TRUE WHERE code = $1`, code)
	return err
}

// GetDefaultLocaleCode returns the default locale code.
// Returns "en" as a hardcoded fallback if the table is empty or has no default.
func GetDefaultLocaleCode(ctx context.Context, db *sql.DB) string {
	var code string
	err := db.QueryRowContext(ctx,
		`SELECT code FROM locales WHERE is_default = TRUE LIMIT 1`).Scan(&code)
	if err != nil || code == "" {
		return "en"
	}
	return code
}

func scanLocales(rows *sql.Rows) ([]Locale, error) {
	var locales []Locale
	for rows.Next() {
		var l Locale
		if err := rows.Scan(&l.Code, &l.Name, &l.IsDefault, &l.Enabled, &l.SortOrder, &l.CreatedAt); err != nil {
			return nil, err
		}
		locales = append(locales, l)
	}
	return locales, rows.Err()
}
