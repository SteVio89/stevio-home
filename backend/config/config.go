package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/SteVio89/stevio-home/crypto"
)

type Config struct {
	// DatabaseURL is the Postgres DSN, e.g.
	// "postgres://user:pass@host:5432/dbname?sslmode=disable".
	DatabaseURL        string
	SigningKeySecret   [32]byte // AES-256-GCM key for encrypting signing keys in DB
	SessionSecretBytes []byte
	SMTPHost           string
	SMTPPort           int
	SMTPUser           string
	SMTPPass           string
	SMTPFrom           string
	BaseURL            string
	Env                string // "development" | "production"
	Port               string
	CORSOrigin         string // only used when Env == "development"
	// Payment provider credentials live in site_settings (encrypted at rest),
	// managed through the admin UI. No env-var fallback by design.
	// AdminEmails is the allowlist of emails that can access admin endpoints.
	// Loaded from ADMIN_EMAILS env: "you@example.com,other@example.com"
	AdminEmails []string
	// AdminEmailHashes contains HMAC-SHA256 hashes of AdminEmails, pre-computed at startup.
	AdminEmailHashes []string
	// EmailHashSalt is the HMAC key used by crypto.HashEmail. Loaded from EMAIL_HASH_SALT env.
	EmailHashSalt string
	// AssetsDir is the directory where uploaded images (icons, screenshots) are stored.
	AssetsDir string
	// AppsDir is the directory where uploaded app binaries (.dmg, .pkg) are stored.
	AppsDir string
}

func Load() (*Config, error) {
	cfg := &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		SMTPHost:    os.Getenv("SMTP_HOST"),
		SMTPUser:    os.Getenv("SMTP_USER"),
		SMTPPass:    os.Getenv("SMTP_PASS"),
		SMTPFrom:    getEnv("SMTP_FROM", "noreply@example.com"),
		BaseURL:     getEnv("BASE_URL", "http://localhost:3000"),
		Env:         getEnv("ENV", "development"),
		Port:        getEnv("PORT", "8080"),
		CORSOrigin:  getEnv("CORS_ORIGIN", "http://localhost:5173"),
	}

	if !strings.HasPrefix(cfg.DatabaseURL, "postgres://") && !strings.HasPrefix(cfg.DatabaseURL, "postgresql://") && cfg.DatabaseURL != "" {
		return nil, fmt.Errorf("config: DATABASE_URL must start with postgres:// or postgresql:// (got %q)", maskDSN(cfg.DatabaseURL))
	}

	portStr := getEnv("SMTP_PORT", "587")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("config: SMTP_PORT must be an integer, got %q", portStr)
	}
	cfg.SMTPPort = port

	cfg.EmailHashSalt = os.Getenv("EMAIL_HASH_SALT")
	cfg.AdminEmails = parseCSV(os.Getenv("ADMIN_EMAILS"))
	for _, e := range cfg.AdminEmails {
		cfg.AdminEmailHashes = append(cfg.AdminEmailHashes, crypto.HashEmail(e, cfg.EmailHashSalt))
	}
	cfg.AssetsDir = getEnv("ASSETS_DIR", "/assets")
	cfg.AppsDir = getEnv("APPS_DIR", "/apps")

	// In production, require a high-entropy EMAIL_HASH_SALT. The salt is the only
	// thing standing between a leaked DB dump and de-pseudonymization of every
	// hashed email (an attacker who guesses a weak salt can brute-force candidate
	// addresses offline). The CLI generator emits a 64-char hex string (32 bytes);
	// enforce that bar in production while leaving development lenient so the local
	// stack can use a human-readable salt.
	if cfg.Env == "production" && cfg.EmailHashSalt != "" {
		b, err := hex.DecodeString(cfg.EmailHashSalt)
		if err != nil || len(b) < 32 {
			return nil, fmt.Errorf("config: EMAIL_HASH_SALT must be a 64-char hex string (32 bytes) in production; use 'store-cli generate salt' to create one")
		}
	}

	secretHex := os.Getenv("SESSION_SECRET")
	if secretHex != "" {
		b, err := hex.DecodeString(secretHex)
		if err != nil || len(b) < 32 {
			return nil, fmt.Errorf("config: SESSION_SECRET must be a 64-char hex string (32 bytes); use 'just gensecret' to generate one")
		}
		cfg.SessionSecretBytes = b
	}

	signingKeyHex := os.Getenv("SIGNING_KEY_SECRET")
	if signingKeyHex != "" {
		b, err := hex.DecodeString(signingKeyHex)
		if err != nil || len(b) != 32 {
			return nil, fmt.Errorf("config: SIGNING_KEY_SECRET must be a 64-char hex string (32 bytes)")
		}
		copy(cfg.SigningKeySecret[:], b)
	}

	var missing []string
	check := func(name, val string) {
		if val == "" {
			missing = append(missing, name)
		}
	}
	check("DATABASE_URL", cfg.DatabaseURL)
	check("SIGNING_KEY_SECRET", signingKeyHex)
	check("EMAIL_HASH_SALT", cfg.EmailHashSalt)
	if len(cfg.SessionSecretBytes) == 0 {
		missing = append(missing, "SESSION_SECRET")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("config: missing required env vars: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseCSV(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// maskDSN strips the userinfo section of a Postgres DSN for safe logging.
func maskDSN(dsn string) string {
	if i := strings.Index(dsn, "@"); i > 0 {
		if j := strings.Index(dsn, "://"); j > 0 && j < i {
			return dsn[:j+3] + "***" + dsn[i:]
		}
	}
	return dsn
}
