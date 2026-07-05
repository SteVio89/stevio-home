// Package payment defines the interface that any payment provider must implement.
// To add a provider, implement Provider and wire it into the Registry in main.go.
package payment

import (
	"context"
	"errors"
	"net/http"
)

var (
	ErrProviderNotConfigured = errors.New("no payment provider configured")
	ErrProviderUnknown       = errors.New("unknown payment provider")
)

// CheckoutParams carries everything a provider needs to create a checkout session.
type CheckoutParams struct {
	AppID          string // internal app ID — stored in custom data, returned in webhook
	AppName        string // human-readable name — shown as line item label
	PriceCents     int    // final price after discounts, in cents
	CurrencyCode   string // ISO 4217 code (e.g. "EUR", "USD")
	TaxCategory    string // Paddle tax category (e.g. "digital-goods"); providers that don't care may ignore
	SuccessURL     string // redirect target after successful payment
	CancelURL      string // redirect target on cancellation
	DiscountCode   string // may be empty; carried through for webhook consumption
	CustomerEmail  string // optional pre-fill; empty string means omit
	ConsentGivenAt string // ISO-8601 timestamp of withdrawal waiver consent; may be empty
}

// CheckoutSession is returned to the frontend to start a checkout. Providers
// populate the fields their client-side flow needs:
//   - Paddle Billing opens an in-page Paddle.js overlay, so it sets
//     TransactionID (the txn_… id the browser passes to Paddle.Checkout.open).
//   - The mock provider redirects the browser, so it sets URL.
type CheckoutSession struct {
	URL           string // redirect URL for the customer (mock provider)
	TransactionID string // Paddle transaction id (txn_…) for the client-side overlay
	SessionID     string // provider-specific idempotency key stored in orders.payment_session
}

// Event represents a confirmed purchase or refund from any payment provider.
type Event struct {
	Type           string // "order" or "refund"
	SessionID      string // idempotency key — must match CheckoutSession.SessionID
	AppID          string // from custom data
	Email          string // buyer raw email — MUST be hashed via crypto.HashEmail before storage
	PriceCents     int    // actual amount charged, in cents
	DiscountCode   string // from custom data; may be empty
	ConsentGivenAt string // ISO-8601 timestamp of withdrawal waiver consent; may be empty
}

// Provider is the interface any payment backend must implement.
type Provider interface {
	// CreateCheckout creates a provider checkout and returns a CheckoutSession
	// carrying the session ID plus whatever the provider's client-side flow
	// needs (Paddle: TransactionID for the overlay; mock: URL for a redirect).
	CreateCheckout(ctx context.Context, p CheckoutParams) (CheckoutSession, error)

	// ParseWebhook validates the raw webhook request body and headers,
	// and returns a confirmed purchase Event or nil (valid but ignorable event type).
	// Returns error if the request is invalid or signature verification fails.
	ParseWebhook(body []byte, headers http.Header) (*Event, error)

	// Name returns the provider identifier stored in site_settings (e.g. "paddle").
	Name() string
}

// Registry maps provider names to initialized provider instances.
type Registry map[string]Provider

// Get returns the provider for the given name, or ErrProviderUnknown.
func (r Registry) Get(name string) (Provider, error) {
	if name == "" {
		return nil, ErrProviderNotConfigured
	}
	p, ok := r[name]
	if !ok {
		return nil, ErrProviderUnknown
	}
	return p, nil
}
