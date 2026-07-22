// Package polar implements payment.Provider for Polar (https://polar.sh), a
// Merchant of Record. Like the Paddle provider, prices live in Postgres on the
// apps table and are sent to Polar per-checkout — never mirrored into a
// provider-side catalog the admin has to maintain.
//
// Polar differs from Paddle in one structural way: an ad-hoc checkout price must
// be attached to a product_id, so Polar cannot do fully catalog-free checkouts.
// We bridge that by lazily creating exactly one Polar product per app (named
// after the app) the first time that app is sold under Polar, caching the
// product id in the provider_products table via the injected ProductStore. The
// price shown to the buyer is always an ad-hoc price built from our DB value, so
// the product's own catalog price is a throwaway placeholder. Product creation
// only ever happens inside a real CreateCheckout call while Polar is the active
// provider — nothing is coupled to app publishing, and switching providers
// leaves the cached rows inert.
//
// Checkout is a redirect flow: CreateCheckout returns the hosted-checkout URL in
// CheckoutSession.URL (the same slot the mock provider uses), so no client-side
// SDK or token is needed. Webhooks are verified per the Standard Webhooks spec
// (https://www.standardwebhooks.com) that Polar follows.
package polar

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/SteVio89/stevio-home/payment"
)

const (
	envSandbox    = "sandbox"
	envProduction = "production"

	apiSandbox    = "https://sandbox-api.polar.sh/v1"
	apiProduction = "https://api.polar.sh/v1"

	// verifyTolerance bounds the webhook timestamp skew we accept, matching the
	// Standard Webhooks recommendation and the Paddle provider's window.
	verifyTolerance = 5 * time.Minute

	requestTimeout = 20 * time.Second
)

// ErrSignatureInvalid is returned by ParseWebhook for any signature-related
// failure (missing headers, timestamp outside tolerance, or HMAC mismatch)
// without leaking which specific check failed.
var ErrSignatureInvalid = errors.New("polar: webhook signature invalid")

// ProductStore persists the app → Polar-product-id mapping. It is injected at
// construction (the checkout handler backs it with the provider_products table)
// so the provider stays free of direct DB coupling and is unit-testable.
type ProductStore interface {
	// GetProductID returns the cached Polar product id for an app in the given
	// environment, or "" if none. Sandbox and production are separate Polar
	// catalogs, so the environment is part of the cache key.
	GetProductID(ctx context.Context, appID, environment string) (string, error)
	// SaveProductID records the Polar product id created for an app in the given
	// environment.
	SaveProductID(ctx context.Context, appID, environment, productID string) error
}

// Provider implements payment.Provider for Polar.
type Provider struct {
	apiKey        string
	webhookSecret string
	environment   string
	store         ProductStore
	httpClient    *http.Client
}

// New returns a Polar provider. Passing empty credentials and a nil store is
// valid — the placeholder instance registered in main.go exists only so the
// registry knows "polar" is a supported provider name. Any real CreateCheckout /
// ParseWebhook call receives a freshly-constructed provider with the
// admin-configured credentials and a live store.
func New(apiKey, webhookSecret, environment string, store ProductStore) *Provider {
	if environment == "" {
		environment = envProduction
	}
	return &Provider{
		apiKey:        apiKey,
		webhookSecret: webhookSecret,
		environment:   environment,
		store:         store,
		httpClient:    &http.Client{Timeout: requestTimeout},
	}
}

func (p *Provider) Name() string { return "polar" }

func (p *Provider) baseURL() string {
	if p.environment == envSandbox {
		return apiSandbox
	}
	return apiProduction
}

// CreateCheckout ensures a Polar product exists for the app, then creates a
// checkout session whose price is an ad-hoc override built from params (our DB
// price after discounts). It returns the hosted-checkout URL for a redirect and
// the session id we stamped into the checkout metadata — that id is echoed back
// on the order.paid webhook and becomes the order's payment_session.
func (p *Provider) CreateCheckout(ctx context.Context, params payment.CheckoutParams) (payment.CheckoutSession, error) {
	if p.apiKey == "" {
		return payment.CheckoutSession{}, errors.New("polar: api_key is required")
	}
	if params.AppName == "" || params.PriceCents <= 0 || params.CurrencyCode == "" {
		return payment.CheckoutSession{}, errors.New("polar: app_name, price_cents, and currency_code are required")
	}
	if p.store == nil {
		return payment.CheckoutSession{}, errors.New("polar: product store not configured")
	}

	// Polar expects ISO 4217 codes in lower case (e.g. "eur").
	currency := strings.ToLower(params.CurrencyCode)

	productID, err := p.ensureProduct(ctx, params, currency)
	if err != nil {
		return payment.CheckoutSession{}, err
	}

	sessionID := newSessionID()

	metadata := map[string]string{
		"session_id": sessionID,
		"app_id":     params.AppID,
	}
	if params.DiscountCode != "" {
		metadata["discount_code"] = params.DiscountCode
	}
	if params.ConsentGivenAt != "" {
		metadata["consent_given_at"] = params.ConsentGivenAt
	}

	reqBody := map[string]any{
		"products": []string{productID},
		// Ad-hoc price: overrides the product's catalog price for this checkout
		// only and never enters the catalog. This is what keeps pricing authority
		// (discounts, currency) in our DB rather than in Polar.
		"prices": map[string]any{
			productID: []map[string]any{{
				"amount_type":    "fixed",
				"price_amount":   params.PriceCents,
				"price_currency": currency,
			}},
		},
		"metadata": metadata,
		// Our SuccessURL carries a {SESSION_ID} placeholder Polar wouldn't
		// interpolate; substitute our own session id so the success page can
		// verify fulfillment by it.
		"success_url": strings.ReplaceAll(params.SuccessURL, "{SESSION_ID}", sessionID),
	}

	var out struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}
	if err := p.apiPost(ctx, "/checkouts/", reqBody, &out); err != nil {
		return payment.CheckoutSession{}, fmt.Errorf("polar: create checkout: %w", err)
	}
	if out.URL == "" {
		return payment.CheckoutSession{}, errors.New("polar: checkout response missing url")
	}

	return payment.CheckoutSession{
		URL:       out.URL,
		SessionID: sessionID,
	}, nil
}

// ensureProduct returns the Polar product id for the app, creating it lazily on
// first use and caching it via the store. The catalog price set at creation is a
// nominal placeholder — every checkout overrides it with an ad-hoc price — so its
// exact value is irrelevant beyond satisfying Polar's "a product needs a price"
// requirement.
//
// A rare race (two concurrent first-checkouts for the same app) can create two
// Polar products; the store's upsert keeps the latest and the other is a
// harmless orphan in the Polar dashboard. Not worth locking for.
func (p *Provider) ensureProduct(ctx context.Context, params payment.CheckoutParams, currency string) (string, error) {
	id, err := p.store.GetProductID(ctx, params.AppID, p.environment)
	if err != nil {
		return "", fmt.Errorf("polar: lookup product for app %s: %w", params.AppID, err)
	}
	if id != "" {
		return id, nil
	}

	reqBody := map[string]any{
		"name": params.AppName,
		// One-time product (no subscription). Omitting organization_id is
		// required: setting it is rejected when authenticating with an
		// organization access token.
		"recurring_interval": nil,
		"prices": []map[string]any{{
			"amount_type":    "fixed",
			"price_amount":   params.PriceCents,
			"price_currency": currency,
		}},
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := p.apiPost(ctx, "/products/", reqBody, &out); err != nil {
		return "", fmt.Errorf("polar: create product: %w", err)
	}
	if out.ID == "" {
		return "", errors.New("polar: product response missing id")
	}
	if err := p.store.SaveProductID(ctx, params.AppID, p.environment, out.ID); err != nil {
		return "", fmt.Errorf("polar: save product mapping: %w", err)
	}
	return out.ID, nil
}

// ParseWebhook verifies the Standard Webhooks signature and returns a typed
// Event for order.paid (fulfillment) and order.refunded (revocation); nil for
// any other event type.
func (p *Provider) ParseWebhook(body []byte, headers http.Header) (*payment.Event, error) {
	if err := p.verifyWebhook(body, headers); err != nil {
		return nil, err
	}

	var env struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("polar: decode webhook envelope: %w", err)
	}

	switch env.Type {
	case "order.paid":
		return parseOrderPaid(body)
	case "order.refunded":
		return parseOrderRefunded(body)
	}
	// Every other event (order.created, checkout.*, refund.created, …) is
	// ignored by design — order.refunded already carries the order metadata we
	// need, and handleRefund is idempotent.
	return nil, nil
}

// --- webhook signature (Standard Webhooks) ---

// verifyWebhook validates the webhook-id / webhook-timestamp / webhook-signature
// headers per the Standard Webhooks spec: HMAC-SHA256 over "id.timestamp.body",
// base64-encoded, compared against each space-delimited "v1,<sig>" token.
func (p *Provider) verifyWebhook(body []byte, headers http.Header) error {
	if p.webhookSecret == "" {
		return errors.New("polar: webhook_secret not configured")
	}

	id := headers.Get("webhook-id")
	ts := headers.Get("webhook-timestamp")
	sigHeader := headers.Get("webhook-signature")
	if id == "" || ts == "" || sigHeader == "" {
		return ErrSignatureInvalid
	}

	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return ErrSignatureInvalid
	}
	tolSec := int64(verifyTolerance / time.Second)
	if diff := time.Now().Unix() - tsInt; diff > tolSec || diff < -tolSec {
		return ErrSignatureInvalid
	}

	expected := computeSignature(decodeSecret(p.webhookSecret), id, ts, body)

	// The header is a space-delimited list of "version,signature" tokens.
	for _, tok := range strings.Split(sigHeader, " ") {
		version, sig, ok := strings.Cut(tok, ",")
		if !ok || version != "v1" {
			continue
		}
		if hmac.Equal([]byte(sig), []byte(expected)) {
			return nil
		}
	}
	return ErrSignatureInvalid
}

// decodeSecret resolves the configured webhook secret to its raw HMAC key.
//
// Although Polar follows Standard Webhooks (whose secrets are "whsec_"+base64
// and are base64-decoded before use), Polar deviates in one crucial way: its
// SDK base64-*encodes* the dashboard secret before handing it to the library,
// which then base64-decodes it straight back — so Polar's effective HMAC key is
// the raw UTF-8 bytes of the secret string exactly as shown in the dashboard.
// We must therefore use the secret verbatim: base64-decoding it (the plain
// Standard Webhooks behaviour) yields the wrong key and every signature fails
// whenever the secret happens to be valid base64, which Polar's alphanumeric
// secrets usually are. See polar-js/src/webhooks.ts (validateEvent).
func decodeSecret(secret string) []byte {
	return []byte(secret)
}

func computeSignature(key []byte, id, ts string, body []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id))
	mac.Write([]byte("."))
	mac.Write([]byte(ts))
	mac.Write([]byte("."))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// Sign produces the base64 Standard Webhooks signature for a payload under the
// given secret. Exported for tests that POST signed fixtures at ParseWebhook.
func Sign(secret, id, ts string, body []byte) string {
	return computeSignature(decodeSecret(secret), id, ts, body)
}

// --- event parsing ---

// orderPayload is a minimal typed view over the order.paid / order.refunded
// webhook bodies — just the fields we consume. net_amount is the amount after
// discounts and before taxes; since Polar never sees our discounts (we send the
// already-discounted price as an ad-hoc amount) it equals what we charged,
// unaffected by any tax Polar adds on top.
type orderPayload struct {
	Data struct {
		NetAmount int            `json:"net_amount"`
		Metadata  map[string]any `json:"metadata"`
		Customer  struct {
			Email string `json:"email"`
		} `json:"customer"`
	} `json:"data"`
}

func parseOrderPaid(body []byte) (*payment.Event, error) {
	var w orderPayload
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fmt.Errorf("polar: decode order.paid: %w", err)
	}
	sessionID := metaString(w.Data.Metadata, "session_id")
	appID := metaString(w.Data.Metadata, "app_id")
	if sessionID == "" || appID == "" {
		return nil, errors.New("polar: order.paid missing session_id or app_id in metadata")
	}
	return &payment.Event{
		Type:           "order",
		SessionID:      sessionID,
		AppID:          appID,
		Email:          w.Data.Customer.Email,
		PriceCents:     w.Data.NetAmount,
		DiscountCode:   metaString(w.Data.Metadata, "discount_code"),
		ConsentGivenAt: metaString(w.Data.Metadata, "consent_given_at"),
	}, nil
}

func parseOrderRefunded(body []byte) (*payment.Event, error) {
	var w orderPayload
	if err := json.Unmarshal(body, &w); err != nil {
		return nil, fmt.Errorf("polar: decode order.refunded: %w", err)
	}
	// Unlike Paddle, Polar echoes the original order metadata on the refund
	// event, so we recover session_id directly with no secondary API lookup.
	sessionID := metaString(w.Data.Metadata, "session_id")
	if sessionID == "" {
		return nil, errors.New("polar: order.refunded missing session_id in metadata")
	}
	return &payment.Event{Type: "refund", SessionID: sessionID}, nil
}

func metaString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// --- HTTP + helpers ---

func (p *Provider) apiPost(ctx context.Context, path string, reqBody, out any) error {
	buf, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL()+path, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("api %s: status %d: %s", path, resp.StatusCode, truncate(respBody, 300))
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}

// newSessionID generates a UUID v4 — matching the session_id format used by the
// other providers so orders.payment_session has a consistent shape.
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
