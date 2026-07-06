package admin

import (
	"errors"
	"net/http"
	"time"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/dbutil"
	"github.com/SteVio89/stevio-home/handlers/common"
)

func (h *AdminHandler) AdminListDiscountCodes(c *app.Ctx) error {
	codes, err := queries.ListDiscountCodes(c.R.Context(), c.DB().DB)
	if err != nil {
		h.log.Printf("admin: list discount codes: %v", err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusOK, codes)
}

func (h *AdminHandler) AdminCreateDiscountCode(c *app.Ctx) error {
	var req struct {
		Code          string  `json:"code"`
		Label         string  `json:"label"`
		DiscountType  string  `json:"discount_type"`
		DiscountValue int     `json:"discount_value"`
		AppID         *string `json:"app_id"`
		MaxUses       *int    `json:"max_uses"`
		ExpiresAt     *string `json:"expires_at"`
		Stackable     bool    `json:"stackable"`
	}
	if err := c.Decode(&req); err != nil {
		return apierr.ErrBadRequest()
	}
	if req.Code == "" || len(req.Code) > 64 || len(req.Label) > 255 {
		return apierr.ErrBadRequest()
	}
	if err := common.ValidateDiscountParams(req.DiscountType, req.DiscountValue); err != nil {
		return apierr.ErrBadRequest()
	}

	expiresAt, err := common.ParseOptionalTime(req.ExpiresAt, time.RFC3339Nano)
	if err != nil {
		return apierr.ErrBadRequest()
	}

	params := queries.InsertDiscountCodeParams{
		Code:          req.Code,
		Label:         req.Label,
		DiscountType:  req.DiscountType,
		DiscountValue: req.DiscountValue,
		AppID:         req.AppID,
		MaxUses:       req.MaxUses,
		ExpiresAt:     expiresAt,
		Stackable:     req.Stackable,
	}

	code, err := queries.InsertDiscountCode(c.R.Context(), c.DB().DB, params)
	if err != nil {
		if dbutil.IsUniqueViolation(err) {
			return apierr.ErrConflict()
		}
		h.log.Printf("admin: create discount code: %v", err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusCreated, code)
}

func (h *AdminHandler) AdminUpdateDiscountCode(c *app.Ctx) error {
	id := c.Param("id")
	var req struct {
		Label         string  `json:"label"`
		DiscountType  string  `json:"discount_type"`
		DiscountValue int     `json:"discount_value"`
		AppID         *string `json:"app_id"`
		MaxUses       *int    `json:"max_uses"`
		ExpiresAt     *string `json:"expires_at"`
		Active        bool    `json:"active"`
		Stackable     bool    `json:"stackable"`
	}
	if err := c.Decode(&req); err != nil {
		return apierr.ErrBadRequest()
	}
	if len(req.Label) > 255 {
		return apierr.ErrBadRequest()
	}
	if err := common.ValidateDiscountParams(req.DiscountType, req.DiscountValue); err != nil {
		return apierr.ErrBadRequest()
	}

	expiresAt, err := common.ParseOptionalTime(req.ExpiresAt, time.RFC3339Nano)
	if err != nil {
		return apierr.ErrBadRequest()
	}

	params := queries.UpdateDiscountCodeParams{
		Label:         req.Label,
		DiscountType:  req.DiscountType,
		DiscountValue: req.DiscountValue,
		AppID:         req.AppID,
		MaxUses:       req.MaxUses,
		ExpiresAt:     expiresAt,
		Active:        req.Active,
		Stackable:     req.Stackable,
	}

	code, err := queries.UpdateDiscountCode(c.R.Context(), c.DB().DB, id, params)
	if err != nil {
		if errors.Is(err, queries.ErrDiscountNotFound) {
			return apierr.ErrNotFound()
		}
		h.log.Printf("admin: update discount code %q: %v", id, err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusOK, code)
}

func (h *AdminHandler) AdminDeleteDiscountCode(c *app.Ctx) error {
	id := c.Param("id")
	if err := queries.SoftDeleteDiscountCode(c.R.Context(), c.DB().DB, id); err != nil {
		if errors.Is(err, queries.ErrDiscountNotFound) {
			return apierr.ErrNotFound()
		}
		h.log.Printf("admin: archive discount code %q: %v", id, err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}

// AdminHardDeleteDiscountCode permanently deletes an already-archived code.
func (h *AdminHandler) AdminHardDeleteDiscountCode(c *app.Ctx) error {
	id := c.Param("id")
	if err := queries.HardDeleteDiscountCode(c.R.Context(), c.DB().DB, id); err != nil {
		if errors.Is(err, queries.ErrDiscountNotFound) {
			return apierr.ErrNotFound()
		}
		h.log.Printf("admin: permanently delete discount code %q: %v", id, err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}

func (h *AdminHandler) AdminRestoreDiscountCode(c *app.Ctx) error {
	id := c.Param("id")
	if err := queries.RestoreDiscountCode(c.R.Context(), c.DB().DB, id); err != nil {
		if errors.Is(err, queries.ErrDiscountNotFound) {
			return apierr.ErrNotFound()
		}
		h.log.Printf("admin: restore discount code %q: %v", id, err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}
