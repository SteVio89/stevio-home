-- Add `environment` to the provider_products key.
--
-- Polar keeps sandbox and production as entirely separate catalogs: a product id
-- created in sandbox does not exist in production. The original (provider, app)
-- key had no notion of environment, so a product cached during sandbox testing
-- was reused verbatim after switching a store to production, and every checkout
-- failed with "Product does not exist" (Polar 422). Keying the cache by
-- environment as well lets each environment hold its own product id.
--
-- Existing rows are backfilled to 'production' rather than dropped: by the time
-- this ships, any leftover sandbox rows have already been cleared by hand and
-- the remaining mappings point at live production products. Backfilling (not
-- deleting) preserves those products so the provider keeps reusing them instead
-- of creating duplicates in the Polar dashboard.
ALTER TABLE provider_products
    ADD COLUMN IF NOT EXISTS environment TEXT NOT NULL DEFAULT 'production';

ALTER TABLE provider_products DROP CONSTRAINT provider_products_pkey;
ALTER TABLE provider_products ADD PRIMARY KEY (provider, app_id, environment);

-- Drop the default so future inserts must state the environment explicitly
-- (the app always does); this keeps a forgotten value from silently landing in
-- 'production'.
ALTER TABLE provider_products ALTER COLUMN environment DROP DEFAULT;
