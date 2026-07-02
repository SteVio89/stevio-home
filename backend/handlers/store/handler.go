package store

import (
	"log"

	"github.com/SteVio89/stevio-home/config"
)

// StoreHandler serves public catalog endpoints (apps, legal, config, i18n).
type StoreHandler struct {
	log *log.Logger
	cfg *config.Config
}

func New(logger *log.Logger, cfg *config.Config) *StoreHandler {
	return &StoreHandler{log: logger, cfg: cfg}
}
