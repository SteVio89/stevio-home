package queries

import (
	"context"
	"database/sql"

	"github.com/SteVio89/stevio-home/db/models"
)

func scanSigningKey(scan func(...any) error) (models.SigningKey, error) {
	var k models.SigningKey
	if err := scan(&k.ID, &k.KeyID, &k.EncryptedPrivateKey, &k.PublicKeyB64, &k.Active, &k.CreatedAt); err != nil {
		return k, err
	}
	return k, nil
}

// GetActiveSigningKey returns the currently active signing key, or (nil, nil) if none.
func GetActiveSigningKey(ctx context.Context, db *sql.DB) (*models.SigningKey, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, key_id, encrypted_private_key, public_key_b64, active, created_at
		FROM signing_keys WHERE active = TRUE`)
	k, err := scanSigningKey(row.Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// GetSigningKeyByID returns a signing key by its primary key, or (nil, nil) if not found.
func GetSigningKeyByID(ctx context.Context, db *sql.DB, id string) (*models.SigningKey, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, key_id, encrypted_private_key, public_key_b64, active, created_at
		FROM signing_keys WHERE id = $1`, id)
	k, err := scanSigningKey(row.Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// InsertSigningKey inserts a new signing key (not active by default).
func InsertSigningKey(ctx context.Context, db *sql.DB, id, keyID, encryptedPrivKey, pubKeyB64 string) (*models.SigningKey, error) {
	row := db.QueryRowContext(ctx, `
		INSERT INTO signing_keys (id, key_id, encrypted_private_key, public_key_b64)
		VALUES ($1, $2, $3, $4)
		RETURNING id, key_id, encrypted_private_key, public_key_b64, active, created_at`,
		id, keyID, encryptedPrivKey, pubKeyB64)
	k, err := scanSigningKey(row.Scan)
	if err != nil {
		return nil, err
	}
	return &k, nil
}

// ActivateSigningKey sets the given key active and all others inactive, atomically.
func ActivateSigningKey(ctx context.Context, db *sql.DB, id string) error {
	return WithTx(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE signing_keys SET active = FALSE WHERE active = TRUE`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `UPDATE signing_keys SET active = TRUE WHERE id = $1`, id)
		return err
	})
}

// DeleteInactiveSigningKeys deletes all inactive signing keys.
func DeleteInactiveSigningKeys(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `DELETE FROM signing_keys WHERE active = FALSE`)
	return err
}
