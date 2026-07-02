package i18n

import (
	"context"
	"testing"
	"time"
)

// --- Locale Queries ---

func TestGetEnabledLocales(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	seedLocale(t, db, "en", "English", true)
	seedLocale(t, db, "de", "Deutsch", false)

	// Disable de.
	_, _ = db.Exec(`UPDATE locales SET enabled = FALSE WHERE code = 'de'`)

	locales, err := GetEnabledLocales(ctx, db)
	if err != nil {
		t.Fatalf("GetEnabledLocales: %v", err)
	}
	if len(locales) != 1 || locales[0].Code != "en" {
		t.Errorf("got %v, want only [en]", locales)
	}
}

func TestListAllLocales(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	seedLocale(t, db, "en", "English", false)
	seedLocale(t, db, "de", "Deutsch", true)

	locales, err := ListAllLocales(ctx, db)
	if err != nil {
		t.Fatalf("ListAllLocales: %v", err)
	}
	if len(locales) != 2 {
		t.Fatalf("got %d locales, want 2", len(locales))
	}
	// Default (de) should be first.
	if locales[0].Code != "de" {
		t.Errorf("first locale = %q, want %q (default first)", locales[0].Code, "de")
	}
}

func TestGetLocale(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	seedLocale(t, db, "en", "English", true)

	l, err := GetLocale(ctx, db, "en")
	if err != nil {
		t.Fatalf("GetLocale: %v", err)
	}
	if l == nil || l.Code != "en" {
		t.Errorf("GetLocale(en) = %v, want en", l)
	}

	l, err = GetLocale(ctx, db, "fr")
	if err != nil {
		t.Fatalf("GetLocale(fr): %v", err)
	}
	if l != nil {
		t.Errorf("GetLocale(fr) = %v, want nil", l)
	}
}

func TestUpsertLocale(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	l := Locale{Code: "fr", Name: "Français", Enabled: true}
	if err := UpsertLocale(ctx, db, l); err != nil {
		t.Fatalf("UpsertLocale: %v", err)
	}

	// Update name.
	l.Name = "French"
	if err := UpsertLocale(ctx, db, l); err != nil {
		t.Fatalf("UpsertLocale update: %v", err)
	}

	got, _ := GetLocale(ctx, db, "fr")
	if got.Name != "French" {
		t.Errorf("name after upsert = %q, want %q", got.Name, "French")
	}
}

func TestSetDefaultLocale(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	seedLocale(t, db, "en", "English", true)
	seedLocale(t, db, "de", "Deutsch", false)

	tx, _ := db.Begin()
	if err := SetDefaultLocale(ctx, tx, "de"); err != nil {
		_ = tx.Rollback()
		t.Fatalf("SetDefaultLocale: %v", err)
	}
	_ = tx.Commit()

	code := GetDefaultLocaleCode(ctx, db)
	if code != "de" {
		t.Errorf("default = %q, want %q", code, "de")
	}
}

func TestGetDefaultLocaleCode_Empty(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	code := GetDefaultLocaleCode(ctx, db)
	if code != "en" {
		t.Errorf("default on empty = %q, want %q", code, "en")
	}
}

// --- UI Translations ---

func TestUITranslations_CRUD(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	// Upsert.
	if err := UpsertUITranslation(ctx, db, "de", "nav.login", "Anmelden"); err != nil {
		t.Fatalf("UpsertUITranslation: %v", err)
	}
	if err := UpsertUITranslation(ctx, db, "de", "nav.logout", "Abmelden"); err != nil {
		t.Fatalf("UpsertUITranslation: %v", err)
	}

	// Get.
	got, err := GetUITranslations(ctx, db, "de")
	if err != nil {
		t.Fatalf("GetUITranslations: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d translations, want 2", len(got))
	}
	if got["nav.login"] != "Anmelden" {
		t.Errorf("nav.login = %q, want %q", got["nav.login"], "Anmelden")
	}

	// Update existing.
	if err := UpsertUITranslation(ctx, db, "de", "nav.login", "Einloggen"); err != nil {
		t.Fatalf("UpsertUITranslation update: %v", err)
	}
	got, _ = GetUITranslations(ctx, db, "de")
	if got["nav.login"] != "Einloggen" {
		t.Errorf("after update = %q, want %q", got["nav.login"], "Einloggen")
	}

	// Delete.
	if err := DeleteUITranslation(ctx, db, "de", "nav.login"); err != nil {
		t.Fatalf("DeleteUITranslation: %v", err)
	}
	got, _ = GetUITranslations(ctx, db, "de")
	if len(got) != 1 {
		t.Errorf("after delete got %d, want 1", len(got))
	}
}

func TestUITranslations_EmptyLocale(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)

	got, err := GetUITranslations(context.Background(), db, "fr")
	if err != nil {
		t.Fatalf("GetUITranslations: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty locale got %d translations, want 0", len(got))
	}
}

// --- Entity Translations ---

func TestEntityTranslation_SingleLocale(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	if err := UpsertEntityTranslation(ctx, db, "product", "p1", "de", "name", "Produkt 1"); err != nil {
		t.Fatalf("UpsertEntityTranslation: %v", err)
	}
	if err := UpsertEntityTranslation(ctx, db, "product", "p1", "de", "description", "Beschreibung"); err != nil {
		t.Fatalf("UpsertEntityTranslation: %v", err)
	}

	fields, err := GetEntityTranslation(ctx, db, "product", "p1", "de")
	if err != nil {
		t.Fatalf("GetEntityTranslation: %v", err)
	}
	if fields["name"] != "Produkt 1" {
		t.Errorf("name = %q, want %q", fields["name"], "Produkt 1")
	}
	if fields["description"] != "Beschreibung" {
		t.Errorf("description = %q, want %q", fields["description"], "Beschreibung")
	}
}

func TestEntityTranslation_AllLocales(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	_ = UpsertEntityTranslation(ctx, db, "product", "p1", "de", "name", "Produkt")
	_ = UpsertEntityTranslation(ctx, db, "product", "p1", "fr", "name", "Produit")

	all, err := GetEntityTranslations(ctx, db, "product", "p1")
	if err != nil {
		t.Fatalf("GetEntityTranslations: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("got %d locales, want 2", len(all))
	}
	if all["de"]["name"] != "Produkt" {
		t.Errorf("de.name = %q", all["de"]["name"])
	}
	if all["fr"]["name"] != "Produit" {
		t.Errorf("fr.name = %q", all["fr"]["name"])
	}
}

func TestEntityTranslation_ForLocale(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	_ = UpsertEntityTranslation(ctx, db, "product", "p1", "de", "name", "Produkt 1")
	_ = UpsertEntityTranslation(ctx, db, "product", "p2", "de", "name", "Produkt 2")

	batch, err := GetEntityTranslationsForLocale(ctx, db, "product", "de")
	if err != nil {
		t.Fatalf("GetEntityTranslationsForLocale: %v", err)
	}
	if len(batch) != 2 {
		t.Fatalf("got %d entities, want 2", len(batch))
	}
	if batch["p1"]["name"] != "Produkt 1" {
		t.Errorf("p1.name = %q", batch["p1"]["name"])
	}
}

func TestUpsertEntityTranslationFields(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	fields := map[string]string{
		"name":        "Produkt",
		"description": "Beschreibung",
	}
	if err := UpsertEntityTranslationFields(ctx, db, "product", "p1", "de", fields); err != nil {
		t.Fatalf("UpsertEntityTranslationFields: %v", err)
	}

	got, _ := GetEntityTranslation(ctx, db, "product", "p1", "de")
	if len(got) != 2 {
		t.Errorf("got %d fields, want 2", len(got))
	}
}

func TestDeleteEntityTranslations(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	_ = UpsertEntityTranslation(ctx, db, "product", "p1", "de", "name", "Produkt")
	_ = UpsertEntityTranslation(ctx, db, "product", "p1", "en", "name", "Product")

	if err := DeleteEntityTranslations(ctx, db, "product", "p1"); err != nil {
		t.Fatalf("DeleteEntityTranslations: %v", err)
	}

	got, _ := GetEntityTranslation(ctx, db, "product", "p1", "de")
	if len(got) != 0 {
		t.Errorf("after delete got %d fields, want 0", len(got))
	}
}

func TestDeleteEntityTranslationsByIDs(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	_ = UpsertEntityTranslation(ctx, db, "product", "p1", "de", "name", "P1")
	_ = UpsertEntityTranslation(ctx, db, "product", "p2", "de", "name", "P2")
	_ = UpsertEntityTranslation(ctx, db, "product", "p3", "de", "name", "P3")

	if err := DeleteEntityTranslationsByIDs(ctx, db, "product", []string{"p1", "p2"}); err != nil {
		t.Fatalf("DeleteEntityTranslationsByIDs: %v", err)
	}

	// p3 should remain.
	got, _ := GetEntityTranslation(ctx, db, "product", "p3", "de")
	if got["name"] != "P3" {
		t.Errorf("p3 should remain, got %v", got)
	}
	// p1 and p2 should be gone.
	got, _ = GetEntityTranslation(ctx, db, "product", "p1", "de")
	if len(got) != 0 {
		t.Error("p1 should be deleted")
	}
}

func TestDeleteEntityTranslationsByIDs_Empty(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)

	// Should not error on empty slice.
	if err := DeleteEntityTranslationsByIDs(context.Background(), db, "product", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Overlay ---

func TestOverlay_Apply(t *testing.T) {
	t.Parallel()
	fields := map[string]string{
		"name":        "Produkt",
		"description": "Beschreibung",
		"empty":       "",
	}
	overlay := NewOverlay(fields)

	name := "Original"
	desc := "Original"
	other := "Original"
	empty := "Original"

	overlay.Apply("name", &name)
	overlay.Apply("description", &desc)
	overlay.Apply("missing", &other)
	overlay.Apply("empty", &empty)

	if name != "Produkt" {
		t.Errorf("name = %q, want %q", name, "Produkt")
	}
	if desc != "Beschreibung" {
		t.Errorf("desc = %q, want %q", desc, "Beschreibung")
	}
	if other != "Original" {
		t.Errorf("other = %q, want %q (missing field)", other, "Original")
	}
	if empty != "Original" {
		t.Errorf("empty = %q, want %q (empty value not applied)", empty, "Original")
	}
}

func TestOverlay_NilSafe(t *testing.T) {
	t.Parallel()
	var overlay *Overlay
	s := "original"
	overlay.Apply("key", &s) // should not panic
	if s != "original" {
		t.Errorf("nil overlay changed value")
	}

	overlay2 := NewOverlay(nil)
	overlay2.Apply("key", &s) // should not panic
	if s != "original" {
		t.Errorf("nil fields overlay changed value")
	}
}

// --- Mail Templates ---

func TestMailTemplate_CRUD(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	// Upsert.
	if err := UpsertMailTemplate(ctx, db, "en", "magic_link", "Your login link", "Click: %s"); err != nil {
		t.Fatalf("UpsertMailTemplate: %v", err)
	}
	if err := UpsertMailTemplate(ctx, db, "de", "magic_link", "Dein Login-Link", "Klicke: %s"); err != nil {
		t.Fatalf("UpsertMailTemplate: %v", err)
	}

	// Get.
	mt, err := GetMailTemplate(ctx, db, "en", "magic_link")
	if err != nil {
		t.Fatalf("GetMailTemplate: %v", err)
	}
	if mt.Subject != "Your login link" {
		t.Errorf("subject = %q", mt.Subject)
	}

	// Not found.
	mt, err = GetMailTemplate(ctx, db, "fr", "magic_link")
	if err != nil {
		t.Fatalf("GetMailTemplate fr: %v", err)
	}
	if mt != nil {
		t.Errorf("expected nil for fr, got %v", mt)
	}

	// List.
	templates, err := ListMailTemplates(ctx, db, "magic_link")
	if err != nil {
		t.Fatalf("ListMailTemplates: %v", err)
	}
	if len(templates) != 2 {
		t.Errorf("list got %d, want 2", len(templates))
	}
}

func TestMailTemplateResolver_FallbackChain(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	// Only insert English template.
	_ = UpsertMailTemplate(ctx, db, "en", "magic_link", "Your link", "Click: %s")

	resolver := NewMailTemplateResolver(db, nil, "en", time.Hour, nil)
	t.Cleanup(resolver.Stop)

	// Request German — should fall back to English.
	subject, body, err := resolver.Resolve(ctx, "de", "magic_link")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if subject != "Your link" {
		t.Errorf("subject = %q, want %q", subject, "Your link")
	}
	if body != "Click: %s" {
		t.Errorf("body = %q, want %q", body, "Click: %s")
	}
}

func TestMailTemplateResolver_PreferLocale(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	_ = UpsertMailTemplate(ctx, db, "en", "magic_link", "Your link", "Click: %s")
	_ = UpsertMailTemplate(ctx, db, "de", "magic_link", "Dein Link", "Klicke: %s")

	resolver := NewMailTemplateResolver(db, nil, "en", time.Hour, nil)
	t.Cleanup(resolver.Stop)

	subject, _, _ := resolver.Resolve(ctx, "de", "magic_link")
	if subject != "Dein Link" {
		t.Errorf("subject = %q, want %q", subject, "Dein Link")
	}
}

func TestMailTemplateResolver_CatalogFallback(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	// No DB templates — fall through to catalog.
	catalog := NewCatalog("en")
	catalog.Load("en", map[string]string{
		"magic_link.subject": "Catalog Subject",
		"magic_link.body":    "Catalog Body: %s",
	})

	resolver := NewMailTemplateResolver(db, catalog, "en", time.Hour, nil)
	t.Cleanup(resolver.Stop)

	subject, body, _ := resolver.Resolve(ctx, "en", "magic_link")
	if subject != "Catalog Subject" {
		t.Errorf("subject = %q, want %q", subject, "Catalog Subject")
	}
	if body != "Catalog Body: %s" {
		t.Errorf("body = %q, want %q", body, "Catalog Body: %s")
	}
}

func TestMailTemplateResolver_SetTemplate(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	resolver := NewMailTemplateResolver(db, nil, "en", time.Hour, nil)
	t.Cleanup(resolver.Stop)

	// Set in-memory.
	resolver.SetTemplate("en", "welcome", "Welcome!", "Hello there")

	subject, body, _ := resolver.Resolve(ctx, "en", "welcome")
	if subject != "Welcome!" {
		t.Errorf("subject = %q", subject)
	}
	if body != "Hello there" {
		t.Errorf("body = %q", body)
	}
}

func TestMailTemplateResolver_NotFound(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	resolver := NewMailTemplateResolver(db, nil, "en", time.Hour, nil)
	t.Cleanup(resolver.Stop)

	subject, body, err := resolver.Resolve(ctx, "en", "nonexistent")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if subject != "" || body != "" {
		t.Errorf("expected empty, got subject=%q body=%q", subject, body)
	}
}
