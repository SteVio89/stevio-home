package sdk

import (
	"log"

	"github.com/SteVio89/stevio-home/config"
	"github.com/SteVio89/stevio-home/crypto"
)

// SDKHandler serves in-app Mac endpoints (license activation, downloads, updates).
type SDKHandler struct {
	log    *log.Logger
	cfg    *config.Config
	signer *crypto.Signer
}

func New(logger *log.Logger, cfg *config.Config, signer *crypto.Signer) *SDKHandler {
	return &SDKHandler{log: logger, cfg: cfg, signer: signer}
}
