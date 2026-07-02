package account

import (
	"net/http"

	"github.com/SteVio89/stevio-home/app"
)

func (h *AccountHandler) GetMe(c *app.Ctx) error {
	return c.JSON(http.StatusOK, map[string]any{"is_admin": c.User().Role == "admin"})
}
