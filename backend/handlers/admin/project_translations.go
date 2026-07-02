package admin

import (
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/handlers/common"
	"github.com/SteVio89/stevio-home/i18n"
)

// allowedProjectFields restricts which fields can be set via the translation endpoint.
var allowedProjectFields = map[string]bool{
	"title":       true,
	"tagline":     true,
	"description": true,
}

func (h *AdminHandler) AdminGetProjectTranslations(c *app.Ctx) error {
	id := c.Param("id")
	locale := c.Param("locale")

	project, err := queries.GetProjectByID(c.R.Context(), c.DB().DB, id)
	if err != nil {
		h.log.Printf("admin: get project translations: lookup %q: %v", id, err)
		return apierr.ErrInternal()
	}
	if project == nil {
		return apierr.ErrNotFound()
	}

	fields, err := i18n.GetEntityTranslation(c.R.Context(), c.DB().DB, common.EntityTypeProject, id, locale)
	if err != nil {
		h.log.Printf("admin: get project translations %q/%q: %v", id, locale, err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusOK, fields)
}

func (h *AdminHandler) AdminUpsertProjectTranslation(c *app.Ctx) error {
	id := c.Param("id")
	locale := c.Param("locale")

	if !c.Locales().IsSupported(c.R.Context(), locale) {
		return apierr.ErrBadRequest()
	}

	project, err := queries.GetProjectByID(c.R.Context(), c.DB().DB, id)
	if err != nil {
		h.log.Printf("admin: upsert project translation: lookup %q: %v", id, err)
		return apierr.ErrInternal()
	}
	if project == nil {
		return apierr.ErrNotFound()
	}

	var fields map[string]string
	if err := c.Decode(&fields); err != nil {
		return apierr.ErrBadRequest()
	}
	for key, val := range fields {
		if !allowedProjectFields[key] {
			return apierr.ErrBadRequest()
		}
		if key == "title" && len(val) > 255 {
			return apierr.ErrBadRequest()
		}
		if key == "tagline" && len(val) > 255 {
			return apierr.ErrBadRequest()
		}
		if key == "description" && len(val) > 5000 {
			return apierr.ErrBadRequest()
		}
	}

	if err := i18n.UpsertEntityTranslationFields(c.R.Context(), c.DB().DB, common.EntityTypeProject, id, locale, fields); err != nil {
		h.log.Printf("admin: upsert project translation %q/%q: %v", id, locale, err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}
