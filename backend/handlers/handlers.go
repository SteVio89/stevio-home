package handlers

import (
	"log"

	appsvc "github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/config"
	"github.com/SteVio89/stevio-home/crypto"
	"github.com/SteVio89/stevio-home/handlers/account"
	"github.com/SteVio89/stevio-home/handlers/admin"
	"github.com/SteVio89/stevio-home/handlers/chat"
	"github.com/SteVio89/stevio-home/handlers/checkout"
	"github.com/SteVio89/stevio-home/handlers/discounts"
	"github.com/SteVio89/stevio-home/handlers/sdk"
	"github.com/SteVio89/stevio-home/handlers/store"
	"github.com/SteVio89/stevio-home/mailer"
	"github.com/SteVio89/stevio-home/payment"
)

type Handlers struct {
	Store     *store.StoreHandler
	Checkout  *checkout.CheckoutHandler
	SDK       *sdk.SDKHandler
	Account   *account.AccountHandler
	Discounts *discounts.DiscountsHandler
	Admin     *admin.AdminHandler
	Chat      *chat.ChatHandler

	// TrustedProxy mirrors the global limiter's setting: behind the production
	// Caddy→nginx chain the real client IP arrives via X-Real-IP, so per-route
	// rate limiters must trust the proxy or they collapse to a single shared bucket.
	TrustedProxy bool
}

func New(app *appsvc.App, cfg *config.Config, signer *crypto.Signer, logger *log.Logger, payments payment.Registry, mail *mailer.Mailer) *Handlers {
	return &Handlers{
		Store:        store.New(logger, cfg),
		Checkout:     checkout.New(logger, cfg, payments),
		SDK:          sdk.New(logger, cfg, signer),
		Account:      account.New(logger, cfg),
		Discounts:    discounts.New(logger),
		Admin:        admin.New(app, logger, cfg, payments, cfg.SigningKeySecret),
		Chat:         chat.New(app, logger, cfg, mail),
		TrustedProxy: cfg.Env == "production",
	}
}
