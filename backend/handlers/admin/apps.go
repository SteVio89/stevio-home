package admin

import (
	"database/sql"
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/handlers/common"
	"github.com/SteVio89/stevio-home/i18n"
	"github.com/SteVio89/stevio-home/pathutil"
)

// AdminCreateApp creates a commerce attachment for an existing project. The
// project must exist and not yet have a commerce row (1:1 partial unique index).
// Display text and image live on the parent project — this endpoint is
// commerce-only.
func (h *AdminHandler) AdminCreateApp(c *app.Ctx) error {
	var req struct {
		ProjectID    string `json:"project_id"`
		BundleID     string `json:"bundle_id"`
		PriceCents   int    `json:"price_cents"`
		PurchaseMode string `json:"purchase_mode"`
		TaxCategory  string `json:"tax_category"`
	}
	if err := c.Decode(&req); err != nil || req.ProjectID == "" || req.BundleID == "" {
		return apierr.ErrBadRequest()
	}
	if len(req.ProjectID) > 128 || len(req.BundleID) > 255 {
		return apierr.ErrBadRequest()
	}
	if !common.IsValidBundleID(req.BundleID) {
		return apierr.ErrBadRequest()
	}
	if req.PurchaseMode != "" && !queries.IsValidPurchaseMode(req.PurchaseMode) {
		return apierr.ErrBadRequest()
	}
	if req.TaxCategory != "" && !queries.IsValidTaxCategory(req.TaxCategory) {
		return apierr.ErrBadRequest()
	}

	ctx := c.R.Context()

	// Verify the project exists and has no live commerce attached.
	project, err := queries.GetProjectByID(ctx, c.DB().DB, req.ProjectID)
	if err != nil {
		h.log.Printf("admin: create app: lookup project %q: %v", req.ProjectID, err)
		return apierr.ErrInternal()
	}
	if project == nil || project.DeletedAt != nil {
		return apierr.ErrNotFound()
	}
	existing, err := queries.GetAppByProjectID(ctx, c.DB().DB, req.ProjectID)
	if err != nil {
		h.log.Printf("admin: create app: lookup existing %q: %v", req.ProjectID, err)
		return apierr.ErrInternal()
	}
	if existing != nil {
		return apierr.ErrConflict()
	}

	var created *models.App
	if err := queries.WithTx(ctx, c.DB().DB, func(tx *sql.Tx) error {
		var err error
		created, err = queries.InsertAppTx(ctx, tx, req.ProjectID, req.BundleID, req.PriceCents, req.PurchaseMode, req.TaxCategory)
		return err
	}); err != nil {
		h.log.Printf("admin: create app: %v", err)
		return apierr.ErrInternal()
	}

	return c.JSON(http.StatusCreated, created)
}

// AdminUpdateApp adjusts pricing/purchase_mode on an existing commerce row.
// project_id and bundle_id are immutable after create.
func (h *AdminHandler) AdminUpdateApp(c *app.Ctx) error {
	id := c.Param("id")
	var req struct {
		PriceCents   int    `json:"price_cents"`
		PurchaseMode string `json:"purchase_mode"`
		TaxCategory  string `json:"tax_category"`
	}
	if err := c.Decode(&req); err != nil {
		return apierr.ErrBadRequest()
	}
	if req.PurchaseMode != "" && !queries.IsValidPurchaseMode(req.PurchaseMode) {
		return apierr.ErrBadRequest()
	}
	if req.TaxCategory != "" && !queries.IsValidTaxCategory(req.TaxCategory) {
		return apierr.ErrBadRequest()
	}
	if err := queries.UpdateApp(c.R.Context(), c.DB().DB, id, req.PriceCents, req.PurchaseMode, req.TaxCategory); err != nil {
		h.log.Printf("admin: update app %q: %v", id, err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}

// AdminListApps returns commerce apps with their parent project's slug + title
// for the admin table view.
func (h *AdminHandler) AdminListApps(c *app.Ctx) error {
	ctx := c.R.Context()
	apps, err := queries.ListAppsAdmin(ctx, c.DB().DB)
	if err != nil {
		h.log.Printf("admin: list apps: %v", err)
		return apierr.ErrInternal()
	}

	// Populate translatable project titles from default-locale translations.
	defaultLocale := c.Locales().Default(ctx)
	if tm, err := i18n.GetEntityTranslationsForLocale(ctx, c.DB().DB, common.EntityTypeProject, defaultLocale); err == nil {
		for i := range apps {
			if title, ok := tm[apps[i].ProjectID]["title"]; ok {
				apps[i].ProjectTitle = title
			}
		}
	}

	return c.JSON(http.StatusOK, apps)
}

func (h *AdminHandler) AdminDeleteApp(c *app.Ctx) error {
	id, err := pathutil.SanitizePathSegment(c.Param("id"))
	if err != nil {
		return apierr.ErrBadRequest()
	}
	if err := queries.SoftDeleteApp(c.R.Context(), c.DB().DB, id); err != nil {
		h.log.Printf("admin: soft delete app %q: %v", id, err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}

func (h *AdminHandler) AdminRestoreApp(c *app.Ctx) error {
	id, err := pathutil.SanitizePathSegment(c.Param("id"))
	if err != nil {
		return apierr.ErrBadRequest()
	}
	if err := queries.RestoreApp(c.R.Context(), c.DB().DB, id); err != nil {
		h.log.Printf("admin: restore app %q: %v", id, err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}

func (h *AdminHandler) AdminListVersions(c *app.Ctx) error {
	id := c.Param("id")
	versions, err := queries.ListVersionsByAppID(c.R.Context(), c.DB().DB, id)
	if err != nil {
		h.log.Printf("admin: list versions %q: %v", id, err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusOK, versions)
}

func (h *AdminHandler) AdminCreateVersion(c *app.Ctx) error {
	id := c.Param("id")
	var req struct {
		Version      string `json:"version"`
		ReleaseNotes string `json:"release_notes"`
	}
	if err := c.Decode(&req); err != nil || req.Version == "" {
		return apierr.ErrBadRequest()
	}
	if len(req.Version) > 64 || len(req.ReleaseNotes) > 10000 {
		return apierr.ErrBadRequest()
	}
	ctx := c.R.Context()
	var v *models.AppVersion
	if err := queries.WithTx(ctx, c.DB().DB, func(tx *sql.Tx) error {
		var err error
		v, err = queries.InsertAppVersionTx(ctx, tx, id, req.Version, "", "")
		if err != nil {
			return err
		}
		if req.ReleaseNotes != "" {
			defaultLocale := c.Locales().Default(ctx)
			return queries.UpsertTranslationTx(ctx, tx, common.EntityTypeVersion, v.ID, defaultLocale, "release_notes", req.ReleaseNotes)
		}
		return nil
	}); err != nil {
		h.log.Printf("admin: create version %q: %v", id, err)
		return apierr.ErrInternal()
	}

	v.ReleaseNotes = req.ReleaseNotes
	return c.JSON(http.StatusCreated, v)
}
