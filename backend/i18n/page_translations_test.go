package i18n

import (
	"context"
	"testing"
)

func TestPageTranslation_UpsertAndGet(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	if err := UpsertPageTranslation(ctx, db, "hero", "de", "headline", "Willkommen"); err != nil {
		t.Fatalf("UpsertPageTranslation headline: %v", err)
	}
	if err := UpsertPageTranslation(ctx, db, "hero", "de", "tagline", "Tagline"); err != nil {
		t.Fatalf("UpsertPageTranslation tagline: %v", err)
	}

	fields, err := GetPageTranslation(ctx, db, "hero", "de")
	if err != nil {
		t.Fatalf("GetPageTranslation: %v", err)
	}
	if fields["headline"] != "Willkommen" {
		t.Errorf("headline = %q, want %q", fields["headline"], "Willkommen")
	}
	if fields["tagline"] != "Tagline" {
		t.Errorf("tagline = %q, want %q", fields["tagline"], "Tagline")
	}
}

func TestPageTranslation_UpsertFields(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	fields := map[string]string{
		"headline": "Willkommen",
		"tagline":  "Tagline",
		"bio":      "Über mich",
	}
	if err := UpsertPageTranslationFields(ctx, db, "hero", "de", fields); err != nil {
		t.Fatalf("UpsertPageTranslationFields: %v", err)
	}

	got, err := GetPageTranslation(ctx, db, "hero", "de")
	if err != nil {
		t.Fatalf("GetPageTranslation: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d fields, want 3", len(got))
	}
	if got["headline"] != "Willkommen" {
		t.Errorf("headline = %q, want %q", got["headline"], "Willkommen")
	}
	if got["bio"] != "Über mich" {
		t.Errorf("bio = %q, want %q", got["bio"], "Über mich")
	}
}

func TestPageTranslation_AllLocales(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	_ = UpsertPageTranslation(ctx, db, "hero", "de", "headline", "Willkommen")
	_ = UpsertPageTranslation(ctx, db, "hero", "en", "headline", "Welcome")

	all, err := GetPageTranslations(ctx, db, "hero")
	if err != nil {
		t.Fatalf("GetPageTranslations: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("got %d locales, want 2", len(all))
	}
	if all["de"]["headline"] != "Willkommen" {
		t.Errorf("de.headline = %q, want %q", all["de"]["headline"], "Willkommen")
	}
	if all["en"]["headline"] != "Welcome" {
		t.Errorf("en.headline = %q, want %q", all["en"]["headline"], "Welcome")
	}
}

func TestPageTranslation_ForLocale(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	_ = UpsertPageTranslation(ctx, db, "hero", "de", "headline", "Willkommen")
	_ = UpsertPageTranslation(ctx, db, "legal_impressum", "de", "content", "Impressum Inhalt")

	batch, err := GetPageTranslationsForLocale(ctx, db, "de")
	if err != nil {
		t.Fatalf("GetPageTranslationsForLocale: %v", err)
	}
	if len(batch) != 2 {
		t.Fatalf("got %d page keys, want 2", len(batch))
	}
	if batch["hero"]["headline"] != "Willkommen" {
		t.Errorf("hero.headline = %q, want %q", batch["hero"]["headline"], "Willkommen")
	}
	if batch["legal_impressum"]["content"] != "Impressum Inhalt" {
		t.Errorf("legal_impressum.content = %q, want %q", batch["legal_impressum"]["content"], "Impressum Inhalt")
	}
}

func TestPageTranslation_DeleteAll(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	_ = UpsertPageTranslation(ctx, db, "hero", "de", "headline", "Willkommen")
	_ = UpsertPageTranslation(ctx, db, "hero", "en", "headline", "Welcome")

	if err := DeletePageTranslations(ctx, db, "hero"); err != nil {
		t.Fatalf("DeletePageTranslations: %v", err)
	}

	got, err := GetPageTranslation(ctx, db, "hero", "de")
	if err != nil {
		t.Fatalf("GetPageTranslation after delete: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("after delete de got %d fields, want 0", len(got))
	}

	got, err = GetPageTranslation(ctx, db, "hero", "en")
	if err != nil {
		t.Fatalf("GetPageTranslation en after delete: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("after delete en got %d fields, want 0", len(got))
	}
}

func TestPageTranslation_DeleteSingleLocale(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	_ = UpsertPageTranslation(ctx, db, "hero", "de", "headline", "Willkommen")
	_ = UpsertPageTranslation(ctx, db, "hero", "en", "headline", "Welcome")

	if err := DeletePageTranslation(ctx, db, "hero", "de"); err != nil {
		t.Fatalf("DeletePageTranslation: %v", err)
	}

	// de should be gone
	got, err := GetPageTranslation(ctx, db, "hero", "de")
	if err != nil {
		t.Fatalf("GetPageTranslation de: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("de should be deleted, got %d fields", len(got))
	}

	// en should remain
	got, err = GetPageTranslation(ctx, db, "hero", "en")
	if err != nil {
		t.Fatalf("GetPageTranslation en: %v", err)
	}
	if got["headline"] != "Welcome" {
		t.Errorf("en.headline = %q, want %q", got["headline"], "Welcome")
	}
}

func TestPageTranslation_OverlayUsage(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	_ = UpsertPageTranslation(ctx, db, "hero", "de", "headline", "Willkommen")
	_ = UpsertPageTranslation(ctx, db, "hero", "de", "tagline", "Deine Plattform")

	fields, err := GetPageTranslation(ctx, db, "hero", "de")
	if err != nil {
		t.Fatalf("GetPageTranslation: %v", err)
	}

	// Use the shared Overlay type with page translation data
	overlay := NewOverlay(fields)

	headline := "Default Headline"
	tagline := "Default Tagline"
	missing := "Default Missing"

	overlay.Apply("headline", &headline)
	overlay.Apply("tagline", &tagline)
	overlay.Apply("nonexistent", &missing)

	if headline != "Willkommen" {
		t.Errorf("headline = %q, want %q", headline, "Willkommen")
	}
	if tagline != "Deine Plattform" {
		t.Errorf("tagline = %q, want %q", tagline, "Deine Plattform")
	}
	if missing != "Default Missing" {
		t.Errorf("missing field should not be overwritten, got %q", missing)
	}
}

func TestPageTranslation_EmptyResult(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	got, err := GetPageTranslation(ctx, db, "nonexistent_page", "de")
	if err != nil {
		t.Fatalf("GetPageTranslation: %v", err)
	}
	if got == nil {
		t.Error("GetPageTranslation returned nil map, want empty non-nil map")
	}
	if len(got) != 0 {
		t.Errorf("got %d fields, want 0", len(got))
	}
}

func TestPageTranslation_UpsertOverwrite(t *testing.T) {
	t.Parallel()
	db := setupI18nDB(t)
	ctx := context.Background()

	if err := UpsertPageTranslation(ctx, db, "hero", "de", "headline", "Erstes Mal"); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := UpsertPageTranslation(ctx, db, "hero", "de", "headline", "Zweites Mal"); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	got, err := GetPageTranslation(ctx, db, "hero", "de")
	if err != nil {
		t.Fatalf("GetPageTranslation: %v", err)
	}
	if got["headline"] != "Zweites Mal" {
		t.Errorf("headline = %q, want %q (second value wins)", got["headline"], "Zweites Mal")
	}
}
