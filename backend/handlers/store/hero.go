package store

import (
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/i18n"
)

// GetHero returns locale-specific hero content from page_translations.
// Uses default-locale-first overlay pattern: load default locale as base,
// then overlay the requested locale on top if different.
func (h *StoreHandler) GetHero(c *app.Ctx) error {
	ctx := c.R.Context()
	defaultLoc := c.Locales().Default(ctx)
	loc := c.Lang()

	fields, err := i18n.GetPageTranslation(ctx, c.DB().DB, "hero", defaultLoc)
	if err != nil {
		h.log.Printf("hero: get default locale %q: %v", defaultLoc, err)
		return apierr.ErrInternal()
	}

	if loc != defaultLoc {
		overlay, err := i18n.GetPageTranslation(ctx, c.DB().DB, "hero", loc)
		if err != nil {
			h.log.Printf("hero: get locale %q: %v", loc, err)
			return apierr.ErrInternal()
		}
		for k, v := range overlay {
			if v != "" {
				fields[k] = v
			}
		}
	}

	return c.JSON(http.StatusOK, map[string]string{
		"headline": fields["headline"],
		"tagline":  fields["tagline"],
		"bio":      fields["bio"],
	})
}
