-- Payment refactor: remove Lemon Squeezy entirely, drop the static Paddle
-- price_id (we now pass products+prices inline via Paddle non-catalog items),
-- rename the test provider to "mock", and add per-app tax_category for Paddle.

-- 1. Drop orphaned payment settings. These keys are no longer read by any
--    code path after this migration. Secrets were AES-256-GCM ciphertext —
--    deleting them is the correct way to "forget" the plaintext.
DELETE FROM site_settings WHERE key IN (
    'lemonsqueezy_api_key',
    'lemonsqueezy_webhook_secret',
    'ls_store_id',
    'ls_variant_id',
    'paddle_price_id'
);

-- 2. If any admin previously selected Lemon Squeezy, clear it — the provider
--    package is gone, so an unchanged value would produce ErrProviderUnknown.
--    Admin must explicitly reselect Paddle or Mock.
UPDATE site_settings SET value = '' WHERE key = 'payment_provider' AND value = 'lemonsqueezy';

-- 3. Rename the test provider for consistency with the new codebase naming.
UPDATE site_settings SET value = 'mock' WHERE key = 'payment_provider' AND value = 'test';

-- 4. Per-app Paddle tax category. Ignored by the mock provider; required by
--    Paddle. Default "digital-goods" is always enabled on any Paddle account
--    (other categories need explicit activation in the dashboard).
ALTER TABLE apps
    ADD COLUMN IF NOT EXISTS tax_category TEXT NOT NULL DEFAULT 'digital-goods';
