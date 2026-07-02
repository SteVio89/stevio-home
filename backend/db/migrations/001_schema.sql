-- stevio-home application schema (Postgres).
--
-- Consolidated schema (pre-prod: no migration history retained). Shape:
--   - Projects own display text (title/tagline/description).
--   - Apps are a commerce attachment on a Project (one-to-one via project_id).
--   - Screenshots generalized to project_images.
--   - All timestamps are TIMESTAMPTZ, all booleans are BOOLEAN.
--
-- Partial unique indexes enforce uniqueness only for non-empty values — letting
-- multiple rows share the default '' without blocking each other.

-- ============================================================
-- Projects (display layer; owns name/tagline/description via entity_translations)
-- ============================================================
CREATE TABLE IF NOT EXISTS projects (
    id              TEXT PRIMARY KEY,
    slug            TEXT NOT NULL DEFAULT '',
    external_url    TEXT NOT NULL DEFAULT '',
    image_url       TEXT NOT NULL DEFAULT '',
    position        INTEGER NOT NULL DEFAULT 0,
    has_detail_page BOOLEAN NOT NULL DEFAULT FALSE,
    deleted_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_projects_position ON projects(position ASC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_slug
    ON projects(slug) WHERE slug != '';

CREATE TABLE IF NOT EXISTS project_images (
    id          TEXT PRIMARY KEY,
    project_id  TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    url         TEXT NOT NULL,
    file_path   TEXT NOT NULL DEFAULT '',
    position    INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_project_images_project
    ON project_images(project_id, position ASC);

-- ============================================================
-- Apps (commerce layer; attached to a project 1:1)
-- ============================================================
CREATE TABLE IF NOT EXISTS apps (
    id            TEXT PRIMARY KEY,
    project_id    TEXT REFERENCES projects(id),
    bundle_id     TEXT NOT NULL DEFAULT '',
    price_cents   INTEGER NOT NULL DEFAULT 0,
    purchase_mode TEXT NOT NULL DEFAULT 'always_new_license',
    deleted_at    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_apps_project_id
    ON apps(project_id) WHERE project_id IS NOT NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS idx_apps_bundle_id
    ON apps(bundle_id) WHERE bundle_id != '' AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS app_versions (
    id            TEXT PRIMARY KEY,
    app_id        TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    version       TEXT NOT NULL,
    download_url  TEXT NOT NULL,
    file_path     TEXT NOT NULL DEFAULT '',
    published_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (app_id, version)
);

CREATE INDEX IF NOT EXISTS idx_app_versions_app
    ON app_versions(app_id, published_at DESC);

-- ============================================================
-- Orders & licenses
-- ============================================================
-- Note: orders.discount_code_id and orders.auto_discount_id are NOT
-- FK-constrained. They're nullable historical references — the discount
-- row may be soft-deleted (deleted_at set) while the order persists. The
-- app layer resolves them via JOIN with IS NULL checks.
CREATE TABLE IF NOT EXISTS orders (
    id                    TEXT PRIMARY KEY,
    payment_session       TEXT NOT NULL UNIQUE,
    -- email and app_id are nullable to support the refund-before-order
    -- "poison pill" stub inserted by InsertRefundStub. FulfillStubOrder
    -- populates both columns when the matching order webhook arrives.
    email                 TEXT,
    app_id                TEXT REFERENCES apps(id),
    price_paid_cents      INTEGER NOT NULL,
    discount_code_id      TEXT,
    auto_discount_id      TEXT,
    original_price_cents  INTEGER NOT NULL DEFAULT 0,
    discount_label        TEXT NOT NULL DEFAULT '',
    discount_type         TEXT NOT NULL DEFAULT '',
    discount_value        INTEGER NOT NULL DEFAULT 0,
    refunded              BOOLEAN NOT NULL DEFAULT FALSE,
    consent_given_at      TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_orders_email ON orders(email);
CREATE INDEX IF NOT EXISTS idx_orders_app ON orders(app_id);
CREATE INDEX IF NOT EXISTS idx_orders_created ON orders(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_discount_code_id
    ON orders(discount_code_id) WHERE discount_code_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_orders_auto_discount_id
    ON orders(auto_discount_id) WHERE auto_discount_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_orders_has_discount
    ON orders(created_at DESC)
    WHERE discount_code_id IS NOT NULL OR auto_discount_id IS NOT NULL;

-- licenses.order_id uses RESTRICT (the default) so an order can never be
-- hard-deleted while a license still references it — refunds flip the
-- `refunded` flag instead. licenses.app_id uses CASCADE for symmetry with
-- the rest of the app-scoped data, even though apps are soft-deleted in
-- practice (so the cascade rarely fires).
CREATE TABLE IF NOT EXISTS licenses (
    id              TEXT PRIMARY KEY,
    key             TEXT NOT NULL UNIQUE,
    order_id        TEXT NOT NULL REFERENCES orders(id) ON DELETE RESTRICT,
    app_id          TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    max_activations INTEGER NOT NULL DEFAULT 3,
    revoked         BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_licenses_order ON licenses(order_id);
CREATE INDEX IF NOT EXISTS idx_licenses_app ON licenses(app_id);

CREATE TABLE IF NOT EXISTS activations (
    id             TEXT PRIMARY KEY,
    license_id     TEXT NOT NULL REFERENCES licenses(id) ON DELETE CASCADE,
    machine_hash   TEXT NOT NULL,
    device_label   TEXT NOT NULL DEFAULT '',
    key_id         TEXT NOT NULL DEFAULT '',
    activated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at   TIMESTAMPTZ,
    UNIQUE (license_id, machine_hash)
);

CREATE INDEX IF NOT EXISTS idx_activations_license ON activations(license_id);

CREATE TABLE IF NOT EXISTS download_tokens (
    token       TEXT PRIMARY KEY,
    license_id  TEXT NOT NULL REFERENCES licenses(id) ON DELETE CASCADE,
    app_id      TEXT NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    expires_at  TIMESTAMPTZ NOT NULL,
    used        BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_download_tokens_license ON download_tokens(license_id);

-- ============================================================
-- Discounts (manual codes + automatic discounts)
-- ============================================================
CREATE TABLE IF NOT EXISTS discount_codes (
    id              TEXT PRIMARY KEY,
    code            TEXT NOT NULL UNIQUE,
    label           TEXT NOT NULL DEFAULT '',
    discount_type   TEXT NOT NULL,
    discount_value  INTEGER NOT NULL,
    app_id          TEXT REFERENCES apps(id),
    max_uses        INTEGER NOT NULL DEFAULT 0,
    uses            INTEGER NOT NULL DEFAULT 0,
    expires_at      TIMESTAMPTZ,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    stackable       BOOLEAN NOT NULL DEFAULT FALSE,
    deleted_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_discount_codes_code ON discount_codes(code);

CREATE TABLE IF NOT EXISTS auto_discounts (
    id              TEXT PRIMARY KEY,
    label           TEXT NOT NULL DEFAULT '',
    discount_type   TEXT NOT NULL,
    discount_value  INTEGER NOT NULL,
    app_id          TEXT REFERENCES apps(id),
    valid_from      TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    active          BOOLEAN NOT NULL DEFAULT TRUE,
    deleted_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_auto_discounts_active ON auto_discounts(active, valid_from, expires_at);
CREATE INDEX IF NOT EXISTS idx_auto_discounts_app_id ON auto_discounts(app_id);

-- ============================================================
-- Site settings (runtime key-value config accessed via settings.Store)
-- ============================================================
CREATE TABLE IF NOT EXISTS site_settings (
    key    TEXT PRIMARY KEY,
    value  TEXT NOT NULL DEFAULT ''
);

-- Seed default site settings. ON CONFLICT DO NOTHING makes this idempotent
-- and safe to rerun (e.g. on schema bootstrap with existing rows).
INSERT INTO site_settings (key, value) VALUES
    ('maintenance_mode', '0'),
    ('hero_title_key', 'hero.title'),
    ('hero_subtitle_key', 'hero.subtitle'),
    ('hero_cta_text_key', 'hero.cta'),
    ('hero_cta_url', '/'),
    ('contact_email', ''),
    ('support_email', '')
ON CONFLICT (key) DO NOTHING;

-- ============================================================
-- Chat (customer support messaging)
-- ============================================================
CREATE TABLE IF NOT EXISTS chat_conversations (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL UNIQUE,
    display_name  TEXT NOT NULL UNIQUE,
    email         TEXT NOT NULL DEFAULT '',
    notified_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_conversations_user_id ON chat_conversations(user_id);

CREATE TABLE IF NOT EXISTS chat_messages (
    id               TEXT PRIMARY KEY,
    conversation_id  TEXT NOT NULL REFERENCES chat_conversations(id) ON DELETE CASCADE,
    sender           TEXT NOT NULL,
    body             TEXT NOT NULL,
    read_at          TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_conversation_id
    ON chat_messages(conversation_id, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_chat_messages_read_at
    ON chat_messages(read_at) WHERE read_at IS NULL;

CREATE TABLE IF NOT EXISTS chat_bans (
    user_id     TEXT PRIMARY KEY,
    reason      TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- Social links (footer/hero links to X, Mastodon, etc.)
-- ============================================================
CREATE TABLE IF NOT EXISTS social_links (
    id          TEXT PRIMARY KEY,
    platform    TEXT NOT NULL,
    url         TEXT NOT NULL,
    position    INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_social_links_position ON social_links(position ASC);

-- ============================================================
-- Signing keys (Ed25519 keypairs for license signing; private key encrypted)
-- ============================================================
CREATE TABLE IF NOT EXISTS signing_keys (
    id                    TEXT PRIMARY KEY,
    key_id                TEXT NOT NULL UNIQUE,
    encrypted_private_key TEXT NOT NULL,
    public_key_b64        TEXT NOT NULL,
    active                BOOLEAN NOT NULL DEFAULT FALSE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Only one active signing key at a time.
CREATE UNIQUE INDEX IF NOT EXISTS idx_signing_keys_active
    ON signing_keys(active) WHERE active = TRUE;
