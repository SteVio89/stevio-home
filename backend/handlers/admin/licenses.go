package admin

import (
	"database/sql"
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/crypto"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/dbutil"
	"github.com/SteVio89/stevio-home/handlers/common"
	"github.com/SteVio89/stevio-home/pathutil"
)

func (h *AdminHandler) AdminUnrevokeLicense(c *app.Ctx) error {
	id, err := pathutil.SanitizePathSegment(c.Param("id"))
	if err != nil {
		return apierr.ErrBadRequest()
	}
	if err := queries.UnrevokeLicense(c.R.Context(), c.DB().DB, id); err != nil {
		h.log.Printf("admin: unrevoke license %s: %v", id, err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}

func (h *AdminHandler) AdminListLicenses(c *app.Ctx) error {
	q := c.R.URL.Query()
	page, perPage, err := common.ParsePaginationParams(q)
	if err != nil {
		return apierr.ErrBadRequest()
	}
	defaultLocale := c.Locales().Default(c.R.Context())
	result, err := queries.ListAllLicenses(c.R.Context(), c.DB().DB, defaultLocale, queries.LicenseListFilter{
		Page:      page,
		PerPage:   perPage,
		AppID:     q.Get("app_id"),
		KeyPrefix: q.Get("key_prefix"),
	})
	if err != nil {
		h.log.Printf("admin: list licenses: %v", err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusOK, result)
}

func (h *AdminHandler) AdminIssueLicense(c *app.Ctx) error {
	var req struct {
		Email      string `json:"email"`
		AppID      string `json:"app_id"`
		PriceCents int    `json:"price_cents"`
	}
	if err := c.Decode(&req); err != nil || req.Email == "" || req.AppID == "" {
		return apierr.ErrBadRequest()
	}
	if len(req.Email) > 255 || len(req.AppID) > 128 || req.PriceCents < 0 {
		return apierr.ErrBadRequest()
	}

	ctx := c.R.Context()
	commerceApp, err := queries.GetAppByID(ctx, c.DB().DB, req.AppID)
	if err != nil {
		h.log.Printf("admin: issue license: get app: %v", err)
		return apierr.ErrInternal()
	}
	if commerceApp == nil || commerceApp.DeletedAt != nil {
		return apierr.ErrNotFound()
	}

	emailHash := crypto.HashEmail(req.Email, h.cfg.EmailHashSalt)
	sessionID := "manual-" + dbutil.NewID()

	var licenseKey string
	err = queries.WithTx(ctx, c.DB().DB, func(tx *sql.Tx) error {
		order, err := queries.InsertOrder(ctx, tx, sessionID, emailHash, commerceApp.ID,
			req.PriceCents, nil, nil, queries.OrderDiscountSnapshot{}, "")
		if err != nil {
			return err
		}
		key := queries.NewLicenseKey()
		_, err = queries.InsertLicense(ctx, tx, key, order.ID, commerceApp.ID, nil)
		if err != nil {
			return err
		}
		licenseKey = key
		return nil
	})
	if err != nil {
		h.log.Printf("admin: issue license: %v", err)
		return apierr.ErrInternal()
	}

	return c.JSON(http.StatusCreated, map[string]string{
		"license_key": licenseKey,
	})
}
