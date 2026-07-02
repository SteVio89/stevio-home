package account

import (
	"net/http"
	"time"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/handlers/common"
)

type licenseWithActivations struct {
	models.License
	Activations []models.Activation `json:"activations"`
}

func (h *AccountHandler) GetLicenses(c *app.Ctx) error {
	email := c.User().EmailHash

	defaultLocale := c.Locales().Default(c.R.Context())
	licenses, err := queries.GetLicensesByEmail(c.R.Context(), c.DB().DB, email, defaultLocale)
	if err != nil {
		h.log.Printf("account: get licenses: %v", err)
		return apierr.ErrInternal()
	}

	out := make([]licenseWithActivations, 0, len(licenses))
	for _, l := range licenses {
		acts, err := queries.GetActivationsByLicenseID(c.R.Context(), c.DB().DB, l.ID)
		if err != nil {
			h.log.Printf("account: get activations: %v", err)
			return apierr.ErrInternal()
		}
		if acts == nil {
			acts = []models.Activation{}
		}
		out = append(out, licenseWithActivations{License: l, Activations: acts})
	}

	return c.JSON(http.StatusOK, out)
}

func (h *AccountHandler) GetOrders(c *app.Ctx) error {
	email := c.User().EmailHash

	defaultLocale := c.Locales().Default(c.R.Context())
	orders, err := queries.GetUserOrders(c.R.Context(), c.DB().DB, email, defaultLocale)
	if err != nil {
		h.log.Printf("account: get orders: %v", err)
		return apierr.ErrInternal()
	}
	if orders == nil {
		orders = []queries.UserOrder{}
	}
	return c.JSON(http.StatusOK, orders)
}

type renameRequest struct {
	DeviceLabel string `json:"device_label"`
}

func (h *AccountHandler) RenameDevice(c *app.Ctx) error {
	id := c.Param("id")
	email := c.User().EmailHash

	var req renameRequest
	if err := c.Decode(&req); err != nil || req.DeviceLabel == "" {
		return apierr.ErrBadRequest()
	}
	if len(req.DeviceLabel) > 255 {
		return apierr.ErrBadRequest()
	}

	act, err := queries.GetActivationByIDAndEmail(c.R.Context(), c.DB().DB, id, email)
	if err != nil {
		h.log.Printf("account: get activation: %v", err)
		return apierr.ErrInternal()
	}
	if act == nil {
		return apierr.ErrNotFound()
	}

	if err := queries.UpdateDeviceLabel(c.R.Context(), c.DB().DB, id, req.DeviceLabel); err != nil {
		h.log.Printf("account: update label: %v", err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "updated"})
}

// CreateDownloadToken handles POST /api/account/licenses/{licenseId}/download-token.
func (h *AccountHandler) CreateDownloadToken(c *app.Ctx) error {
	licenseID := c.Param("licenseId")
	email := c.User().EmailHash

	defaultLocale := c.Locales().Default(c.R.Context())
	licenses, err := queries.GetLicensesByEmail(c.R.Context(), c.DB().DB, email, defaultLocale)
	if err != nil {
		h.log.Printf("download-token: get licenses for %q: %v", email, err)
		return apierr.ErrInternal()
	}

	var appID string
	var revoked bool
	for _, l := range licenses {
		if l.ID == licenseID {
			appID = l.AppID
			revoked = l.Revoked
			break
		}
	}
	if appID == "" {
		return apierr.ErrNotFound()
	}
	if revoked {
		return apierr.ErrLicenseRevoked
	}

	token, err := common.GenerateToken()
	if err != nil {
		h.log.Printf("download-token: generate token: %v", err)
		return apierr.ErrInternal()
	}

	downloadTTLMin := c.Settings().GetInt(c.R.Context(), "download_token_ttl_min", 15)
	expiresAt := time.Now().UTC().Add(time.Duration(downloadTTLMin) * time.Minute)
	if err := queries.InsertDownloadToken(c.R.Context(), c.DB().DB, token, licenseID, appID, expiresAt); err != nil {
		h.log.Printf("download-token: insert: %v", err)
		return apierr.ErrInternal()
	}

	return c.JSON(http.StatusOK, map[string]string{
		"url":        "/api/downloads/file?token=" + token,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}
