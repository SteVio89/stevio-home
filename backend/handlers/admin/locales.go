package admin

import (
	"database/sql"
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/handlers/common"
	"github.com/SteVio89/stevio-home/i18n"
)

func (h *AdminHandler) AdminListLocales(c *app.Ctx) error {
	locales, err := i18n.ListAllLocales(c.R.Context(), c.DB().DB)
	if err != nil {
		h.log.Printf("admin: list locales: %v", err)
		return apierr.ErrInternal()
	}
	if locales == nil {
		locales = []i18n.Locale{}
	}
	return c.JSON(http.StatusOK, locales)
}

func (h *AdminHandler) AdminCreateLocale(c *app.Ctx) error {
	var req struct {
		Code      string `json:"code"`
		Name      string `json:"name"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.Decode(&req); err != nil {
		return apierr.ErrBadRequest()
	}
	if !common.LocaleCodeRe.MatchString(req.Code) {
		return apierr.ErrBadRequest()
	}
	if req.Name == "" || len(req.Name) > 100 {
		return apierr.ErrBadRequest()
	}

	existing, err := i18n.GetLocale(c.R.Context(), c.DB().DB, req.Code)
	if err != nil {
		h.log.Printf("admin: create locale check: %v", err)
		return apierr.ErrInternal()
	}
	if existing != nil {
		return apierr.ErrConflict()
	}

	row := i18n.Locale{
		Code:      req.Code,
		Name:      req.Name,
		IsDefault: false,
		Enabled:   true,
		SortOrder: req.SortOrder,
	}
	if err := i18n.UpsertLocale(c.R.Context(), c.DB().DB, row); err != nil {
		h.log.Printf("admin: create locale: %v", err)
		return apierr.ErrInternal()
	}

	c.Locales().Invalidate()
	return c.JSON(http.StatusCreated, row)
}

func (h *AdminHandler) AdminUpdateLocale(c *app.Ctx) error {
	code := c.Param("code")

	existing, err := i18n.GetLocale(c.R.Context(), c.DB().DB, code)
	if err != nil {
		h.log.Printf("admin: update locale get: %v", err)
		return apierr.ErrInternal()
	}
	if existing == nil {
		return apierr.ErrNotFound()
	}

	var req struct {
		Name      *string `json:"name"`
		Enabled   *bool   `json:"enabled"`
		IsDefault *bool   `json:"is_default"`
		SortOrder *int    `json:"sort_order"`
	}
	if err := c.Decode(&req); err != nil {
		return apierr.ErrBadRequest()
	}

	if req.Name != nil {
		if *req.Name == "" || len(*req.Name) > 100 {
			return apierr.ErrBadRequest()
		}
		existing.Name = *req.Name
	}
	if req.SortOrder != nil {
		existing.SortOrder = *req.SortOrder
	}

	if req.IsDefault != nil && *req.IsDefault && !existing.IsDefault {
		if !existing.Enabled {
			return apierr.ErrBadRequest()
		}
		if err := c.DB().WithTx(c.R.Context(), func(tx *sql.Tx) error {
			return i18n.SetDefaultLocale(c.R.Context(), tx, code)
		}); err != nil {
			h.log.Printf("admin: set default locale: %v", err)
			return apierr.ErrInternal()
		}
		existing.IsDefault = true
	}

	if req.Enabled != nil {
		if !*req.Enabled && existing.IsDefault {
			return apierr.ErrBadRequest()
		}
		existing.Enabled = *req.Enabled
	}

	if err := i18n.UpsertLocale(c.R.Context(), c.DB().DB, *existing); err != nil {
		h.log.Printf("admin: update locale: %v", err)
		return apierr.ErrInternal()
	}

	c.Locales().Invalidate()
	return c.JSON(http.StatusOK, existing)
}
