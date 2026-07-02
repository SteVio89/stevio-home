package sdk

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/pathutil"
)

// ServeDownload validates a one-time download token and serves the file.
func (h *SDKHandler) ServeDownload(c *app.Ctx) error {
	token := c.R.URL.Query().Get("token")
	if token == "" {
		return apierr.ErrBadRequest()
	}

	appID, err := queries.ConsumeDownloadToken(c.R.Context(), c.DB().DB, token)
	if err != nil {
		switch {
		case errors.Is(err, queries.ErrDownloadTokenUsed):
			return apierr.ErrDownloadTokenUsed
		case errors.Is(err, queries.ErrDownloadTokenExpired):
			return apierr.ErrDownloadTokenExpired
		default:
			return apierr.ErrNotFound()
		}
	}

	latest, err := queries.GetLatestVersion(c.R.Context(), c.DB().DB, appID)
	if err != nil {
		h.log.Printf("download: get latest version for %q: %v", appID, err)
		return apierr.ErrInternal()
	}
	if latest == nil || latest.FilePath == "" {
		return apierr.ErrNotFound()
	}

	filePath, err := pathutil.SafePath(h.cfg.AppsDir, latest.FilePath)
	if err != nil {
		h.log.Printf("download: path traversal blocked: %v", err)
		return apierr.ErrNotFound()
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		h.log.Printf("download: file not found: %s", filePath)
		return apierr.ErrNotFound()
	}

	safeFile := pathutil.SanitizeFilename(filepath.Base(filePath))
	c.W.Header().Set("Content-Disposition", `attachment; filename="`+safeFile+`"`)
	c.W.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(c.W, c.R, filePath)
	return nil
}
