package config

import (
	"reflect"
	"testing"
)

func TestParseCSV(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty string", "", nil},
		{"single value", "admin@example.com", []string{"admin@example.com"}},
		{"two values", "a@b.com,c@d.com", []string{"a@b.com", "c@d.com"}},
		{"whitespace trimmed", " a@b.com , c@d.com ", []string{"a@b.com", "c@d.com"}},
		{"trailing comma", "a@b.com,", []string{"a@b.com"}},
		{"leading comma", ",a@b.com", []string{"a@b.com"}},
		{"multiple commas", "a@b.com,,c@d.com", []string{"a@b.com", "c@d.com"}},
		{"only commas", ",,", nil},
		{"spaces only entries", " , , ", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCSV(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseCSV(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadRequiredVars(t *testing.T) {
	// Clear env to test required var validation
	t.Setenv("DATABASE_URL", "")
	t.Setenv("SIGNING_KEY_SECRET", "")
	t.Setenv("SESSION_SECRET", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing required vars")
	}
}

// validTestSecret is a 64-char hex string (32 bytes) suitable for SESSION_SECRET in tests.
const validTestSecret = "aaaaaaaabbbbbbbbccccccccddddddddeeeeeeeeffffffff0000000011111111"

// validSigningSecret is a 64-char hex string (32 bytes) suitable for SIGNING_KEY_SECRET in tests.
const validSigningSecret = "1111111122222222333333334444444455555555666666667777777788888888"

func TestLoadSuccess(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test:5432/test?sslmode=disable")
	t.Setenv("SIGNING_KEY_SECRET", validSigningSecret)
	t.Setenv("SESSION_SECRET", validTestSecret)
	t.Setenv("EMAIL_HASH_SALT", "test-salt")
	t.Setenv("ADMIN_EMAILS", "admin@test.com, super@test.com")
	t.Setenv("SMTP_PORT", "587")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DatabaseURL != "postgres://test:test:5432/test?sslmode=disable" {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://test:test:5432/test?sslmode=disable")
	}
	if len(cfg.SessionSecretBytes) != 32 {
		t.Errorf("SessionSecretBytes length = %d, want 32", len(cfg.SessionSecretBytes))
	}
	if len(cfg.AdminEmails) != 2 {
		t.Errorf("AdminEmails length = %d, want 2", len(cfg.AdminEmails))
	}
	if len(cfg.AdminEmailHashes) != 2 {
		t.Errorf("AdminEmailHashes length = %d, want 2", len(cfg.AdminEmailHashes))
	}
}

func TestLoadSessionSecretInvalid(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test:5432/test?sslmode=disable")
	t.Setenv("SIGNING_KEY_SECRET", validSigningSecret)
	t.Setenv("SMTP_PORT", "587")

	// Non-hex string
	t.Setenv("SESSION_SECRET", "not-valid-hex!")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for non-hex SESSION_SECRET")
	}

	// Too short (only 16 bytes = 32 hex chars)
	t.Setenv("SESSION_SECRET", "aabbccddeeff00112233445566778899")
	_, err = Load()
	if err == nil {
		t.Fatal("expected error for SESSION_SECRET shorter than 32 bytes")
	}
}

// validHexSalt is a 64-char hex string (32 bytes), the shape store-cli emits.
const validHexSalt = "abababababababababababababababababababababababababababababababcdcd"

func TestLoadEmailSaltProductionValidation(t *testing.T) {
	base := func() {
		t.Setenv("DATABASE_URL", "postgres://test:test@localhost:5432/test?sslmode=disable")
		t.Setenv("SIGNING_KEY_SECRET", validSigningSecret)
		t.Setenv("SESSION_SECRET", validTestSecret)
		t.Setenv("SMTP_PORT", "587")
	}

	t.Run("weak salt rejected in production", func(t *testing.T) {
		base()
		t.Setenv("ENV", "production")
		t.Setenv("EMAIL_HASH_SALT", "dev-salt-not-for-production")
		if _, err := Load(); err == nil {
			t.Fatal("expected error for weak EMAIL_HASH_SALT in production")
		}
	})

	t.Run("hex salt accepted in production", func(t *testing.T) {
		base()
		t.Setenv("ENV", "production")
		t.Setenv("EMAIL_HASH_SALT", validHexSalt)
		if _, err := Load(); err != nil {
			t.Fatalf("unexpected error for valid hex salt in production: %v", err)
		}
	})

	t.Run("weak salt allowed in development", func(t *testing.T) {
		base()
		t.Setenv("ENV", "development")
		t.Setenv("EMAIL_HASH_SALT", "dev-salt-not-for-production")
		if _, err := Load(); err != nil {
			t.Fatalf("dev must tolerate a weak salt: %v", err)
		}
	})
}

func TestLoadSMTPPortInvalid(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test:5432/test?sslmode=disable")
	t.Setenv("SIGNING_KEY_SECRET", validSigningSecret)
	t.Setenv("SESSION_SECRET", validTestSecret)
	t.Setenv("SMTP_PORT", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid SMTP_PORT")
	}
}
