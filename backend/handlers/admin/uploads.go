package admin

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/handlers/common"
	"github.com/SteVio89/stevio-home/pathutil"
)

// AdminUploadBinary uploads a downloadable artifact for an app version.
// Stored under {AppsDir}/{appID}/<filename>; recorded as a relative path on
// the app_versions row (download_url defaulted to /api/downloads/file).
func (h *AdminHandler) AdminUploadBinary(c *app.Ctx) error {
	appID, err := pathutil.SanitizePathSegment(c.Param("id"))
	if err != nil {
		return apierr.ErrBadRequest()
	}
	versionID := c.Param("vid")

	c.R.Body = http.MaxBytesReader(c.W, c.R.Body, common.MaxBinarySize)
	if err := c.R.ParseMultipartForm(32 << 20); err != nil {
		return apierr.ErrBadRequest()
	}

	file, hdr, err := c.R.FormFile("file")
	if err != nil {
		return apierr.ErrBadRequest()
	}
	defer func() { _ = file.Close() }()

	ext := filepath.Ext(hdr.Filename)
	if !common.AllowedBinaryExt(ext) {
		return apierr.ErrBadRequest()
	}

	dir, err := pathutil.SafePath(h.cfg.AppsDir, appID)
	if err != nil {
		h.log.Printf("admin: safePath apps dir: %v", err)
		return apierr.ErrBadRequest()
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		h.log.Printf("admin: mkdir apps: %v", err)
		return apierr.ErrInternal()
	}

	safeFilename := pathutil.SanitizeFilename(filepath.Base(hdr.Filename))
	if safeFilename == "download" {
		safeFilename = "app" + strings.ToLower(ext)
	}
	dst, err := pathutil.SafePath(dir, safeFilename)
	if err != nil {
		h.log.Printf("admin: safePath binary dst: %v", err)
		return apierr.ErrBadRequest()
	}
	if err := common.WriteFile(dst, file); err != nil {
		h.log.Printf("admin: write binary: %v", err)
		return apierr.ErrInternal()
	}

	relPath := appID + "/" + safeFilename
	downloadURL := "/api/downloads/file"
	if err := queries.UpdateAppVersionFilePath(c.R.Context(), c.DB().DB, versionID, relPath, downloadURL); err != nil {
		h.log.Printf("admin: update version file path: %v", err)
		return apierr.ErrInternal()
	}

	return c.JSON(http.StatusOK, map[string]string{"file_path": relPath})
}

// AdminUploadProjectGalleryImage uploads a single image to the project's
// gallery (project_images), returning the inserted row. Replaces the old
// per-app screenshot upload — same UX, but keyed by project.
func (h *AdminHandler) AdminUploadProjectGalleryImage(c *app.Ctx) error {
	projectID, err := pathutil.SanitizePathSegment(c.Param("id"))
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

	altText := c.R.FormValue("alt_text")

	dir, err := pathutil.SafePath(h.cfg.AssetsDir, "project_images", projectID)
	if err != nil {
		h.log.Printf("admin: safePath project_images dir: %v", err)
		return apierr.ErrBadRequest()
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		h.log.Printf("admin: mkdir project_images: %v", err)
		return apierr.ErrInternal()
	}

	token, err := common.GenerateToken()
	if err != nil {
		h.log.Printf("admin: generate project image id: %v", err)
		return apierr.ErrInternal()
	}
	filename := token[:16] + strings.ToLower(ext)
	dst, err := pathutil.SafePath(dir, filename)
	if err != nil {
		h.log.Printf("admin: safePath project image dst: %v", err)
		return apierr.ErrBadRequest()
	}
	if err := common.WriteFile(dst, file); err != nil {
		h.log.Printf("admin: write project image: %v", err)
		return apierr.ErrInternal()
	}

	filePath := "project_images/" + projectID + "/" + filename
	url := "/media/project_images/" + projectID + "/" + filename

	maxPos, err := queries.GetProjectImageMaxPosition(c.R.Context(), c.DB().DB, projectID)
	if err != nil {
		h.log.Printf("admin: get max position: %v", err)
		return apierr.ErrInternal()
	}

	ctx := c.R.Context()
	var img *models.ProjectImage
	if err := queries.WithTx(ctx, c.DB().DB, func(tx *sql.Tx) error {
		var ierr error
		img, ierr = queries.InsertProjectImageTx(ctx, tx, projectID, url, filePath, maxPos+1)
		if ierr != nil {
			return ierr
		}
		if altText != "" {
			defaultLocale := c.Locales().Default(ctx)
			return queries.UpsertTranslationTx(ctx, tx, common.EntityTypeProjectImage, img.ID, defaultLocale, "alt_text", altText)
		}
		return nil
	}); err != nil {
		h.log.Printf("admin: insert project image: %v", err)
		return apierr.ErrInternal()
	}

	img.AltText = altText
	return c.JSON(http.StatusCreated, img)
}
