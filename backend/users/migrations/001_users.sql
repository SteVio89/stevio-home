-- Users table: one row per authenticated email (hashed).
-- email_hash is HMAC-SHA256(email, EMAIL_HASH_SALT) — we never store raw emails.
CREATE TABLE IF NOT EXISTS users (
    id          TEXT PRIMARY KEY,
    email_hash  TEXT NOT NULL UNIQUE,
    role        TEXT NOT NULL DEFAULT 'member',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email_hash ON users(email_hash);
