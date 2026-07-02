package admin

import (
	"database/sql"
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/validate"
)

var validPlatforms = map[string]bool{
	"github":      true,
	"linkedin":    true,
	"steam":       true,
	"twitch":      true,
	"xing":        true,
	"playstation": true,
	"youtube":     true,
	"gitlab":      true,
	"codeberg":    true,
	"website":     true,
	"email":       true,
}

func (h *AdminHandler) AdminListSocialLinks(c *app.Ctx) error {
	links, err := queries.ListSocialLinksAdmin(c.R.Context(), c.DB().DB)
	if err != nil {
		h.log.Printf("admin: list social links: %v", err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusOK, links)
}

func (h *AdminHandler) AdminCreateSocialLink(c *app.Ctx) error {
	var req struct {
		Platform string `json:"platform"`
		URL      string `json:"url"`
	}
	if err := c.Decode(&req); err != nil || req.Platform == "" || req.URL == "" {
		return apierr.ErrBadRequest()
	}
	if !validPlatforms[req.Platform] {
		return apierr.ErrBadRequest()
	}
	if err := validate.LinkURL(req.URL); err != nil {
		return apierr.ErrBadRequest()
	}

	ctx := c.R.Context()
	maxPos, err := queries.MaxSocialLinkPosition(ctx, c.DB().DB)
	if err != nil {
		h.log.Printf("admin: max social link position: %v", err)
		return apierr.ErrInternal()
	}

	link, err := queries.InsertSocialLink(ctx, c.DB().DB, req.Platform, req.URL, maxPos+1)
	if err != nil {
		h.log.Printf("admin: create social link: %v", err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusCreated, link)
}

func (h *AdminHandler) AdminUpdateSocialLink(c *app.Ctx) error {
	id := c.Param("id")
	var req struct {
		Platform string `json:"platform"`
		URL      string `json:"url"`
	}
	if err := c.Decode(&req); err != nil || req.Platform == "" || req.URL == "" {
		return apierr.ErrBadRequest()
	}
	if !validPlatforms[req.Platform] {
		return apierr.ErrBadRequest()
	}
	if err := validate.LinkURL(req.URL); err != nil {
		return apierr.ErrBadRequest()
	}

	if err := queries.UpdateSocialLink(c.R.Context(), c.DB().DB, id, req.Platform, req.URL); err != nil {
		h.log.Printf("admin: update social link %q: %v", id, err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}

func (h *AdminHandler) AdminDeleteSocialLink(c *app.Ctx) error {
	if err := queries.DeleteSocialLink(c.R.Context(), c.DB().DB, c.Param("id")); err != nil {
		h.log.Printf("admin: delete social link %q: %v", c.Param("id"), err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}

func (h *AdminHandler) AdminReorderSocialLinks(c *app.Ctx) error {
	var req struct {
		Positions map[string]int `json:"positions"`
	}
	if err := c.Decode(&req); err != nil || len(req.Positions) == 0 {
		return apierr.ErrBadRequest()
	}

	if err := queries.WithTx(c.R.Context(), c.DB().DB, func(tx *sql.Tx) error {
		for id, pos := range req.Positions {
			if err := queries.UpdateSocialLinkPosition(c.R.Context(), tx, id, pos); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		h.log.Printf("admin: reorder social links: %v", err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}
