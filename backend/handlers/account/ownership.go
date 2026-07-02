package account

import (
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/queries"
)

func (h *AccountHandler) GetOwnership(c *app.Ctx) error {
	appID := c.Param("id")
	emailHash := c.User().EmailHash

	commerceApp, err := queries.GetAppByID(c.R.Context(), c.DB().DB, appID)
	if err != nil {
		h.log.Printf("ownership: get app %q: %v", appID, err)
		return apierr.ErrInternal()
	}
	if commerceApp == nil || commerceApp.DeletedAt != nil {
		return apierr.ErrNotFound()
	}

	status, err := queries.GetOwnershipStatus(c.R.Context(), c.DB().DB, commerceApp.ID, emailHash)
	if err != nil {
		h.log.Printf("ownership: check %q: %v", commerceApp.ID, err)
		return apierr.ErrInternal()
	}

	return c.JSON(http.StatusOK, map[string]any{
		"has_license":   status.HasLicense,
		"purchase_mode": commerceApp.PurchaseMode,
	})
}
