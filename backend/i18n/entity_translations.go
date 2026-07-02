package i18n

import (
	"context"
	"database/sql"

	"github.com/SteVio89/stevio-home/dbutil"
)

// GetEntityTranslation returns field→value for a single entity in one locale.
func GetEntityTranslation(ctx context.Context, db *sql.DB, entityType, entityID, locale string) (map[string]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT field, value FROM entity_translations
		 WHERE entity_type = $1 AND entity_id = $2 AND locale = $3`,
		entityType, entityID, locale)
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

// GetEntityTranslations returns all translations for a single entity across
// all locales. Result: locale → field → value.
func GetEntityTranslations(ctx context.Context, db *sql.DB, entityType, entityID string) (map[string]map[string]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT locale, field, value FROM entity_translations
		 WHERE entity_type = $1 AND entity_id = $2`,
		entityType, entityID)
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

// GetEntityTranslationsForLocale returns translations for all entities of a type
// in one locale. Result: entityID → field → value. Used for batch loading (avoids N+1).
func GetEntityTranslationsForLocale(ctx context.Context, db *sql.DB, entityType, locale string) (map[string]map[string]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT entity_id, field, value FROM entity_translations
		 WHERE entity_type = $1 AND locale = $2`,
		entityType, locale)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]map[string]string)
	for rows.Next() {
		var entityID, field, value string
		if err := rows.Scan(&entityID, &field, &value); err != nil {
			return nil, err
		}
		if result[entityID] == nil {
			result[entityID] = make(map[string]string)
		}
		result[entityID][field] = value
	}
	return result, rows.Err()
}

// UpsertEntityTranslation creates or updates a single entity translation field.
func UpsertEntityTranslation(ctx context.Context, db *sql.DB, entityType, entityID, locale, field, value string) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO entity_translations (entity_type, entity_id, locale, field, value)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT(entity_type, entity_id, locale, field) DO UPDATE SET
		   value = excluded.value,
		   updated_at = NOW()`,
		entityType, entityID, locale, field, value)
	return err
}

// UpsertEntityTranslationFields upserts multiple fields for one entity+locale.
// Each field is a separate Exec call. Could be batched as a multi-row INSERT
// in Postgres if/when this becomes a hotspot.
func UpsertEntityTranslationFields(ctx context.Context, db *sql.DB, entityType, entityID, locale string, fields map[string]string) error {
	for field, value := range fields {
		if err := UpsertEntityTranslation(ctx, db, entityType, entityID, locale, field, value); err != nil {
			return err
		}
	}
	return nil
}

// DeleteEntityTranslations deletes all translations for an entity.
func DeleteEntityTranslations(ctx context.Context, db *sql.DB, entityType, entityID string) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM entity_translations WHERE entity_type = $1 AND entity_id = $2`,
		entityType, entityID)
	return err
}

// DeleteEntityTranslationsByIDs deletes all translations for multiple entities
// of a given type in a single statement.
func DeleteEntityTranslationsByIDs(ctx context.Context, db *sql.DB, entityType string, entityIDs []string) error {
	if len(entityIDs) == 0 {
		return nil
	}

	args := make([]any, 0, len(entityIDs)+1)
	args = append(args, entityType)
	for _, id := range entityIDs {
		args = append(args, id)
	}

	// entity_type = $1, entity IDs start at $2.
	placeholders := dbutil.InPlaceholders(len(entityIDs), 1)

	_, err := db.ExecContext(ctx,
		`DELETE FROM entity_translations WHERE entity_type = $1 AND entity_id IN (`+placeholders+`)`,
		args...)
	return err
}

// Overlay applies translated values to struct fields.
// Only non-empty translation values are applied.
type Overlay struct {
	fields map[string]string
}

// NewOverlay creates an Overlay from a field→value map (from GetEntityTranslation).
func NewOverlay(fields map[string]string) *Overlay {
	return &Overlay{fields: fields}
}

// Apply sets target to the translation value for field, if non-empty.
func (o *Overlay) Apply(field string, target *string) {
	if o == nil || o.fields == nil {
		return
	}
	if v, ok := o.fields[field]; ok && v != "" {
		*target = v
	}
}
