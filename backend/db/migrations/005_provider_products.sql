-- provider_products maps an internal app to the product record a payment
-- provider requires. Paddle uses non-catalog transactions (no product needed),
-- but Polar attaches each ad-hoc checkout price to a product_id, so we lazily
-- create one product per (provider, app) on first checkout and cache its
-- external id here. Prices still live on the apps table and are sent per
-- checkout as ad-hoc prices — this table only records the provider-side product
-- handle. Rows are inert when the provider is unset or switched, so nothing is
-- coupled to app publishing.
CREATE TABLE IF NOT EXISTS provider_products (
    provider            TEXT NOT NULL,
    app_id              TEXT NOT NULL,
    external_product_id TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (provider, app_id)
);

-- Polar credentials (mirrors the paddle_* rows from 002_payment_settings).
-- Secrets are stored as AES-256-GCM ciphertext encrypted under SIGNING_KEY_SECRET
-- by the admin handler before Upsert; empty means "not configured".
INSERT INTO site_settings (key, value) VALUES
    ('polar_api_key',        ''),
    ('polar_webhook_secret', ''),
    ('polar_environment',    'production')
ON CONFLICT (key) DO NOTHING;
