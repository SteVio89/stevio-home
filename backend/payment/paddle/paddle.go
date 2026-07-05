// Package paddle implements payment.Provider for Paddle Billing using
// non-catalog items: every transaction carries the product name, description,
// unit price, currency, and tax category inline. This matches our store's
// model (products and prices live in Postgres on the apps table) and means
// the admin does not need to mirror them into a Paddle catalog.
//
// We wrap the official github.com/PaddleHQ/paddle-go-sdk/v5 so we inherit
// signature verification (with replay-protection) and HTTP transport for
// free. Credentials are passed in at construction time by the checkout
// handler on each request — an empty-credentials placeholder instance is
// registered at startup just so the provider name is known.
package paddle

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	sdk "github.com/PaddleHQ/paddle-go-sdk/v5"

	"github.com/SteVio89/stevio-home/payment"
)

const (
	envSandbox    = "sandbox"
	envProduction = "production"

	verifyTolerance = 5 * time.Minute

	// defaultTaxCategory is the fallback used when a checkout arrives without
	// an app-level tax_category set. "standard" is Paddle's "Standard digital
	// goods" — the pre-approved Default category for downloadable software, and
	// what stevio sells. (The "digital-goods" slug is a different, narrower
	// category for non-software media files and is not activated by default.)
	defaultTaxCategory = "standard"
)

// ErrSignatureInvalid is returned by ParseWebhook when signature verification
// fails for any SDK reason (missing header, bad HMAC, replay window exceeded).
var ErrSignatureInvalid = errors.New("webhook signature invalid")

// Provider implements payment.Provider for Paddle Billing.
type Provider struct {
	apiKey        string
	webhookSecret string
	environment   string
}

// New returns a Paddle provider. Passing empty strings is valid — the
// placeholder instance registered in main.go exists only so the registry
// knows "paddle" is a supported provider name. Any real CreateCheckout /
// ParseWebhook call must receive a freshly-constructed provider with the
// admin-configured credentials.
func New(apiKey, webhookSecret, environment string) *Provider {
	if environment == "" {
		environment = envProduction
	}
	return &Provider{
		apiKey:        apiKey,
		webhookSecret: webhookSecret,
		environment:   environment,
	}
}

func (p *Provider) Name() string { return "paddle" }

func (p *Provider) baseURL() string {
	if p.environment == envSandbox {
		return sdk.SandboxBaseURL
	}
	return sdk.ProductionBaseURL
}

// CreateCheckout creates a Paddle transaction with a non-catalog item built
// from params (product name, tax category, price, currency) and returns its
// transaction id, which the frontend hands to Paddle.Checkout.open to render
// the in-page overlay. No Paddle-side catalog is involved — every field lives
// in our DB and is passed inline per checkout.
func (p *Provider) CreateCheckout(ctx context.Context, params payment.CheckoutParams) (payment.CheckoutSession, error) {
	if p.apiKey == "" {
		return payment.CheckoutSession{}, errors.New("paddle: api_key is required")
	}
	if params.AppName == "" || params.PriceCents <= 0 || params.CurrencyCode == "" {
		return payment.CheckoutSession{}, errors.New("paddle: app_name, price_cents, and currency_code are required")
	}

	client, err := sdk.New(p.apiKey, sdk.WithBaseURL(p.baseURL()))
	if err != nil {
		return payment.CheckoutSession{}, fmt.Errorf("paddle: client: %w", err)
	}

	sessionID := newSessionID()

	custom := sdk.CustomData{
		"app_id":     params.AppID,
		"session_id": sessionID,
	}
	if params.DiscountCode != "" {
		custom["discount_code"] = params.DiscountCode
	}
	if params.ConsentGivenAt != "" {
		custom["consent_given_at"] = params.ConsentGivenAt
	}

	taxCategory := params.TaxCategory
	if taxCategory == "" {
		taxCategory = defaultTaxCategory
	}

	cc := sdk.CurrencyCode(params.CurrencyCode)
	req := &sdk.CreateTransactionRequest{
		Items:        buildTransactionItems(params, taxCategory),
		CustomData:   custom,
		CurrencyCode: &cc,
	}

	tx, err := client.CreateTransaction(ctx, req)
	if err != nil {
		return payment.CheckoutSession{}, fmt.Errorf("paddle: create transaction: %w", err)
	}

	// Billing checkout runs as an in-page Paddle.js overlay: the browser calls
	// Paddle.Checkout.open({ transactionId }). The transaction id is what the
	// frontend needs; the hosted checkout.url (default payment link + _ptxn) is
	// carried along only as a fallback for non-overlay clients.
	if tx.ID == "" {
		return payment.CheckoutSession{}, errors.New("paddle: response missing transaction id")
	}

	return payment.CheckoutSession{
		URL:           extractCheckoutURL(tx),
		TransactionID: tx.ID,
		SessionID:     sessionID,
	}, nil
}

// buildTransactionItems assembles the single non-catalog line item Paddle
// charges for. Extracted from CreateCheckout so the exact JSON shape can be
// asserted in a unit test without a network client.
func buildTransactionItems(params payment.CheckoutParams, taxCategory string) []sdk.CreateTransactionItems {
	return []sdk.CreateTransactionItems{
		*sdk.NewCreateTransactionItemsTransactionItemCreateWithProduct(
			&sdk.TransactionItemCreateWithProduct{
				Quantity: 1,
				Price: sdk.TransactionPriceCreateWithProduct{
					Description: params.AppName + " license",
					TaxMode:     sdk.TaxModeAccountSetting,
					UnitPrice: sdk.Money{
						Amount:       strconv.Itoa(params.PriceCents),
						CurrencyCode: sdk.CurrencyCode(params.CurrencyCode),
					},
					// Lock the purchasable quantity to exactly 1. The SDK's
					// `json:"quantity,omitempty"` is a no-op on this struct value,
					// so a zero PriceQuantity serializes as `"quantity":{}`, which
					// Paddle rejects (minimum/maximum required). Setting 1..1 also
					// hides the quantity stepper on hosted checkout, keeping one
					// license per order in line with fulfillOrder.
					Quantity: sdk.PriceQuantity{Minimum: 1, Maximum: 1},
					Product: sdk.TransactionSubscriptionProductCreate{
						Name:        params.AppName,
						TaxCategory: sdk.TaxCategory(taxCategory),
					},
				},
			},
		),
	}
}

// ParseWebhook verifies the Paddle-Signature via the SDK and returns a typed
// Event for transaction.completed / refund adjustments; nil for other events.
func (p *Provider) ParseWebhook(body []byte, headers http.Header) (*payment.Event, error) {
	if p.webhookSecret == "" {
		return nil, errors.New("paddle: webhook_secret not configured")
	}

	req, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("paddle: synth request: %w", err)
	}
	req.Header = headers.Clone()

	verifier := sdk.NewWebhookVerifier(
		p.webhookSecret,
		sdk.VerifierWithTimestampTolerance(verifyTolerance),
	)
	ok, err := verifier.Verify(req)
	if err != nil {
		switch {
		case errors.Is(err, sdk.ErrMissingSignature),
			errors.Is(err, sdk.ErrInvalidSignatureFormat),
			errors.Is(err, sdk.ErrReplayAttack):
			return nil, ErrSignatureInvalid
		}
		return nil, fmt.Errorf("paddle: verify: %w", err)
	}
	if !ok {
		return nil, ErrSignatureInvalid
	}

	// Route on event_type. We unmarshal into a tiny local envelope rather than
	// the SDK's notification types so we're not coupled to SDK schema churn
	// — v4's GenericNotificationEvent was renamed in v5, and our typed views
	// below cover exactly the fields we need.
	var envelope struct {
		EventType string `json:"event_type"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("paddle: decode notification envelope: %w", err)
	}

	switch envelope.EventType {
	case "transaction.completed":
		return parseTransactionCompleted(body)
	case "adjustment.created":
		return p.parseAdjustment(req.Context(), body)
	}
	return nil, nil
}

// --- event parsing ---

// minimal typed view over the relevant webhook fields. Using a local shape
// rather than the full SDK notification type keeps this adapter resilient to
// SDK schema additions and easy to test with fixtures.
type txCompleted struct {
	Data struct {
		ID         string         `json:"id"`
		CustomData map[string]any `json:"custom_data"`
		Status     string         `json:"status"`
		Customer   struct {
			Email string `json:"email"`
		} `json:"customer"`
		Details struct {
			Totals struct {
				// Paddle amounts are minor-unit strings (e.g. "1999" for $19.99).
				Total string `json:"total"`
			} `json:"totals"`
		} `json:"details"`
	} `json:"data"`
}

type adjustmentCreated struct {
	Data struct {
		Action        string `json:"action"`
		Status        string `json:"status"`
		TransactionID string `json:"transaction_id"`
	} `json:"data"`
}

func parseTransactionCompleted(body []byte) (*payment.Event, error) {
	var w txCompleted
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fmt.Errorf("paddle: decode transaction.completed: %w", err)
	}
	if w.Data.Status != "completed" {
		return nil, nil
	}
	sessionID, _ := w.Data.CustomData["session_id"].(string)
	appID, _ := w.Data.CustomData["app_id"].(string)
	if sessionID == "" || appID == "" {
		return nil, errors.New("paddle: transaction.completed missing session_id or app_id in custom_data")
	}
	discountCode, _ := w.Data.CustomData["discount_code"].(string)
	consentGivenAt, _ := w.Data.CustomData["consent_given_at"].(string)

	priceCents, err := strconv.Atoi(w.Data.Details.Totals.Total)
	if err != nil {
		return nil, fmt.Errorf("paddle: parse total: %w", err)
	}

	return &payment.Event{
		Type:           "order",
		SessionID:      sessionID,
		AppID:          appID,
		Email:          w.Data.Customer.Email,
		PriceCents:     priceCents,
		DiscountCode:   discountCode,
		ConsentGivenAt: consentGivenAt,
	}, nil
}

// parseAdjustment handles refund adjustments. Paddle's adjustment webhook
// does not echo the originating transaction's custom_data, so we look up
// the original transaction via the SDK to recover our session_id.
func (p *Provider) parseAdjustment(ctx context.Context, body []byte) (*payment.Event, error) {
	var a adjustmentCreated
	if err := json.Unmarshal(body, &a); err != nil {
		return nil, fmt.Errorf("paddle: decode adjustment.created: %w", err)
	}
	if a.Data.Action != "refund" {
		return nil, nil
	}
	if a.Data.Status != "approved" && a.Data.Status != "pending_approval" {
		return nil, nil
	}
	if a.Data.TransactionID == "" {
		return nil, errors.New("paddle: adjustment missing transaction_id")
	}

	client, err := sdk.New(p.apiKey, sdk.WithBaseURL(p.baseURL()))
	if err != nil {
		return nil, fmt.Errorf("paddle: client: %w", err)
	}
	tx, err := client.GetTransaction(ctx, &sdk.GetTransactionRequest{TransactionID: a.Data.TransactionID})
	if err != nil {
		return nil, fmt.Errorf("paddle: get transaction: %w", err)
	}

	sessionID := customDataString(tx, "session_id")
	if sessionID == "" {
		return nil, errors.New("paddle: refund lookup missing session_id in original custom_data")
	}

	return &payment.Event{
		Type:      "refund",
		SessionID: sessionID,
	}, nil
}

// --- small helpers ---

// extractCheckoutURL pulls the hosted-checkout URL off a Paddle Transaction.
// Paddle returns the URL under data.checkout.url in the wire format; the SDK
// mirrors that as tx.Checkout.URL (*string).
func extractCheckoutURL(tx *sdk.Transaction) string {
	if tx == nil {
		return ""
	}
	if tx.Checkout != nil && tx.Checkout.URL != nil {
		return *tx.Checkout.URL
	}
	return ""
}

// customDataString reads a string key off a Transaction's custom_data.
// sdk.CustomData is a value type (map[string]any).
func customDataString(tx *sdk.Transaction, key string) string {
	if tx == nil || tx.CustomData == nil {
		return ""
	}
	v, _ := tx.CustomData[key].(string)
	return v
}

// newSessionID generates a UUID v4 — matching the session_id format used by
// the mock provider, so orders.payment_session has a consistent shape across
// providers.
func newSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand.Read failed: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
