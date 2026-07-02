package store

import (
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/i18n"
	"github.com/SteVio89/stevio-home/markdown"
)

const legalPlaceholder = "Please configure your legal pages in Admin > Legal Pages."

func (h *StoreHandler) GetImpressum(c *app.Ctx) error {
	return h.serveLegalPage(c, "impressum")
}

func (h *StoreHandler) GetPrivacyPolicy(c *app.Ctx) error {
	return h.serveLegalPage(c, "privacy_policy")
}

func (h *StoreHandler) GetRefundPolicy(c *app.Ctx) error {
	return h.serveLegalPage(c, "refund_policy")
}

func (h *StoreHandler) serveLegalPage(c *app.Ctx, pageKey string) error {
	ctx := c.R.Context()
	loc := c.Lang()
	defaultLoc := c.Locales().Default(ctx)

	fields, err := i18n.GetPageTranslation(ctx, c.DB().DB, pageKey, loc)
	if err != nil {
		h.log.Printf("legal: get %q (locale %s): %v", pageKey, loc, err)
		return apierr.ErrInternal()
	}
	md := fields["content"]

	if md == "" && loc != defaultLoc {
		defFields, err := i18n.GetPageTranslation(ctx, c.DB().DB, pageKey, defaultLoc)
		if err != nil {
			h.log.Printf("legal: get %q (default locale %s): %v", pageKey, defaultLoc, err)
			return apierr.ErrInternal()
		}
		md = defFields["content"]
	}
	if md == "" {
		md = legalPlaceholder
	}
	return c.JSON(http.StatusOK, map[string]string{"html": markdown.ToHTML(md)})
}
