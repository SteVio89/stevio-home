package admin

import (
	"net/http"
	"unicode"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/queries"
)

func (h *AdminHandler) AdminGetSales(c *app.Ctx) error {
	start := c.R.URL.Query().Get("start")
	end := c.R.URL.Query().Get("end")

	if !isValidDateParam(start) || !isValidDateParam(end) {
		return apierr.ErrBadRequest()
	}

	defaultLocale := c.Locales().Default(c.R.Context())
	report, err := queries.GetSalesReport(c.R.Context(), c.DB().DB, start, end, defaultLocale)
	if err != nil {
		h.log.Printf("admin sales: %v", err)
		return apierr.ErrInternal()
	}

	return c.JSON(http.StatusOK, report)
}

func isValidDateParam(s string) bool {
	if s == "" {
		return true
	}
	if len(s) != 10 {
		return false
	}
	for _, c := range s {
		if !unicode.IsDigit(c) && c != '-' {
			return false
		}
	}
	return true
}
