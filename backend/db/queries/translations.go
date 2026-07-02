package queries

import (
	"context"
	"database/sql"
)

// UpsertTranslationTx upserts a single entity translation field inside a transaction.
// This mirrors i18n.UpsertEntityTranslation but works on *sql.Tx for atomicity.
func UpsertTranslationTx(ctx context.Context, tx *sql.Tx, entityType, entityID, locale, field, value string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO entity_translations (entity_type, entity_id, locale, field, value)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT(entity_type, entity_id, locale, field) DO UPDATE SET
		   value = excluded.value,
		   updated_at = NOW()`,
		entityType, entityID, locale, field, value)
	return err
}

// UpsertTranslationFieldsTx upserts multiple fields for one entity+locale inside a transaction.
func UpsertTranslationFieldsTx(ctx context.Context, tx *sql.Tx, entityType, entityID, locale string, fields map[string]string) error {
	for field, value := range fields {
		if err := UpsertTranslationTx(ctx, tx, entityType, entityID, locale, field, value); err != nil {
			return err
		}
	}
	return nil
}
