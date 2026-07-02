package i18n

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
)

// GetUITranslations returns all UI translation overrides for a locale.
func GetUITranslations(ctx context.Context, db *sql.DB, locale string) (map[string]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT key, value FROM locale_translations WHERE locale = $1 ORDER BY key`, locale)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		result[k] = v
	}
	return result, rows.Err()
}

// UpsertUITranslation creates or updates a single UI translation override.
func UpsertUITranslation(ctx context.Context, db *sql.DB, locale, key, value string) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO locale_translations (locale, key, value)
		 VALUES ($1, $2, $3)
		 ON CONFLICT(locale, key) DO UPDATE SET
		   value = excluded.value,
		   updated_at = NOW()`,
		locale, key, value)
	return err
}

// DeleteUITranslation removes a single UI translation override.
func DeleteUITranslation(ctx context.Context, db *sql.DB, locale, key string) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM locale_translations WHERE locale = $1 AND key = $2`, locale, key)
	return err
}

// UITranslationsHandler returns an http.Handler that serves GET /{locale}
// as a JSON map of UI translation overrides. Includes Cache-Control headers.
func UITranslationsHandler(db *sql.DB, locales *LocaleCache) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locale := r.PathValue("locale")
		if locale == "" {
			http.Error(w, "locale required", http.StatusBadRequest)
			return
		}

		if !locales.IsSupported(r.Context(), locale) {
			http.NotFound(w, r)
			return
		}

		translations, err := GetUITranslations(r.Context(), db, locale)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=60")
		_ = json.NewEncoder(w).Encode(translations)
	})
}
