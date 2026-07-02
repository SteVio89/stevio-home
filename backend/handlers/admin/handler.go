package admin

import (
	"log"

	appsvc "github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/config"
	"github.com/SteVio89/stevio-home/payment"
)

// AdminHandler serves admin-only management endpoints.
type AdminHandler struct {
	app              *appsvc.App
	log              *log.Logger
	cfg              *config.Config
	payments         payment.Registry
	signingKeySecret [32]byte
}

func New(app *appsvc.App, logger *log.Logger, cfg *config.Config, payments payment.Registry, signingKeySecret [32]byte) *AdminHandler {
	return &AdminHandler{app: app, log: logger, cfg: cfg, payments: payments, signingKeySecret: signingKeySecret}
}
