package admin

import (
	"errors"
	"net/http"
	"time"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/handlers/common"
)

func (h *AdminHandler) AdminListAutoDiscounts(c *app.Ctx) error {
	discounts, err := queries.ListAutoDiscounts(c.R.Context(), c.DB().DB)
	if err != nil {
		h.log.Printf("admin: list auto discounts: %v", err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusOK, discounts)
}

func (h *AdminHandler) AdminCreateAutoDiscount(c *app.Ctx) error {
	var req struct {
		Label         string  `json:"label"`
		DiscountType  string  `json:"discount_type"`
		DiscountValue int     `json:"discount_value"`
		AppID         *string `json:"app_id"`
		ValidFrom     *string `json:"valid_from"`
		ExpiresAt     *string `json:"expires_at"`
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

	validFrom, err := common.ParseOptionalTime(req.ValidFrom, time.RFC3339Nano)
	if err != nil {
		return apierr.ErrBadRequest()
	}
	expiresAt, err := common.ParseOptionalTime(req.ExpiresAt, time.RFC3339Nano)
	if err != nil {
		return apierr.ErrBadRequest()
	}

	params := queries.InsertAutoDiscountParams{
		Label:         req.Label,
		DiscountType:  req.DiscountType,
		DiscountValue: req.DiscountValue,
		AppID:         req.AppID,
		ValidFrom:     validFrom,
		ExpiresAt:     expiresAt,
	}

	d, err := queries.InsertAutoDiscount(c.R.Context(), c.DB().DB, params)
	if err != nil {
		h.log.Printf("admin: create auto discount: %v", err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusCreated, d)
}

func (h *AdminHandler) AdminUpdateAutoDiscount(c *app.Ctx) error {
	id := c.Param("id")
	var req struct {
		Label         string  `json:"label"`
		DiscountType  string  `json:"discount_type"`
		DiscountValue int     `json:"discount_value"`
		AppID         *string `json:"app_id"`
		ValidFrom     *string `json:"valid_from"`
		ExpiresAt     *string `json:"expires_at"`
		Active        bool    `json:"active"`
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

	validFrom, err := common.ParseOptionalTime(req.ValidFrom, time.RFC3339Nano)
	if err != nil {
		return apierr.ErrBadRequest()
	}
	expiresAt, err := common.ParseOptionalTime(req.ExpiresAt, time.RFC3339Nano)
	if err != nil {
		return apierr.ErrBadRequest()
	}

	params := queries.UpdateAutoDiscountParams{
		Label:         req.Label,
		DiscountType:  req.DiscountType,
		DiscountValue: req.DiscountValue,
		AppID:         req.AppID,
		ValidFrom:     validFrom,
		ExpiresAt:     expiresAt,
		Active:        req.Active,
	}

	d, err := queries.UpdateAutoDiscount(c.R.Context(), c.DB().DB, id, params)
	if err != nil {
		if errors.Is(err, queries.ErrAutoDiscountNotFound) {
			return apierr.ErrNotFound()
		}
		h.log.Printf("admin: update auto discount %q: %v", id, err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusOK, d)
}

func (h *AdminHandler) AdminDeleteAutoDiscount(c *app.Ctx) error {
	id := c.Param("id")
	if err := queries.SoftDeleteAutoDiscount(c.R.Context(), c.DB().DB, id); err != nil {
		if errors.Is(err, queries.ErrAutoDiscountNotFound) {
			return apierr.ErrNotFound()
		}
		h.log.Printf("admin: archive auto discount %q: %v", id, err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}

func (h *AdminHandler) AdminRestoreAutoDiscount(c *app.Ctx) error {
	id := c.Param("id")
	if err := queries.RestoreAutoDiscount(c.R.Context(), c.DB().DB, id); err != nil {
		if errors.Is(err, queries.ErrAutoDiscountNotFound) {
			return apierr.ErrNotFound()
		}
		h.log.Printf("admin: restore auto discount %q: %v", id, err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}
