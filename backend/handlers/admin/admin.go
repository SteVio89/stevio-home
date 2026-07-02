package admin

import (
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/handlers/common"
)

func (h *AdminHandler) AdminGetStats(c *app.Ctx) error {
	stats, err := queries.GetAdminStats(c.R.Context(), c.DB().DB)
	if err != nil {
		h.log.Printf("admin: stats: %v", err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusOK, stats)
}

func (h *AdminHandler) AdminListOrders(c *app.Ctx) error {
	q := c.R.URL.Query()
	page, perPage, err := common.ParsePaginationParams(q)
	if err != nil {
		return apierr.ErrBadRequest()
	}
	filter := queries.OrderListFilter{
		Page:    page,
		PerPage: perPage,
		AppID:   q.Get("app_id"),
		From:    q.Get("from"),
		To:      q.Get("to"),
	}
	defaultLocale := c.Locales().Default(c.R.Context())
	result, err := queries.ListAllOrders(c.R.Context(), c.DB().DB, defaultLocale, filter)
	if err != nil {
		h.log.Printf("admin: list orders: %v", err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusOK, result)
}
