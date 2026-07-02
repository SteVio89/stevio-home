package i18n

import (
	"context"
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/SteVio89/stevio-home/testutil"
)

func setupI18nDB(t *testing.T) *sql.DB {
	t.Helper()
	return testutil.SetupTestDB(t, "i18n", MigrationFiles)
}

func TestLocaleCache_FallbackWhenEmpty(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	cache := NewLocaleCache(db, WithTTL(time.Millisecond))
	ctx := context.Background()

	got := cache.Default(ctx)
	if got != "en" {
		t.Errorf("Default() = %q, want %q", got, "en")
	}

	locales := cache.EnabledLocales(ctx)
	if len(locales) != 1 || locales[0].Code != "en" {
		t.Errorf("EnabledLocales() = %v, want fallback [en]", locales)
	}
}

func TestLocaleCache_LoadsFromDB(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	seedLocale(t, db, "de", "Deutsch", true)
	seedLocale(t, db, "en", "English", false)

	cache := NewLocaleCache(db, WithTTL(time.Hour))

	got := cache.Default(ctx)
	if got != "de" {
		t.Errorf("Default() = %q, want %q", got, "de")
	}

	codes := cache.Supported(ctx)
	if len(codes) != 2 {
		t.Fatalf("Supported() returned %d codes, want 2", len(codes))
	}
}

func TestLocaleCache_IsSupported(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	seedLocale(t, db, "en", "English", true)
	cache := NewLocaleCache(db, WithTTL(time.Hour))

	if !cache.IsSupported(ctx, "en") {
		t.Error("IsSupported(en) = false, want true")
	}
	if cache.IsSupported(ctx, "fr") {
		t.Error("IsSupported(fr) = true, want false")
	}
}

func TestLocaleCache_Invalidate(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	seedLocale(t, db, "en", "English", true)
	cache := NewLocaleCache(db, WithTTL(time.Hour))

	codes := cache.Supported(ctx)
	if len(codes) != 1 {
		t.Fatalf("initial Supported() = %d, want 1", len(codes))
	}

	seedLocale(t, db, "de", "Deutsch", false)
	cache.Invalidate()

	codes = cache.Supported(ctx)
	if len(codes) != 2 {
		t.Fatalf("after invalidate Supported() = %d, want 2", len(codes))
	}
}

func TestLocaleCache_FromRequest(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)

	seedLocale(t, db, "en", "English", true)
	seedLocale(t, db, "de", "Deutsch", false)
	cache := NewLocaleCache(db, WithTTL(time.Hour))

	tests := []struct {
		name       string
		acceptLang string
		want       string
	}{
		{"exact match", "de", "de"},
		{"subtag match", "de-AT", "de"},
		{"quality preference", "fr;q=0.5, de;q=0.9", "de"},
		{"empty header", "", "en"},
		{"no match", "ja", "en"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := http.NewRequest("GET", "/", nil)
			if tt.acceptLang != "" {
				r.Header.Set("Accept-Language", tt.acceptLang)
			}
			got := cache.FromRequest(r)
			if got != tt.want {
				t.Errorf("FromRequest(%q) = %q, want %q", tt.acceptLang, got, tt.want)
			}
		})
	}
}

func TestResolveAcceptLang(t *testing.T) {
	t.Parallel()
	supported := []string{"en", "de", "pt"}
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{"empty header", "", "en"},
		{"exact match", "de", "de"},
		{"subtag", "pt-BR", "pt"},
		{"quality", "fr;q=0.5, de;q=0.9", "de"},
		{"no match", "ja", "en"},
		{"case insensitive", "DE", "de"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveAcceptLang(tt.header, supported, "en")
			if got != tt.want {
				t.Errorf("resolveAcceptLang(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

func seedLocale(t *testing.T, db *sql.DB, code, name string, isDefault bool) {
	t.Helper()
	def := 0
	if isDefault {
		def = 1
	}
	_, err := db.Exec(
		`INSERT INTO locales (code, name, is_default, enabled) VALUES ($1, $2, $3, 1)`,
		code, name, def)
	if err != nil {
		t.Fatalf("seed locale %s: %v", code, err)
	}
}
