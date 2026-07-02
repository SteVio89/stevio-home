package admin

import (
	"database/sql"
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/i18n"
)

// allowedPageKeys restricts which page keys are valid for page translation endpoints.
var allowedPageKeys = map[string]bool{
	"hero":           true,
	"impressum":      true,
	"privacy_policy": true,
	"refund_policy":  true,
}

// knownPageKeys for deterministic iteration in list-all.
var knownPageKeys = []string{"hero", "impressum", "privacy_policy", "refund_policy"}

// pageKeyFieldLimits: max chars per field value, keyed by page_key.
// Hero: 2000 chars. Legal: 100,000 chars.
var pageKeyFieldLimits = map[string]int{
	"hero":           2000,
	"impressum":      100_000,
	"privacy_policy": 100_000,
	"refund_policy":  100_000,
}

// AdminListPageTranslations returns all page translations across all known page keys.
// Response shape: pageKey -> locale -> field -> value.
func (h *AdminHandler) AdminListPageTranslations(c *app.Ctx) error {
	ctx := c.R.Context()
	result := make(map[string]map[string]map[string]string, len(knownPageKeys))

	for _, key := range knownPageKeys {
		translations, err := i18n.GetPageTranslations(ctx, c.DB().DB, key)
		if err != nil {
			h.log.Printf("admin: list page translations %q: %v", key, err)
			return apierr.ErrInternal()
		}
		result[key] = translations
	}

	return c.JSON(http.StatusOK, result)
}

// AdminGetPageTranslation returns field->value for a single page key in one locale.
func (h *AdminHandler) AdminGetPageTranslation(c *app.Ctx) error {
	ctx := c.R.Context()
	pageKey := c.Param("pageKey")
	locale := c.Param("locale")

	if !allowedPageKeys[pageKey] {
		return apierr.ErrNotFound()
	}
	if !c.Locales().IsSupported(ctx, locale) {
		return apierr.ErrBadRequest()
	}

	fields, err := i18n.GetPageTranslation(ctx, c.DB().DB, pageKey, locale)
	if err != nil {
		h.log.Printf("admin: get page translation %q/%q: %v", pageKey, locale, err)
		return apierr.ErrInternal()
	}

	return c.JSON(http.StatusOK, fields)
}

// AdminUpsertPageTranslation creates or updates fields for a single page key + locale.
func (h *AdminHandler) AdminUpsertPageTranslation(c *app.Ctx) error {
	ctx := c.R.Context()
	pageKey := c.Param("pageKey")
	locale := c.Param("locale")

	if !allowedPageKeys[pageKey] {
		return apierr.ErrNotFound()
	}
	if !c.Locales().IsSupported(ctx, locale) {
		return apierr.ErrBadRequest()
	}

	var req struct {
		Fields map[string]string `json:"fields"`
	}
	if err := c.Decode(&req); err != nil {
		return apierr.ErrBadRequest()
	}

	limit := pageKeyFieldLimits[pageKey]
	for _, v := range req.Fields {
		if len(v) > limit {
			return apierr.ErrBadRequest()
		}
	}

	if err := c.DB().WithTx(ctx, func(tx *sql.Tx) error {
		return i18n.UpsertPageTranslationFields(ctx, tx, pageKey, locale, req.Fields)
	}); err != nil {
		h.log.Printf("admin: upsert page translation %q/%q: %v", pageKey, locale, err)
		return apierr.ErrInternal()
	}

	return c.NoContent()
}

// AdminDeletePageTranslation deletes all translations for a page key in one locale.
func (h *AdminHandler) AdminDeletePageTranslation(c *app.Ctx) error {
	ctx := c.R.Context()
	pageKey := c.Param("pageKey")
	locale := c.Param("locale")

	if !allowedPageKeys[pageKey] {
		return apierr.ErrNotFound()
	}
	if !c.Locales().IsSupported(ctx, locale) {
		return apierr.ErrBadRequest()
	}

	if err := i18n.DeletePageTranslation(ctx, c.DB().DB, pageKey, locale); err != nil {
		h.log.Printf("admin: delete page translation %q/%q: %v", pageKey, locale, err)
		return apierr.ErrInternal()
	}

	return c.NoContent()
}
