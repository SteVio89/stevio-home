// Package handlers provides reusable admin handler funcs for the i18n engine.
// Apps wire these into their own route trees, controlling URL structure and auth scopes.
package handlers

import (
	"errors"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/i18n"
)

var errInvalidLocaleCode = errors.New("invalid locale code format")

var validLocaleCode = regexp.MustCompile(`^[a-z]{2,8}(-[a-z0-9]{1,8})*$`)

// --- Locale Handlers ---

// LocaleHandlers provides CRUD handlers for the locale registry.
type LocaleHandlers struct {
	locales *i18n.LocaleCache
	db      *app.DB
	logger  *log.Logger
}

// NewLocaleHandlers creates a new LocaleHandlers.
func NewLocaleHandlers(locales *i18n.LocaleCache, db *app.DB, logger *log.Logger) *LocaleHandlers {
	if logger == nil {
		logger = log.Default()
	}
	return &LocaleHandlers{locales: locales, db: db, logger: logger}
}

// List handles GET — returns all locales (enabled + disabled).
func (h *LocaleHandlers) List(c *app.Ctx) error {
	locales, err := i18n.ListAllLocales(c.R.Context(), h.db.DB)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, locales)
}

// Create handles POST — creates a new locale.
func (h *LocaleHandlers) Create(c *app.Ctx) error {
	var req struct {
		Code      string `json:"code"`
		Name      string `json:"name"`
		IsDefault bool   `json:"is_default"`
		Enabled   bool   `json:"enabled"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.DecodeValidate(&req, func(ve *app.ValidationErrors) {
		req.Code = strings.ToLower(strings.TrimSpace(req.Code))
		req.Name = strings.TrimSpace(req.Name)
		if req.Code == "" {
			ve.Add("code", app.ErrRequired)
		} else if !validLocaleCode.MatchString(req.Code) {
			ve.Add("code", errInvalidLocaleCode)
		}
		if req.Name == "" {
			ve.Add("name", app.ErrRequired)
		}
	}); err != nil {
		return err
	}

	locale := i18n.Locale{
		Code:      req.Code,
		Name:      req.Name,
		IsDefault: req.IsDefault,
		Enabled:   req.Enabled,
		SortOrder: req.SortOrder,
	}
	if err := i18n.UpsertLocale(c.R.Context(), h.db.DB, locale); err != nil {
		return err
	}

	h.locales.Invalidate()
	return c.JSON(http.StatusCreated, locale)
}

// Update handles PATCH /{code} — updates an existing locale.
func (h *LocaleHandlers) Update(c *app.Ctx) error {
	code := c.Param("code")
	if code == "" {
		return app.BadRequest("locale code required")
	}

	existing, err := i18n.GetLocale(c.R.Context(), h.db.DB, code)
	if err != nil {
		return err
	}
	if existing == nil {
		return app.NotFound("locale not found")
	}

	var req struct {
		Name      *string `json:"name"`
		IsDefault *bool   `json:"is_default"`
		Enabled   *bool   `json:"enabled"`
		SortOrder *int    `json:"sort_order"`
	}
	if err := c.Decode(&req); err != nil {
		return err
	}

	if req.Name != nil {
		existing.Name = strings.TrimSpace(*req.Name)
	}
	if req.IsDefault != nil {
		existing.IsDefault = *req.IsDefault
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.SortOrder != nil {
		existing.SortOrder = *req.SortOrder
	}

	if err := i18n.UpsertLocale(c.R.Context(), h.db.DB, *existing); err != nil {
		return err
	}

	h.locales.Invalidate()
	return c.JSON(http.StatusOK, existing)
}

// --- UI Translation Handlers ---

// UITranslationHandlers provides CRUD for UI string overrides.
type UITranslationHandlers struct {
	locales *i18n.LocaleCache
	db      *app.DB
	logger  *log.Logger
}

// NewUITranslationHandlers creates a new UITranslationHandlers.
func NewUITranslationHandlers(locales *i18n.LocaleCache, db *app.DB, logger *log.Logger) *UITranslationHandlers {
	if logger == nil {
		logger = log.Default()
	}
	return &UITranslationHandlers{locales: locales, db: db, logger: logger}
}

// Get handles GET /{locale} — returns all UI translations for a locale.
func (h *UITranslationHandlers) Get(c *app.Ctx) error {
	locale := c.Param("locale")
	if locale == "" {
		return app.BadRequest("locale required")
	}

	translations, err := i18n.GetUITranslations(c.R.Context(), h.db.DB, locale)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, translations)
}

// Upsert handles PUT /{locale} — creates or updates UI translations.
// Expects JSON body: {"key": "value", ...}
func (h *UITranslationHandlers) Upsert(c *app.Ctx) error {
	locale := c.Param("locale")
	if locale == "" {
		return app.BadRequest("locale required")
	}

	var translations map[string]string
	if err := c.Decode(&translations); err != nil {
		return err
	}

	for key, value := range translations {
		if err := i18n.UpsertUITranslation(c.R.Context(), h.db.DB, locale, key, value); err != nil {
			return err
		}
	}

	return c.NoContent()
}

// Delete handles DELETE /{locale}/{key} — removes a single UI translation.
func (h *UITranslationHandlers) Delete(c *app.Ctx) error {
	locale := c.Param("locale")
	key := c.Param("key")
	if locale == "" || key == "" {
		return app.BadRequest("locale and key required")
	}

	if err := i18n.DeleteUITranslation(c.R.Context(), h.db.DB, locale, key); err != nil {
		return err
	}
	return c.NoContent()
}

// --- Mail Template Handlers ---

// MailTemplateHandlers provides CRUD for mail templates.
type MailTemplateHandlers struct {
	locales  *i18n.LocaleCache
	resolver *i18n.MailTemplateResolver
	db       *app.DB
	logger   *log.Logger
}

// NewMailTemplateHandlers creates a new MailTemplateHandlers.
func NewMailTemplateHandlers(locales *i18n.LocaleCache, resolver *i18n.MailTemplateResolver, db *app.DB, logger *log.Logger) *MailTemplateHandlers {
	if logger == nil {
		logger = log.Default()
	}
	return &MailTemplateHandlers{locales: locales, resolver: resolver, db: db, logger: logger}
}

// Get handles GET /{template} — returns all locales for a template.
func (h *MailTemplateHandlers) Get(c *app.Ctx) error {
	template := c.Param("template")
	if template == "" {
		return app.BadRequest("template name required")
	}

	templates, err := i18n.ListMailTemplates(c.R.Context(), h.db.DB, template)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, templates)
}

// Upsert handles PUT /{locale}/{template} — creates or updates a mail template.
func (h *MailTemplateHandlers) Upsert(c *app.Ctx) error {
	locale := c.Param("locale")
	template := c.Param("template")
	if locale == "" || template == "" {
		return app.BadRequest("locale and template required")
	}

	var req struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := c.DecodeValidate(&req, func(ve *app.ValidationErrors) {
		req.Subject = strings.TrimSpace(req.Subject)
		req.Body = strings.TrimSpace(req.Body)
		if req.Subject == "" {
			ve.Add("subject", app.ErrRequired)
		}
		if req.Body == "" {
			ve.Add("body", app.ErrRequired)
		}
	}); err != nil {
		return err
	}

	if err := i18n.UpsertMailTemplate(c.R.Context(), h.db.DB, locale, template, req.Subject, req.Body); err != nil {
		return err
	}

	// Update in-memory cache.
	h.resolver.SetTemplate(locale, template, req.Subject, req.Body)

	return c.NoContent()
}
