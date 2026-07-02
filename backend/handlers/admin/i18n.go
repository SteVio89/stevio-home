package admin

import (
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/handlers/common"
	"github.com/SteVio89/stevio-home/i18n"
)

func (h *AdminHandler) AdminGetUITranslations(c *app.Ctx) error {
	loc := c.Param("locale")
	if !common.LocaleCodeRe.MatchString(loc) {
		return apierr.ErrBadRequest()
	}
	translations, err := i18n.GetUITranslations(c.R.Context(), c.DB().DB, loc)
	if err != nil {
		h.log.Printf("admin: get ui translations %q: %v", loc, err)
		return apierr.ErrInternal()
	}
	if translations == nil {
		translations = make(map[string]string)
	}
	return c.JSON(http.StatusOK, translations)
}

func (h *AdminHandler) AdminUpsertUITranslation(c *app.Ctx) error {
	loc := c.Param("locale")

	if !c.Locales().IsSupported(c.R.Context(), loc) {
		return apierr.ErrBadRequest()
	}

	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := c.Decode(&req); err != nil {
		return apierr.ErrBadRequest()
	}
	if !common.TranslationKeyRe.MatchString(req.Key) || len(req.Key) > 128 {
		return apierr.ErrBadRequest()
	}
	if len(req.Value) > 5000 {
		return apierr.ErrBadRequest()
	}

	if err := i18n.UpsertUITranslation(c.R.Context(), c.DB().DB, loc, req.Key, req.Value); err != nil {
		h.log.Printf("admin: upsert ui translation %q/%q: %v", loc, req.Key, err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}

func (h *AdminHandler) AdminDeleteUITranslation(c *app.Ctx) error {
	loc := c.Param("locale")
	key := c.Param("key")

	if !c.Locales().IsSupported(c.R.Context(), loc) {
		return apierr.ErrBadRequest()
	}

	if err := i18n.DeleteUITranslation(c.R.Context(), c.DB().DB, loc, key); err != nil {
		h.log.Printf("admin: delete ui translation %q/%q: %v", loc, key, err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}
