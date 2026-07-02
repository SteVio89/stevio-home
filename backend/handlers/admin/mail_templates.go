package admin

import (
	"fmt"
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/handlers/common"
	"github.com/SteVio89/stevio-home/i18n"
)

var defaultMailTemplates = map[string]map[string]i18n.MailTemplate{
	"magic_link": {
		"de": {Locale: "de", Template: "magic_link", Subject: "Dein Login-Link", Body: "Klicke den folgenden Link, um dich einzuloggen.\n\nDieser Link läuft in %d Minuten ab und kann nur einmal verwendet werden.\n\n%s\n\nFalls du dies nicht angefordert hast, ignoriere diese E-Mail."},
		"en": {Locale: "en", Template: "magic_link", Subject: "Your login link", Body: "Click the link below to log in to your account.\n\nThis link expires in %d minutes and can only be used once.\n\n%s\n\nIf you didn't request this, ignore this email."},
	},
}

func (h *AdminHandler) AdminGetMailTemplate(c *app.Ctx) error {
	loc := c.Param("locale")
	if !common.LocaleCodeRe.MatchString(loc) {
		return apierr.ErrBadRequest()
	}

	tmpl, err := i18n.GetMailTemplate(c.R.Context(), c.DB().DB, loc, "magic_link")
	if err != nil {
		h.log.Printf("admin: get mail template %q: %v", loc, err)
		return apierr.ErrInternal()
	}

	if tmpl == nil {
		if defaults, ok := defaultMailTemplates["magic_link"]; ok {
			if def, ok := defaults[loc]; ok {
				tmpl = &def
			} else if def, ok := defaults["de"]; ok {
				tmpl = &def
				tmpl.Locale = loc
			}
		}
		if tmpl == nil {
			tmpl = &i18n.MailTemplate{Locale: loc, Template: "magic_link", Subject: "", Body: ""}
		}
	}

	return c.JSON(http.StatusOK, tmpl)
}

func (h *AdminHandler) AdminUpsertMailTemplate(c *app.Ctx) error {
	loc := c.Param("locale")

	if !c.Locales().IsSupported(c.R.Context(), loc) {
		return apierr.ErrBadRequest()
	}

	var req struct {
		Subject string `json:"subject"`
		Body    string `json:"body"`
	}
	if err := c.Decode(&req); err != nil {
		return apierr.ErrBadRequest()
	}
	if len(req.Subject) > 255 || len(req.Body) > 10000 {
		return apierr.ErrBadRequest()
	}
	if req.Subject == "" || req.Body == "" {
		return apierr.ErrBadRequest()
	}

	testBody := fmt.Sprintf(req.Body, 15, "https://example.com/verify?token=test")
	if testBody == req.Body {
		return apierr.ErrBadRequest()
	}

	if err := i18n.UpsertMailTemplate(c.R.Context(), c.DB().DB, loc, "magic_link", req.Subject, req.Body); err != nil {
		h.log.Printf("admin: upsert mail template %q: %v", loc, err)
		return apierr.ErrInternal()
	}

	if r := h.app.MailTemplates(); r != nil {
		r.SetTemplate(loc, "magic_link", req.Subject, req.Body)
	}
	return c.NoContent()
}
