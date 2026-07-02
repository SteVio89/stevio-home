package admin

import (
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/crypto"
	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/db/queries"
)

type adminLookupRequest struct {
	Email string `json:"email"`
}

type adminUserData struct {
	Hash           string                      `json:"hash"`
	Sessions       []queries.UserSession       `json:"sessions"`
	Orders         []queries.UserOrder         `json:"orders"`
	Licenses       []models.License            `json:"licenses"`
	Activations    []models.Activation         `json:"activations"`
	DownloadTokens []queries.UserDownloadToken `json:"download_tokens"`
}

func (h *AdminHandler) AdminLookupUser(c *app.Ctx) error {
	var req adminLookupRequest
	if err := c.Decode(&req); err != nil || req.Email == "" {
		return apierr.ErrBadRequest()
	}

	emailHash := crypto.HashEmail(req.Email, h.cfg.EmailHashSalt)

	sessions, err := queries.GetUserSessions(c.R.Context(), c.DB().DB, emailHash)
	if err != nil {
		h.log.Printf("admin lookup: sessions: %v", err)
		return apierr.ErrInternal()
	}

	defaultLocale := c.Locales().Default(c.R.Context())

	orders, err := queries.GetUserOrders(c.R.Context(), c.DB().DB, emailHash, defaultLocale)
	if err != nil {
		h.log.Printf("admin lookup: orders: %v", err)
		return apierr.ErrInternal()
	}

	licenses, err := queries.GetLicensesByEmail(c.R.Context(), c.DB().DB, emailHash, defaultLocale)
	if err != nil {
		h.log.Printf("admin lookup: licenses: %v", err)
		return apierr.ErrInternal()
	}
	if licenses == nil {
		licenses = []models.License{}
	}

	activations, err := queries.GetUserActivationsByEmail(c.R.Context(), c.DB().DB, emailHash)
	if err != nil {
		h.log.Printf("admin lookup: activations: %v", err)
		return apierr.ErrInternal()
	}

	tokens, err := queries.GetUserDownloadTokensByEmail(c.R.Context(), c.DB().DB, emailHash)
	if err != nil {
		h.log.Printf("admin lookup: download tokens: %v", err)
		return apierr.ErrInternal()
	}

	return c.JSON(http.StatusOK, adminUserData{
		Hash:           emailHash,
		Sessions:       sessions,
		Orders:         orders,
		Licenses:       licenses,
		Activations:    activations,
		DownloadTokens: tokens,
	})
}

type adminRenameActivationRequest struct {
	DeviceLabel string `json:"device_label"`
}

func (h *AdminHandler) AdminRenameActivation(c *app.Ctx) error {
	id := c.Param("id")

	var req adminRenameActivationRequest
	if err := c.Decode(&req); err != nil || req.DeviceLabel == "" {
		return apierr.ErrBadRequest()
	}
	if len(req.DeviceLabel) > 255 {
		return apierr.ErrBadRequest()
	}

	if err := queries.UpdateDeviceLabel(c.R.Context(), c.DB().DB, id, req.DeviceLabel); err != nil {
		h.log.Printf("admin rename activation: %v", err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "updated"})
}

func (h *AdminHandler) AdminRevokeActivation(c *app.Ctx) error {
	id := c.Param("id")

	if err := queries.DeleteActivation(c.R.Context(), c.DB().DB, id); err != nil {
		h.log.Printf("admin revoke activation: %v", err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}

func (h *AdminHandler) AdminDeleteUserSessions(c *app.Ctx) error {
	hash := c.Param("hash")
	if hash == "" {
		return apierr.ErrBadRequest()
	}

	n, err := queries.DeleteSessionsByEmail(c.R.Context(), c.DB().DB, hash)
	if err != nil {
		h.log.Printf("admin delete sessions: %v", err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusOK, map[string]any{"deleted": n})
}

func (h *AdminHandler) AdminVoidOrder(c *app.Ctx) error {
	id := c.Param("id")

	if err := queries.VoidOrder(c.R.Context(), c.DB().DB, id); err != nil {
		h.log.Printf("admin void order: %v", err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}
