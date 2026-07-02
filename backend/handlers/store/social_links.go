package store

import (
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/queries"
)

func (h *StoreHandler) ListSocialLinks(c *app.Ctx) error {
	links, err := queries.ListSocialLinksPublic(c.R.Context(), c.DB().DB)
	if err != nil {
		h.log.Printf("store: list social links: %v", err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusOK, links)
}
