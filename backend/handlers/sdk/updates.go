package sdk

import (
	"net/http"
	"time"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/handlers/common"
	"github.com/SteVio89/stevio-home/i18n"
)

type updateCheckResponse struct {
	LatestVersion   string `json:"latest_version"`
	DownloadURL     string `json:"download_url"`
	ReleaseNotes    string `json:"release_notes"`
	UpdateAvailable bool   `json:"update_available"`
}

func (h *SDKHandler) CheckForUpdate(c *app.Ctx) error {
	bundleID := c.R.URL.Query().Get("bundle_id")
	currentVersion := c.R.URL.Query().Get("version")
	licenseKey := c.R.URL.Query().Get("license_key")
	if bundleID == "" || currentVersion == "" || licenseKey == "" {
		return apierr.ErrBadRequest()
	}

	ctx := c.R.Context()
	appID, err := queries.GetAppIDByBundleID(ctx, c.DB().DB, bundleID)
	if err != nil {
		h.log.Printf("updates: resolve bundle_id: %v", err)
		return apierr.ErrInternal()
	}
	if appID == "" {
		return apierr.ErrNotFound()
	}

	defaultLocale := c.Locales().Default(ctx)
	license, err := queries.GetLicenseByKey(ctx, c.DB().DB, licenseKey, defaultLocale)
	if err != nil {
		h.log.Printf("updates: get license: %v", err)
		return apierr.ErrInternal()
	}
	if license == nil || license.AppID != appID {
		return apierr.ErrLicenseInvalid
	}
	if license.Revoked {
		return apierr.ErrLicenseRevoked
	}

	latest, err := queries.GetLatestVersion(ctx, c.DB().DB, appID)
	if err != nil {
		h.log.Printf("updates: get latest version: %v", err)
		return apierr.ErrInternal()
	}
	if latest == nil {
		return apierr.ErrNotFound()
	}

	downloadURL := latest.DownloadURL

	if latest.FilePath != "" {
		token, err := common.GenerateToken()
		if err != nil {
			h.log.Printf("updates: generate token: %v", err)
			return apierr.ErrInternal()
		}
		downloadTTLMin := c.Settings().GetInt(ctx, "download_token_ttl_min", 15)
		expiresAt := time.Now().UTC().Add(time.Duration(downloadTTLMin) * time.Minute)
		if err := queries.InsertDownloadToken(ctx, c.DB().DB, token, license.ID, appID, expiresAt); err != nil {
			h.log.Printf("updates: insert download token: %v", err)
			return apierr.ErrInternal()
		}
		downloadURL = h.cfg.BaseURL + "/api/downloads/file?token=" + token
	}

	// Apply default locale translations first, then overlay requested locale.
	loc := c.Lang()
	if fields, err := i18n.GetEntityTranslation(ctx, c.DB().DB, common.EntityTypeVersion, latest.ID, defaultLocale); err == nil {
		i18n.NewOverlay(fields).Apply("release_notes", &latest.ReleaseNotes)
	}
	if loc != defaultLocale {
		if fields, err := i18n.GetEntityTranslation(ctx, c.DB().DB, common.EntityTypeVersion, latest.ID, loc); err == nil {
			i18n.NewOverlay(fields).Apply("release_notes", &latest.ReleaseNotes)
		}
	}

	return c.JSON(http.StatusOK, updateCheckResponse{
		LatestVersion:   latest.Version,
		DownloadURL:     downloadURL,
		ReleaseNotes:    latest.ReleaseNotes,
		UpdateAvailable: latest.Version != currentVersion,
	})
}
