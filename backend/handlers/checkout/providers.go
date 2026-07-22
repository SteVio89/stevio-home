package checkout

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/crypto"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/payment"
	"github.com/SteVio89/stevio-home/payment/mock"
	"github.com/SteVio89/stevio-home/payment/paddle"
	"github.com/SteVio89/stevio-home/payment/polar"
)

// errPaymentNotConfigured is returned when the active provider is known but
// one or more required credentials/settings are missing. Callers should map
// this to a 503 response.
var errPaymentNotConfigured = &apierr.APIError{
	Code:    "payment_not_configured",
	Status:  503,
	Message: "Payment is not configured",
}

// buildProvider resolves the currently-configured payment provider by reading
// site_settings on every call, decrypting any secret values, and returning a
// freshly constructed payment.Provider. This replaces the startup-time registry
// lookup so admin changes take effect without a redeploy.
//
// Returns payment.ErrProviderNotConfigured / ErrProviderUnknown for the "no
// provider selected" / "unknown name" cases so the caller can distinguish them
// from mid-configuration states (partially-filled credentials).
func (h *CheckoutHandler) buildProvider(ctx context.Context, c *app.Ctx) (payment.Provider, error) {
	name, _ := c.Settings().Get(ctx, "payment_provider")
	if name == "" {
		return nil, payment.ErrProviderNotConfigured
	}
	if _, ok := h.payments[name]; !ok {
		return nil, payment.ErrProviderUnknown
	}

	switch name {
	case "paddle":
		apiKey, err := h.loadSecret(ctx, c, "paddle_api_key")
		if err != nil {
			return nil, fmt.Errorf("decrypt paddle_api_key: %w", err)
		}
		hookSec, err := h.loadSecret(ctx, c, "paddle_webhook_secret")
		if err != nil {
			return nil, fmt.Errorf("decrypt paddle_webhook_secret: %w", err)
		}
		env, _ := c.Settings().Get(ctx, "paddle_environment")
		if apiKey == "" || hookSec == "" {
			return nil, errPaymentNotConfigured
		}
		return paddle.New(apiKey, hookSec, env), nil

	case "polar":
		apiKey, err := h.loadSecret(ctx, c, "polar_api_key")
		if err != nil {
			return nil, fmt.Errorf("decrypt polar_api_key: %w", err)
		}
		hookSec, err := h.loadSecret(ctx, c, "polar_webhook_secret")
		if err != nil {
			return nil, fmt.Errorf("decrypt polar_webhook_secret: %w", err)
		}
		env, _ := c.Settings().Get(ctx, "polar_environment")
		if apiKey == "" || hookSec == "" {
			return nil, errPaymentNotConfigured
		}
		return polar.New(apiKey, hookSec, env, polarProductStore{db: c.DB().DB}), nil

	case "mock":
		return mock.New(h.cfg.BaseURL, h.cfg.SigningKeySecret), nil
	}

	return nil, payment.ErrProviderUnknown
}

// polarProductStore adapts db/queries to polar.ProductStore, backing the lazy
// app → Polar-product-id cache with the provider_products table. Kept here (not
// in the polar package) so the provider stays free of DB-query coupling.
type polarProductStore struct{ db *sql.DB }

func (s polarProductStore) GetProductID(ctx context.Context, appID, environment string) (string, error) {
	return queries.GetProviderProductID(ctx, s.db, "polar", appID, environment)
}

func (s polarProductStore) SaveProductID(ctx context.Context, appID, environment, productID string) error {
	return queries.UpsertProviderProduct(ctx, s.db, "polar", appID, environment, productID)
}

// loadSecret reads an encrypted setting and decrypts it. An unset (empty)
// value is returned as ("", nil) so callers can distinguish "not configured"
// from "decryption failure".
func (h *CheckoutHandler) loadSecret(ctx context.Context, c *app.Ctx, key string) (string, error) {
	ct, _ := c.Settings().Get(ctx, key)
	if ct == "" {
		return "", nil
	}
	return crypto.DecryptSetting(h.signingKeySecret, ct)
}

// isProviderConfigError returns true for the "no provider"/"unknown provider"
// sentinels so CreateCheckout can produce a 503 without leaking internals.
func isProviderConfigError(err error) bool {
	return errors.Is(err, payment.ErrProviderNotConfigured) ||
		errors.Is(err, payment.ErrProviderUnknown)
}
