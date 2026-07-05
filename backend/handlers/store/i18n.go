package store

import (
	"net/http"
	"strconv"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/handlers/common"
	"github.com/SteVio89/stevio-home/i18n"
)

// GetUITranslations handles GET /api/i18n/{locale}.
func (h *StoreHandler) GetUITranslations(c *app.Ctx) error {
	loc := c.Param("locale")
	if !common.LocaleCodeRe.MatchString(loc) {
		return c.JSON(http.StatusOK, map[string]string{})
	}
	translations, err := i18n.GetUITranslations(c.R.Context(), c.DB().DB, loc)
	if err != nil {
		h.log.Printf("i18n: get translations %q: %v", loc, err)
		return apierr.ErrInternal()
	}
	if translations == nil {
		translations = make(map[string]string)
	}
	c.W.Header().Set("Cache-Control", "public, max-age=60")
	return c.JSON(http.StatusOK, translations)
}

// GetPublicConfig handles GET /api/config.
//
// Settings are stored as TEXT in site_settings; this handler is the boundary
// where they get coerced into proper JSON types (bool, int) so frontend code
// doesn't have to remember which "1"/"0" means what.
func (h *StoreHandler) GetPublicConfig(c *app.Ctx) error {
	s, err := c.Settings().GetAll(c.R.Context())
	if err != nil {
		h.log.Printf("public-config: %v", err)
		return apierr.ErrInternal()
	}

	return c.JSON(http.StatusOK, map[string]any{
		"currency_symbol":  orDefault(s["currency_symbol"], "€"),
		"currency_code":    orDefault(s["currency_code"], "EUR"),
		"site_name":        orDefault(s["site_name"], "My Store"),
		"maintenance_mode": parseBoolFlag(s["maintenance_mode"]),
		"payment_enabled":  s["payment_provider"] != "",
		"payment_provider": s["payment_provider"],
		// Paddle client-side token is browser-safe by design; the frontend needs
		// it (plus the environment) to initialise Paddle.js for the overlay.
		"paddle_client_token": s["paddle_client_token"],
		"paddle_environment":  orDefault(s["paddle_environment"], "production"),
		"max_activations":     parseIntDefault(s["max_activations"], 3),
		"base_url":            h.cfg.BaseURL,
		"locales":             c.Locales().EnabledLocales(c.R.Context()),
	})
}

func orDefault(val, def string) string {
	if val == "" {
		return def
	}
	return val
}

// parseBoolFlag treats "1"/"true"/"yes"/"on" (case-insensitive) as true and
// everything else (including the empty string) as false. Tolerant on input so
// settings written by future admin UIs don't silently fall back to false.
func parseBoolFlag(val string) bool {
	switch val {
	case "1", "true", "TRUE", "True", "yes", "YES", "on", "ON":
		return true
	default:
		return false
	}
}

func parseIntDefault(val string, def int) int {
	if val == "" {
		return def
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return def
	}
	return n
}
