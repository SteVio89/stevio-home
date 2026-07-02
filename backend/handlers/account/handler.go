package account

import (
	"log"

	"github.com/SteVio89/stevio-home/config"
)

// AccountHandler serves authenticated user endpoints.
type AccountHandler struct {
	log *log.Logger
	cfg *config.Config
}

func New(logger *log.Logger, cfg *config.Config) *AccountHandler {
	return &AccountHandler{log: logger, cfg: cfg}
}
