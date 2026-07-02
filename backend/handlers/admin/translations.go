package admin

import (
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/handlers/common"
	"github.com/SteVio89/stevio-home/i18n"
)

// AdminGetAppTranslations returns all locales' translation rows for a commerce
// app. Post-refactor, the only translatable field on apps is system_requirements
// (display text moved to the parent project).
func (h *AdminHandler) AdminGetAppTranslations(c *app.Ctx) error {
	appID := c.Param("id")
	translations, err := i18n.GetEntityTranslations(c.R.Context(), c.DB().DB, common.EntityTypeApp, appID)
	if err != nil {
		h.log.Printf("admin: get translations %q: %v", appID, err)
		return apierr.ErrInternal()
	}
	supported := c.Locales().Supported(c.R.Context())
	result := make(map[string]map[string]string, len(supported))
	for _, loc := range supported {
		if t, ok := translations[loc]; ok {
			result[loc] = t
		} else {
			result[loc] = map[string]string{}
		}
	}
	return c.JSON(http.StatusOK, result)
}

func (h *AdminHandler) AdminGetVersionTranslations(c *app.Ctx) error {
	appID := c.Param("id")
	versions, err := queries.ListVersionsByAppID(c.R.Context(), c.DB().DB, appID)
	if err != nil {
		h.log.Printf("admin: get version translations: list versions %q: %v", appID, err)
		return apierr.ErrInternal()
	}

	supported := c.Locales().Supported(c.R.Context())
	result := make(map[string]map[string]map[string]string, len(versions))
	for _, ver := range versions {
		tm, err := i18n.GetEntityTranslations(c.R.Context(), c.DB().DB, common.EntityTypeVersion, ver.ID)
		if err != nil {
			h.log.Printf("admin: get version translations %q: %v", ver.ID, err)
			return apierr.ErrInternal()
		}
		locMap := make(map[string]map[string]string, len(supported))
		for _, loc := range supported {
			if t, ok := tm[loc]; ok {
				locMap[loc] = t
			} else {
				locMap[loc] = map[string]string{}
			}
		}
		result[ver.ID] = locMap
	}
	return c.JSON(http.StatusOK, result)
}

func (h *AdminHandler) AdminUpsertVersionTranslation(c *app.Ctx) error {
	appID := c.Param("id")
	vid := c.Param("vid")
	loc := c.Param("locale")

	if !c.Locales().IsSupported(c.R.Context(), loc) {
		return apierr.ErrBadRequest()
	}

	versions, err := queries.ListVersionsByAppID(c.R.Context(), c.DB().DB, appID)
	if err != nil {
		h.log.Printf("admin: upsert version translation: list versions %q: %v", appID, err)
		return apierr.ErrInternal()
	}
	found := false
	for _, v := range versions {
		if v.ID == vid {
			found = true
			break
		}
	}
	if !found {
		return apierr.ErrNotFound()
	}

	var req struct {
		ReleaseNotes string `json:"release_notes"`
	}
	if err := c.Decode(&req); err != nil {
		return apierr.ErrBadRequest()
	}
	if len(req.ReleaseNotes) > 10000 {
		return apierr.ErrBadRequest()
	}
	if err := i18n.UpsertEntityTranslation(c.R.Context(), c.DB().DB, common.EntityTypeVersion, vid, loc, "release_notes", req.ReleaseNotes); err != nil {
		h.log.Printf("admin: upsert version translation %q/%q: %v", vid, loc, err)
		return apierr.ErrInternal()
	}
	c.W.WriteHeader(http.StatusCreated)
	return nil
}

// AdminUpsertAppTranslation accepts only the system_requirements field on the
// app entity. Display text (name/tagline/description) moved to the project
// translation endpoints — those keys are rejected with 400.
func (h *AdminHandler) AdminUpsertAppTranslation(c *app.Ctx) error {
	appID := c.Param("id")
	loc := c.Param("locale")

	if !c.Locales().IsSupported(c.R.Context(), loc) {
		return apierr.ErrBadRequest()
	}

	app, err := queries.GetAppByID(c.R.Context(), c.DB().DB, appID)
	if err != nil {
		h.log.Printf("admin: upsert app translation: get app %q: %v", appID, err)
		return apierr.ErrInternal()
	}
	if app == nil {
		return apierr.ErrNotFound()
	}

	var req struct {
		SystemRequirements string `json:"system_requirements"`
	}
	if err := c.Decode(&req); err != nil {
		return apierr.ErrBadRequest()
	}
	if len(req.SystemRequirements) > 5000 {
		return apierr.ErrBadRequest()
	}
	if err := i18n.UpsertEntityTranslation(c.R.Context(), c.DB().DB, common.EntityTypeApp, appID, loc, "system_requirements", req.SystemRequirements); err != nil {
		h.log.Printf("admin: upsert translation %q/%q: %v", appID, loc, err)
		return apierr.ErrInternal()
	}
	c.W.WriteHeader(http.StatusCreated)
	return nil
}
