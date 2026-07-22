package queries

import (
	"context"
	"database/sql"
	"errors"
)

// GetProviderProductID returns the cached external product id for a
// (provider, app, environment) triple, or "" if no mapping exists yet. The
// environment is part of the key because providers such as Polar keep separate
// catalogs per environment, so a product id is only valid within the one it was
// created in.
func GetProviderProductID(ctx context.Context, db *sql.DB, provider, appID, environment string) (string, error) {
	var id string
	err := db.QueryRowContext(ctx,
		`SELECT external_product_id FROM provider_products WHERE provider = $1 AND app_id = $2 AND environment = $3`,
		provider, appID, environment,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

// UpsertProviderProduct records (or replaces) the external product id created
// for a (provider, app, environment) triple.
func UpsertProviderProduct(ctx context.Context, db *sql.DB, provider, appID, environment, externalProductID string) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO provider_products (provider, app_id, environment, external_product_id)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (provider, app_id, environment) DO UPDATE SET external_product_id = EXCLUDED.external_product_id`,
		provider, appID, environment, externalProductID,
	)
	return err
}
