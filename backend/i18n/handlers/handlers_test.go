package handlers_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SteVio89/stevio-home/i18n"
	_ "github.com/SteVio89/stevio-home/i18n/handlers" // ensure handlers package compiles
	"github.com/SteVio89/stevio-home/testutil"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	return testutil.SetupTestDB(t, "i18n", i18n.MigrationFiles)
}

func seedLocale(t *testing.T, db *sql.DB, code, name string, isDefault, enabled bool) {
	t.Helper()
	_, err := db.Exec(
		`INSERT INTO locales (code, name, is_default, enabled) VALUES ($1, $2, $3, $4)`,
		code, name, isDefault, enabled)
	if err != nil {
		t.Fatalf("seed locale %s: %v", code, err)
	}
}

func getReq(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// --- Locale Queries ---

func TestLocaleQueries_CRUD(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	ctx := context.Background()

	l := i18n.Locale{Code: "de", Name: "Deutsch", Enabled: true}
	if err := i18n.UpsertLocale(ctx, db, l); err != nil {
		t.Fatalf("UpsertLocale: %v", err)
	}

	got, err := i18n.GetLocale(ctx, db, "de")
	if err != nil {
		t.Fatalf("GetLocale: %v", err)
	}
	if got == nil || got.Name != "Deutsch" {
		t.Errorf("GetLocale(de) = %v", got)
	}

	all, err := i18n.ListAllLocales(ctx, db)
	if err != nil {
		t.Fatalf("ListAllLocales: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("ListAllLocales = %d, want 1", len(all))
	}
}

// --- UI Translation Queries ---

func TestUITranslationQueries_CRUD(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	ctx := context.Background()

	if err := i18n.UpsertUITranslation(ctx, db, "de", "nav.login", "Anmelden"); err != nil {
		t.Fatalf("UpsertUITranslation: %v", err)
	}

	translations, err := i18n.GetUITranslations(ctx, db, "de")
	if err != nil {
		t.Fatalf("GetUITranslations: %v", err)
	}
	if translations["nav.login"] != "Anmelden" {
		t.Errorf("nav.login = %q", translations["nav.login"])
	}

	if err := i18n.DeleteUITranslation(ctx, db, "de", "nav.login"); err != nil {
		t.Fatalf("DeleteUITranslation: %v", err)
	}

	translations, _ = i18n.GetUITranslations(ctx, db, "de")
	if len(translations) != 0 {
		t.Errorf("after delete: %d translations", len(translations))
	}
}

// --- Mail Template Queries ---

func TestMailTemplateQueries_CRUD(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	ctx := context.Background()

	if err := i18n.UpsertMailTemplate(ctx, db, "en", "magic_link", "Your link", "Click: %s"); err != nil {
		t.Fatalf("UpsertMailTemplate: %v", err)
	}

	mt, err := i18n.GetMailTemplate(ctx, db, "en", "magic_link")
	if err != nil {
		t.Fatalf("GetMailTemplate: %v", err)
	}
	if mt.Subject != "Your link" {
		t.Errorf("subject = %q", mt.Subject)
	}

	templates, err := i18n.ListMailTemplates(ctx, db, "magic_link")
	if err != nil {
		t.Fatalf("ListMailTemplates: %v", err)
	}
	if len(templates) != 1 {
		t.Errorf("ListMailTemplates = %d, want 1", len(templates))
	}
}

// --- UITranslationsHandler (stdlib http.Handler) ---

func TestUITranslationsHandler(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)

	seedLocale(t, db, "de", "Deutsch", false, true)
	_ = i18n.UpsertUITranslation(context.Background(), db, "de", "nav.login", "Anmelden")

	cache := i18n.NewLocaleCache(db)
	handler := i18n.UITranslationsHandler(db, cache)

	mux := http.NewServeMux()
	mux.Handle("GET /i18n/{locale}", handler)

	// Valid locale.
	w := getReq(t, mux, "/i18n/de")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if w.Header().Get("Cache-Control") != "public, max-age=60" {
		t.Errorf("Cache-Control = %q", w.Header().Get("Cache-Control"))
	}
	var translations map[string]string
	decodeJSON(t, w, &translations)
	if translations["nav.login"] != "Anmelden" {
		t.Errorf("nav.login = %q", translations["nav.login"])
	}

	// Unknown locale.
	w = getReq(t, mux, "/i18n/fr")
	if w.Code != http.StatusNotFound {
		t.Errorf("unknown locale status = %d, want 404", w.Code)
	}
}

// --- Overlay ---

func TestEntityTranslation_Overlay(t *testing.T) {
	t.Parallel()

	fields := map[string]string{
		"name":  "Produkt",
		"empty": "",
	}
	overlay := i18n.NewOverlay(fields)

	name := "Original"
	empty := "Keep"
	missing := "Keep"

	overlay.Apply("name", &name)
	overlay.Apply("empty", &empty)
	overlay.Apply("missing", &missing)

	if name != "Produkt" {
		t.Errorf("name = %q", name)
	}
	if empty != "Keep" {
		t.Errorf("empty value should not be applied, got %q", empty)
	}
	if missing != "Keep" {
		t.Errorf("missing field should not be applied, got %q", missing)
	}
}
