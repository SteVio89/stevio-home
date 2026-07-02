package i18n

import (
	"context"

	"github.com/SteVio89/stevio-home/dbutil"
)

// GetPageTranslation returns field→value for a single page key in one locale.
// Returns an empty (non-nil) map when no rows are found.
func GetPageTranslation(ctx context.Context, db dbutil.Querier, pageKey, locale string) (map[string]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT field, value FROM page_translations WHERE page_key = $1 AND locale = $2`,
		pageKey, locale)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]string)
	for rows.Next() {
		var field, value string
		if err := rows.Scan(&field, &value); err != nil {
			return nil, err
		}
		result[field] = value
	}
	return result, rows.Err()
}

// GetPageTranslations returns all translations for a single page key across all locales.
// Result: locale → field → value.
func GetPageTranslations(ctx context.Context, db dbutil.Querier, pageKey string) (map[string]map[string]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT locale, field, value FROM page_translations WHERE page_key = $1`,
		pageKey)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]map[string]string)
	for rows.Next() {
		var locale, field, value string
		if err := rows.Scan(&locale, &field, &value); err != nil {
			return nil, err
		}
		if result[locale] == nil {
			result[locale] = make(map[string]string)
		}
		result[locale][field] = value
	}
	return result, rows.Err()
}

// GetPageTranslationsForLocale returns translations for all page keys in one locale.
// Result: pageKey → field → value. Used for batch loading (avoids N+1).
func GetPageTranslationsForLocale(ctx context.Context, db dbutil.Querier, locale string) (map[string]map[string]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT page_key, field, value FROM page_translations WHERE locale = $1`,
		locale)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]map[string]string)
	for rows.Next() {
		var pageKey, field, value string
		if err := rows.Scan(&pageKey, &field, &value); err != nil {
			return nil, err
		}
		if result[pageKey] == nil {
			result[pageKey] = make(map[string]string)
		}
		result[pageKey][field] = value
	}
	return result, rows.Err()
}

// UpsertPageTranslation creates or updates a single page translation field.
func UpsertPageTranslation(ctx context.Context, db dbutil.Querier, pageKey, locale, field, value string) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO page_translations (page_key, locale, field, value)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT(page_key, locale, field) DO UPDATE SET
		   value = excluded.value,
		   updated_at = NOW()`,
		pageKey, locale, field, value)
	return err
}

// UpsertPageTranslationFields upserts multiple fields for one page key + locale.
// Each field is a separate Exec call. Could be batched as a multi-row INSERT
// in Postgres if/when this becomes a hotspot.
func UpsertPageTranslationFields(ctx context.Context, db dbutil.Querier, pageKey, locale string, fields map[string]string) error {
	for field, value := range fields {
		if err := UpsertPageTranslation(ctx, db, pageKey, locale, field, value); err != nil {
			return err
		}
	}
	return nil
}

// DeletePageTranslations deletes all translations for a page key (all locales).
func DeletePageTranslations(ctx context.Context, db dbutil.Querier, pageKey string) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM page_translations WHERE page_key = $1`,
		pageKey)
	return err
}

// DeletePageTranslation deletes translations for a page key in a single locale.
func DeletePageTranslation(ctx context.Context, db dbutil.Querier, pageKey, locale string) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM page_translations WHERE page_key = $1 AND locale = $2`,
		pageKey, locale)
	return err
}
