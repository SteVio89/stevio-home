package checkout

import (
	"log"

	"github.com/SteVio89/stevio-home/config"
	"github.com/SteVio89/stevio-home/payment"
)

// CheckoutHandler serves the payment/checkout flow.
type CheckoutHandler struct {
	log              *log.Logger
	cfg              *config.Config
	payments         payment.Registry
	signingKeySecret [32]byte
}

func New(logger *log.Logger, cfg *config.Config, payments payment.Registry) *CheckoutHandler {
	return &CheckoutHandler{
		log:              logger,
		cfg:              cfg,
		payments:         payments,
		signingKeySecret: cfg.SigningKeySecret,
	}
}
