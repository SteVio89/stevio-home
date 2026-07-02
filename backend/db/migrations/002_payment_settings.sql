-- Payment provider credentials are admin-managed via /api/admin/settings.
-- Secret values (api_key, webhook_secret) are stored as AES-256-GCM ciphertext
-- encrypted under SIGNING_KEY_SECRET; encryption happens in the admin handler
-- before Upsert, so no plaintext is ever persisted here. Empty defaults mean
-- "not configured" — the checkout handler returns 503 payment_not_configured
-- until an admin sets them.
INSERT INTO site_settings (key, value) VALUES
    ('lemonsqueezy_api_key',        ''),
    ('lemonsqueezy_webhook_secret', ''),
    ('paddle_api_key',              ''),
    ('paddle_webhook_secret',       ''),
    ('paddle_price_id',             ''),
    ('paddle_environment',          'production')
ON CONFLICT (key) DO NOTHING;
