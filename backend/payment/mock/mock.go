// Package mock implements a payment.Provider that simulates a real provider's
// webhook flow locally. It does not contact any external service, but it DOES
// exercise the same verification + ParseWebhook + fulfillOrder code path that
// the real Paddle provider uses — a payment "completes" by POSTing an
// HMAC-SHA256-signed JSON envelope to /api/payment/webhook and going through
// the exact same handler chain real Paddle webhooks trigger.
//
// The webhook secret is derived deterministically from SIGNING_KEY_SECRET via
// HMAC-SHA256 with the domain-separation label "mock_webhook_secret", so no
// additional admin-managed setting is required.
//
// Envelope shape is intentionally NOT Paddle-bit-compatible. The mock owns
// its own wire format (small, flat, easy to hand-construct in tests) and the
// signature header is X-Mock-Signature, so a Paddle-signed payload cannot
// accidentally be accepted by this provider and vice versa.
package mock

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"context"

	"github.com/SteVio89/stevio-home/payment"
)

// SignatureHeader is the HTTP header carrying the hex-encoded HMAC-SHA256
// signature of the raw request body. Exported so the MockComplete handler
// can set it on the synthetic webhook request it produces.
const SignatureHeader = "X-Mock-Signature"

// ErrSignatureInvalid is returned by ParseWebhook for any signature-related
// failure: missing header, wrong encoding, or HMAC mismatch.
var ErrSignatureInvalid = errors.New("mock: webhook signature invalid")

// Envelope is the mock webhook wire format. Kept flat and self-contained so
// a test can build one by hand with no SDK dependency.
type Envelope struct {
	EventType      string `json:"event_type"` // "order" | "refund"
	SessionID      string `json:"session_id"`
	AppID          string `json:"app_id,omitempty"` // omitted for refund
	Email          string `json:"email,omitempty"`  // omitted for refund
	PriceCents     int    `json:"price_cents,omitempty"`
	CurrencyCode   string `json:"currency_code,omitempty"`
	DiscountCode   string `json:"discount_code,omitempty"`
	ConsentGivenAt string `json:"consent_given_at,omitempty"`
}

// Provider implements payment.Provider with no external API calls.
type Provider struct {
	baseURL       string // e.g. "http://localhost:3000"
	webhookSecret []byte // HMAC-SHA256(signingKeySecret, "mock_webhook_secret")
}

// New returns a mock provider whose webhook secret is derived from the given
// signing-key secret via HMAC-SHA256 with a fixed domain-separation label.
// The same signingKeySecret must be used on the signing side (the MockComplete
// handler) so the derived secrets match.
func New(baseURL string, signingKeySecret [32]byte) *Provider {
	return &Provider{
		baseURL:       baseURL,
		webhookSecret: DeriveSecret(signingKeySecret),
	}
}

// DeriveSecret produces the deterministic mock webhook secret from the
// SIGNING_KEY_SECRET. Exported so the MockComplete handler can sign envelopes
// with exactly the same key the provider verifies against.
func DeriveSecret(signingKeySecret [32]byte) []byte {
	mac := hmac.New(sha256.New, signingKeySecret[:])
	mac.Write([]byte("mock_webhook_secret"))
	return mac.Sum(nil)
}

// Sign computes the hex-encoded HMAC-SHA256 of body under secret. Used both
// by the MockComplete handler to produce X-Mock-Signature and by tests that
// POST signed envelopes directly.
func Sign(secret, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func (p *Provider) Name() string { return "mock" }

// CreateCheckout returns a URL to the frontend /mock-checkout page carrying
// the data the page needs to render and to build the completion request.
// No external call is made.
func (p *Provider) CreateCheckout(_ context.Context, params payment.CheckoutParams) (payment.CheckoutSession, error) {
	sessionID := newSessionID()

	q := url.Values{}
	q.Set("session_id", sessionID)
	q.Set("app_id", params.AppID)
	q.Set("app_name", params.AppName)
	q.Set("price_cents", strconv.Itoa(params.PriceCents))
	q.Set("currency_code", params.CurrencyCode)
	if params.DiscountCode != "" {
		q.Set("discount_code", params.DiscountCode)
	}
	if params.ConsentGivenAt != "" {
		q.Set("consent_given_at", params.ConsentGivenAt)
	}

	// Frontend route — intentionally relative so the same URL works in dev,
	// preview, and behind the prod reverse proxy.
	checkoutURL := "/mock-checkout?" + q.Encode()

	return payment.CheckoutSession{
		URL:       checkoutURL,
		SessionID: sessionID,
	}, nil
}

// ParseWebhook verifies the X-Mock-Signature header against the raw body and
// decodes the Envelope into a payment.Event. Signature failures return
// ErrSignatureInvalid without leaking which check failed.
func (p *Provider) ParseWebhook(body []byte, headers http.Header) (*payment.Event, error) {
	if len(p.webhookSecret) == 0 {
		return nil, errors.New("mock: webhook secret not initialized")
	}
	sigHex := headers.Get(SignatureHeader)
	if sigHex == "" {
		return nil, ErrSignatureInvalid
	}
	sig, err := hex.DecodeString(sigHex)
	if err != nil {
		return nil, ErrSignatureInvalid
	}
	mac := hmac.New(sha256.New, p.webhookSecret)
	mac.Write(body)
	if !hmac.Equal(mac.Sum(nil), sig) {
		return nil, ErrSignatureInvalid
	}

	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("mock: decode envelope: %w", err)
	}

	switch env.EventType {
	case "order":
		if env.SessionID == "" || env.AppID == "" {
			return nil, errors.New("mock: order envelope missing session_id or app_id")
		}
		return &payment.Event{
			Type:           "order",
			SessionID:      env.SessionID,
			AppID:          env.AppID,
			Email:          env.Email,
			PriceCents:     env.PriceCents,
			DiscountCode:   env.DiscountCode,
			ConsentGivenAt: env.ConsentGivenAt,
		}, nil
	case "refund":
		if env.SessionID == "" {
			return nil, errors.New("mock: refund envelope missing session_id")
		}
		return &payment.Event{
			Type:      "refund",
			SessionID: env.SessionID,
		}, nil
	}
	// Unknown event types are ignored by design — matches Paddle's contract.
	return nil, nil
}

func newSessionID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand.Read failed: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
