package admin

import (
	"database/sql"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/handlers/common"
	"github.com/SteVio89/stevio-home/i18n"
	"github.com/SteVio89/stevio-home/pathutil"
	"github.com/SteVio89/stevio-home/validate"
)

// commerceRequest is the optional commerce attachment included in
// AdminCreateProject. Empty bundle_id (zero value) means "no commerce".
type commerceRequest struct {
	BundleID     string `json:"bundle_id"`
	PriceCents   int    `json:"price_cents"`
	PurchaseMode string `json:"purchase_mode"`
	TaxCategory  string `json:"tax_category"`
}

// resolveDetailPage enforces invariants documented in the plan:
//   - Commerce attached → has_detail_page = true (forced)
//   - external_url set  → has_detail_page = false (forced)
//   - Neither           → admin-controlled (use the requested value)
func resolveDetailPage(externalURL *string, hasCommerce, requested bool) bool {
	if hasCommerce {
		return true
	}
	if externalURL != nil && *externalURL != "" {
		return false
	}
	return requested
}

func (h *AdminHandler) AdminListProjects(c *app.Ctx) error {
	ctx := c.R.Context()
	projects, err := queries.ListProjectsAdmin(ctx, c.DB().DB)
	if err != nil {
		h.log.Printf("admin: list projects: %v", err)
		return apierr.ErrInternal()
	}

	// Populate translatable fields from default-locale translations.
	defaultLocale := c.Locales().Default(ctx)
	if tm, err := i18n.GetEntityTranslationsForLocale(ctx, c.DB().DB, common.EntityTypeProject, defaultLocale); err == nil {
		for i := range projects {
			o := i18n.NewOverlay(tm[projects[i].ID])
			o.Apply("title", &projects[i].Title)
			o.Apply("tagline", &projects[i].Tagline)
			o.Apply("description", &projects[i].Description)
		}
	}

	return c.JSON(http.StatusOK, projects)
}

// AdminCreateProject creates a new project, optionally with a commerce
// attachment in the same transaction. When commerce is present, has_detail_page
// is forced to true; when external_url is present, it is forced to false.
func (h *AdminHandler) AdminCreateProject(c *app.Ctx) error {
	var req struct {
		Slug          string           `json:"slug"`
		ExternalURL   *string          `json:"external_url"`
		ImageURL      string           `json:"image_url"`
		HasDetailPage bool             `json:"has_detail_page"`
		Title         string           `json:"title"`
		Tagline       string           `json:"tagline"`
		Description   string           `json:"description"`
		Commerce      *commerceRequest `json:"commerce"`
	}
	if err := c.Decode(&req); err != nil {
		return apierr.ErrBadRequest()
	}
	if req.Title == "" {
		return apierr.ErrBadRequest()
	}
	if len(req.Slug) > 255 || len(req.Title) > 255 || len(req.Tagline) > 255 || len(req.Description) > 5000 {
		return apierr.ErrBadRequest()
	}
	if req.ImageURL != "" && !common.IsValidIconURL(req.ImageURL) {
		return apierr.ErrBadRequest()
	}
	if req.ExternalURL != nil && *req.ExternalURL != "" {
		if err := validate.LinkURL(*req.ExternalURL); err != nil {
			return apierr.ErrBadRequest()
		}
	}
	hasCommerce := req.Commerce != nil && req.Commerce.BundleID != ""
	if hasCommerce {
		if !common.IsValidBundleID(req.Commerce.BundleID) {
			return apierr.ErrBadRequest()
		}
		if req.Commerce.PurchaseMode != "" && !queries.IsValidPurchaseMode(req.Commerce.PurchaseMode) {
			return apierr.ErrBadRequest()
		}
		if req.ExternalURL != nil && *req.ExternalURL != "" {
			// commerce + external_url is incoherent; reject.
			return apierr.ErrBadRequest()
		}
	}

	ctx := c.R.Context()
	maxPos, err := queries.MaxProjectPosition(ctx, c.DB().DB)
	if err != nil {
		h.log.Printf("admin: max project position: %v", err)
		return apierr.ErrInternal()
	}

	hasDetailPage := resolveDetailPage(req.ExternalURL, hasCommerce, req.HasDetailPage)
	defaultLocale := c.Locales().Default(ctx)

	var project models.Project
	if err := queries.WithTx(ctx, c.DB().DB, func(tx *sql.Tx) error {
		var terr error
		project, terr = queries.InsertProjectTx(ctx, tx, req.Slug, req.ImageURL, req.ExternalURL, maxPos+1, hasDetailPage)
		if terr != nil {
			return terr
		}
		if terr := queries.UpsertTranslationFieldsTx(ctx, tx, common.EntityTypeProject, project.ID, defaultLocale, map[string]string{
			"title":       req.Title,
			"tagline":     req.Tagline,
			"description": req.Description,
		}); terr != nil {
			return terr
		}
		if hasCommerce {
			a, terr := queries.InsertAppTx(ctx, tx, project.ID, req.Commerce.BundleID, req.Commerce.PriceCents, req.Commerce.PurchaseMode, req.Commerce.TaxCategory)
			if terr != nil {
				return terr
			}
			project.Commerce = a
		}
		return nil
	}); err != nil {
		h.log.Printf("admin: create project: %v", err)
		return apierr.ErrInternal()
	}

	project.Title = req.Title
	project.Tagline = req.Tagline
	project.Description = req.Description

	return c.JSON(http.StatusCreated, project)
}

// AdminUpdateProject updates a project's editable fields. has_detail_page is
// validated against the project's current commerce/external_url state — see
// resolveDetailPage for the invariants.
func (h *AdminHandler) AdminUpdateProject(c *app.Ctx) error {
	id := c.Param("id")
	var req struct {
		Slug          string  `json:"slug"`
		ExternalURL   *string `json:"external_url"`
		ImageURL      string  `json:"image_url"`
		HasDetailPage bool    `json:"has_detail_page"`
		Title         string  `json:"title"`
		Tagline       string  `json:"tagline"`
		Description   string  `json:"description"`
	}
	if err := c.Decode(&req); err != nil {
		return apierr.ErrBadRequest()
	}
	if len(req.Slug) > 255 || len(req.Title) > 255 || len(req.Tagline) > 255 || len(req.Description) > 5000 {
		return apierr.ErrBadRequest()
	}
	if req.ImageURL != "" && !common.IsValidIconURL(req.ImageURL) {
		return apierr.ErrBadRequest()
	}
	if req.ExternalURL != nil && *req.ExternalURL != "" {
		if err := validate.LinkURL(*req.ExternalURL); err != nil {
			return apierr.ErrBadRequest()
		}
	}

	ctx := c.R.Context()
	commerce, err := queries.GetAppByProjectID(ctx, c.DB().DB, id)
	if err != nil {
		h.log.Printf("admin: update project %q: lookup commerce: %v", id, err)
		return apierr.ErrInternal()
	}
	hasCommerce := commerce != nil
	if hasCommerce && req.ExternalURL != nil && *req.ExternalURL != "" {
		// Commerce + external_url is incoherent.
		return apierr.ErrBadRequest()
	}
	hasDetailPage := resolveDetailPage(req.ExternalURL, hasCommerce, req.HasDetailPage)

	defaultLocale := c.Locales().Default(ctx)
	if err := queries.WithTx(ctx, c.DB().DB, func(tx *sql.Tx) error {
		if err := queries.UpdateProjectTx(ctx, tx, id, req.Slug, req.ImageURL, req.ExternalURL, hasDetailPage); err != nil {
			return err
		}
		return queries.UpsertTranslationFieldsTx(ctx, tx, common.EntityTypeProject, id, defaultLocale, map[string]string{
			"title":       req.Title,
			"tagline":     req.Tagline,
			"description": req.Description,
		})
	}); err != nil {
		h.log.Printf("admin: update project %q: %v", id, err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}

func (h *AdminHandler) AdminDeleteProject(c *app.Ctx) error {
	if err := queries.SoftDeleteProject(c.R.Context(), c.DB().DB, c.Param("id")); err != nil {
		h.log.Printf("admin: delete project %q: %v", c.Param("id"), err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}

func (h *AdminHandler) AdminRestoreProject(c *app.Ctx) error {
	if err := queries.RestoreProject(c.R.Context(), c.DB().DB, c.Param("id")); err != nil {
		h.log.Printf("admin: restore project %q: %v", c.Param("id"), err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}

// AdminHardDeleteProject permanently removes a project (and its app, images,
// translations). Refuses with 409 when the project has order history, which must
// be retained. Image files are unlinked from disk after the DB delete commits.
func (h *AdminHandler) AdminHardDeleteProject(c *app.Ctx) error {
	id := c.Param("id")
	filePaths, err := queries.HardDeleteProject(c.R.Context(), c.DB().DB, id,
		common.EntityTypeProject, common.EntityTypeProjectImage)
	if err != nil {
		if errors.Is(err, queries.ErrProjectHasOrders) {
			return apierr.ErrConflict()
		}
		if errors.Is(err, sql.ErrNoRows) {
			return apierr.ErrNotFound()
		}
		h.log.Printf("admin: permanently delete project %q: %v", id, err)
		return apierr.ErrInternal()
	}

	// Unlink image files (same pattern as AdminDeleteProjectImage). Best-effort:
	// the DB rows are already gone, so a stray file is harmless.
	for _, fp := range filePaths {
		fullPath, err := pathutil.SafePath(h.cfg.AssetsDir, fp)
		if err != nil {
			h.log.Printf("admin: safePath hard-delete project image %q: %v", fp, err)
			continue
		}
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			h.log.Printf("admin: remove project image file %q: %v", fullPath, err)
		}
	}
	return c.NoContent()
}

// AdminUploadProjectImage uploads the project's primary square card image
// (shown on Landing). Stored at /media/projects/<id>.<ext> and persisted in
// projects.image_url.
func (h *AdminHandler) AdminUploadProjectImage(c *app.Ctx) error {
	id, err := pathutil.SanitizePathSegment(c.Param("id"))
	if err != nil {
		return apierr.ErrBadRequest()
	}

	c.R.Body = http.MaxBytesReader(c.W, c.R.Body, common.MaxImageSize)
	if err := c.R.ParseMultipartForm(common.MaxImageSize); err != nil {
		return apierr.ErrBadRequest()
	}

	file, hdr, err := c.R.FormFile("file")
	if err != nil {
		return apierr.ErrBadRequest()
	}
	defer func() { _ = file.Close() }()

	ext := filepath.Ext(hdr.Filename)
	if !common.AllowedImageExt(ext) {
		return apierr.ErrBadRequest()
	}

	dir, err := pathutil.SafePath(h.cfg.AssetsDir, "projects")
	if err != nil {
		h.log.Printf("admin: safePath projects: %v", err)
		return apierr.ErrBadRequest()
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		h.log.Printf("admin: mkdir projects: %v", err)
		return apierr.ErrInternal()
	}

	filename := id + strings.ToLower(ext)
	dst, err := pathutil.SafePath(dir, filename)
	if err != nil {
		h.log.Printf("admin: safePath project image dst: %v", err)
		return apierr.ErrBadRequest()
	}
	if err := common.WriteFile(dst, file); err != nil {
		h.log.Printf("admin: write project image: %v", err)
		return apierr.ErrInternal()
	}

	imageURL := "/media/projects/" + filename
	if err := queries.UpdateProjectImageURL(c.R.Context(), c.DB().DB, id, imageURL); err != nil {
		h.log.Printf("admin: update project image url: %v", err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusOK, map[string]string{"image_url": imageURL})
}

func (h *AdminHandler) AdminReorderProjects(c *app.Ctx) error {
	var req struct {
		Positions map[string]int `json:"positions"`
	}
	if err := c.Decode(&req); err != nil || len(req.Positions) == 0 {
		return apierr.ErrBadRequest()
	}

	if err := queries.WithTx(c.R.Context(), c.DB().DB, func(tx *sql.Tx) error {
		for id, pos := range req.Positions {
			if err := queries.UpdateProjectPosition(c.R.Context(), tx, id, pos); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		h.log.Printf("admin: reorder projects: %v", err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}

// ── Project gallery images (replaces app screenshots) ──────────────────────

// AdminReorderProjectImages updates positions for images attached to one project.
// Each ID in the map must belong to the project (enforced per-row).
func (h *AdminHandler) AdminReorderProjectImages(c *app.Ctx) error {
	var req struct {
		Positions map[string]int `json:"positions"`
	}
	if err := c.Decode(&req); err != nil || len(req.Positions) == 0 {
		return apierr.ErrBadRequest()
	}

	projectID := c.Param("id")
	if err := queries.WithTx(c.R.Context(), c.DB().DB, func(tx *sql.Tx) error {
		for id, pos := range req.Positions {
			if err := queries.UpdateProjectImagePositionForProject(c.R.Context(), tx, id, projectID, pos); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		h.log.Printf("admin: reorder project images: %v", err)
		return apierr.ErrInternal()
	}

	return c.NoContent()
}

// AdminDeleteProjectImage hard-deletes a project image, both from the DB and
// from the assets directory on disk.
func (h *AdminHandler) AdminDeleteProjectImage(c *app.Ctx) error {
	projectID := c.Param("id")
	imgID := c.Param("imgId")

	filePath, err := queries.DeleteProjectImageForProject(c.R.Context(), c.DB().DB, imgID, projectID)
	if err != nil {
		h.log.Printf("admin: delete project image %q: %v", imgID, err)
		return apierr.ErrInternal()
	}

	if filePath != "" {
		fullPath, err := pathutil.SafePath(h.cfg.AssetsDir, filePath)
		if err != nil {
			h.log.Printf("admin: safePath delete project image: %v", err)
		} else {
			if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				h.log.Printf("admin: remove project image file %q: %v", fullPath, err)
			}
		}
	}

	return c.NoContent()
}

// AdminGetProjectImageTranslations returns a per-image map of locale → fields,
// for the alt-text translation editor.
func (h *AdminHandler) AdminGetProjectImageTranslations(c *app.Ctx) error {
	projectID := c.Param("id")
	imgs, err := queries.ListProjectImages(c.R.Context(), c.DB().DB, projectID)
	if err != nil {
		h.log.Printf("admin: get project image translations: list %q: %v", projectID, err)
		return apierr.ErrInternal()
	}

	supported := c.Locales().Supported(c.R.Context())
	result := make(map[string]map[string]map[string]string, len(imgs))
	for _, img := range imgs {
		tm, err := i18n.GetEntityTranslations(c.R.Context(), c.DB().DB, common.EntityTypeProjectImage, img.ID)
		if err != nil {
			h.log.Printf("admin: get project image translations %q: %v", img.ID, err)
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
		result[img.ID] = locMap
	}
	return c.JSON(http.StatusOK, result)
}

// AdminUpsertProjectImageTranslation sets the alt_text translation for one
// (image, locale) pair. Image must belong to the URL's project.
func (h *AdminHandler) AdminUpsertProjectImageTranslation(c *app.Ctx) error {
	projectID := c.Param("id")
	imgID := c.Param("imgId")
	loc := c.Param("locale")

	if !c.Locales().IsSupported(c.R.Context(), loc) {
		return apierr.ErrBadRequest()
	}

	imgs, err := queries.ListProjectImages(c.R.Context(), c.DB().DB, projectID)
	if err != nil {
		h.log.Printf("admin: upsert project image translation: list %q: %v", projectID, err)
		return apierr.ErrInternal()
	}
	found := false
	for _, im := range imgs {
		if im.ID == imgID {
			found = true
			break
		}
	}
	if !found {
		return apierr.ErrNotFound()
	}

	var req struct {
		AltText string `json:"alt_text"`
	}
	if err := c.Decode(&req); err != nil {
		return apierr.ErrBadRequest()
	}
	if len(req.AltText) > 500 {
		return apierr.ErrBadRequest()
	}
	if err := i18n.UpsertEntityTranslation(c.R.Context(), c.DB().DB, common.EntityTypeProjectImage, imgID, loc, "alt_text", req.AltText); err != nil {
		h.log.Printf("admin: upsert project image translation %q/%q: %v", imgID, loc, err)
		return apierr.ErrInternal()
	}
	c.W.WriteHeader(http.StatusCreated)
	return nil
}

// ── Commerce attach / detach ───────────────────────────────────────────────

// AdminAttachCommerce creates a commerce row for an existing showcase project,
// flipping has_detail_page on (since commerce always implies a detail page).
func (h *AdminHandler) AdminAttachCommerce(c *app.Ctx) error {
	projectID, err := pathutil.SanitizePathSegment(c.Param("id"))
	if err != nil {
		return apierr.ErrBadRequest()
	}
	var req commerceRequest
	if err := c.Decode(&req); err != nil || req.BundleID == "" {
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
	project, err := queries.GetProjectByID(ctx, c.DB().DB, projectID)
	if err != nil {
		h.log.Printf("admin: attach commerce: lookup project %q: %v", projectID, err)
		return apierr.ErrInternal()
	}
	if project == nil || project.DeletedAt != nil {
		return apierr.ErrNotFound()
	}
	if project.ExternalURL != nil && *project.ExternalURL != "" {
		// External + commerce is incoherent.
		return apierr.ErrBadRequest()
	}
	existing, err := queries.GetAppByProjectID(ctx, c.DB().DB, projectID)
	if err != nil {
		h.log.Printf("admin: attach commerce: lookup existing %q: %v", projectID, err)
		return apierr.ErrInternal()
	}
	if existing != nil {
		return apierr.ErrConflict()
	}

	var created *models.App
	if err := queries.WithTx(ctx, c.DB().DB, func(tx *sql.Tx) error {
		var terr error
		created, terr = queries.InsertAppTx(ctx, tx, projectID, req.BundleID, req.PriceCents, req.PurchaseMode, req.TaxCategory)
		if terr != nil {
			return terr
		}
		// Flip has_detail_page on (commerce always has a detail page).
		return queries.UpdateProjectTx(ctx, tx, projectID, project.Slug, project.ImageURL, project.ExternalURL, true)
	}); err != nil {
		h.log.Printf("admin: attach commerce %q: %v", projectID, err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusCreated, created)
}

// AdminDetachCommerce soft-deletes the commerce row for a project. The project
// itself stays intact; has_detail_page stays as-is (admin can flip it via update).
func (h *AdminHandler) AdminDetachCommerce(c *app.Ctx) error {
	projectID, err := pathutil.SanitizePathSegment(c.Param("id"))
	if err != nil {
		return apierr.ErrBadRequest()
	}

	ctx := c.R.Context()
	commerce, err := queries.GetAppByProjectID(ctx, c.DB().DB, projectID)
	if err != nil {
		h.log.Printf("admin: detach commerce: lookup %q: %v", projectID, err)
		return apierr.ErrInternal()
	}
	if commerce == nil {
		return apierr.ErrNotFound()
	}
	if err := queries.SoftDeleteApp(ctx, c.DB().DB, commerce.ID); err != nil {
		h.log.Printf("admin: detach commerce %q: %v", projectID, err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}
