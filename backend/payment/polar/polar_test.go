package polar

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"testing"
	"time"
)

// signedHeaders builds the Standard Webhooks headers a valid Polar delivery
// carries, signing with the given secret at the current time.
func signedHeaders(secret string, body []byte) http.Header {
	id := "msg_test_123"
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	h := http.Header{}
	h.Set("webhook-id", id)
	h.Set("webhook-timestamp", ts)
	h.Set("webhook-signature", "v1,"+Sign(secret, id, ts, body))
	return h
}

// A base64 secret, as Standard Webhooks / Polar issue them.
var testSecret = "whsec_" + base64.StdEncoding.EncodeToString([]byte("super-secret-key-material"))

func newTestProvider() *Provider {
	return New("", testSecret, envSandbox, nil)
}

func TestParseWebhook_OrderPaid(t *testing.T) {
	body := []byte(`{
		"type": "order.paid",
		"data": {
			"net_amount": 2990,
			"customer": { "email": "buyer@example.com" },
			"metadata": {
				"session_id": "sess-123",
				"app_id": "my-app",
				"discount_code": "SAVE10",
				"consent_given_at": "2026-04-16T12:30:00Z"
			}
		}
	}`)
	ev, err := newTestProvider().ParseWebhook(body, signedHeaders(testSecret, body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != "order" {
		t.Errorf("Type: want 'order', got %q", ev.Type)
	}
	if ev.SessionID != "sess-123" {
		t.Errorf("SessionID: want 'sess-123', got %q", ev.SessionID)
	}
	if ev.AppID != "my-app" {
		t.Errorf("AppID: want 'my-app', got %q", ev.AppID)
	}
	if ev.Email != "buyer@example.com" {
		t.Errorf("Email: want 'buyer@example.com', got %q", ev.Email)
	}
	if ev.PriceCents != 2990 {
		t.Errorf("PriceCents: want 2990 (net_amount), got %d", ev.PriceCents)
	}
	if ev.DiscountCode != "SAVE10" {
		t.Errorf("DiscountCode: want 'SAVE10', got %q", ev.DiscountCode)
	}
	if ev.ConsentGivenAt != "2026-04-16T12:30:00Z" {
		t.Errorf("ConsentGivenAt: want '2026-04-16T12:30:00Z', got %q", ev.ConsentGivenAt)
	}
}

func TestParseWebhook_OrderRefunded(t *testing.T) {
	body := []byte(`{
		"type": "order.refunded",
		"data": { "net_amount": 2990, "metadata": { "session_id": "sess-123", "app_id": "my-app" } }
	}`)
	ev, err := newTestProvider().ParseWebhook(body, signedHeaders(testSecret, body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev == nil {
		t.Fatal("expected refund event, got nil")
	}
	if ev.Type != "refund" {
		t.Errorf("Type: want 'refund', got %q", ev.Type)
	}
	if ev.SessionID != "sess-123" {
		t.Errorf("SessionID: want 'sess-123', got %q", ev.SessionID)
	}
}

func TestParseWebhook_IgnoredEventType(t *testing.T) {
	body := []byte(`{"type":"order.created","data":{"metadata":{"session_id":"s","app_id":"a"}}}`)
	ev, err := newTestProvider().ParseWebhook(body, signedHeaders(testSecret, body))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev != nil {
		t.Errorf("expected nil for ignored event type, got %+v", ev)
	}
}

func TestParseWebhook_MissingMetadata(t *testing.T) {
	body := []byte(`{"type":"order.paid","data":{"net_amount":100,"metadata":{}}}`)
	if _, err := newTestProvider().ParseWebhook(body, signedHeaders(testSecret, body)); err == nil {
		t.Fatal("expected error for missing session_id/app_id in metadata")
	}
}

func TestParseWebhook_BadSignature(t *testing.T) {
	body := []byte(`{"type":"order.paid","data":{"net_amount":100,"metadata":{"session_id":"s","app_id":"a"}}}`)
	h := signedHeaders(testSecret, body)
	// Tamper the body after signing — the HMAC must no longer match.
	tampered := append(body[:len(body)-1], []byte(`, "x":1}`)...)
	if _, err := newTestProvider().ParseWebhook(tampered, h); err != ErrSignatureInvalid {
		t.Fatalf("want ErrSignatureInvalid for tampered body, got %v", err)
	}
}

func TestParseWebhook_WrongSecret(t *testing.T) {
	body := []byte(`{"type":"order.paid","data":{"net_amount":100,"metadata":{"session_id":"s","app_id":"a"}}}`)
	h := signedHeaders("whsec_"+base64.StdEncoding.EncodeToString([]byte("a-different-key")), body)
	if _, err := newTestProvider().ParseWebhook(body, h); err != ErrSignatureInvalid {
		t.Fatalf("want ErrSignatureInvalid for wrong secret, got %v", err)
	}
}

func TestParseWebhook_StaleTimestamp(t *testing.T) {
	body := []byte(`{"type":"order.paid","data":{"net_amount":100,"metadata":{"session_id":"s","app_id":"a"}}}`)
	id := "msg_test_123"
	ts := strconv.FormatInt(time.Now().Add(-30*time.Minute).Unix(), 10)
	h := http.Header{}
	h.Set("webhook-id", id)
	h.Set("webhook-timestamp", ts)
	h.Set("webhook-signature", "v1,"+Sign(testSecret, id, ts, body))
	if _, err := newTestProvider().ParseWebhook(body, h); err != ErrSignatureInvalid {
		t.Fatalf("want ErrSignatureInvalid for stale timestamp, got %v", err)
	}
}

// decodeSecret must use the configured secret verbatim as the HMAC key, because
// Polar signs with the raw bytes of the dashboard secret (it base64-encodes then
// the library base64-decodes, cancelling out). Base64-decoding here would derive
// the wrong key and break verification for any secret that is valid base64.
func TestDecodeSecret_Verbatim(t *testing.T) {
	// An arbitrary custom string is used byte-for-byte.
	raw := "not+valid+base64+@@@"
	if got := decodeSecret(raw); string(got) != raw {
		t.Errorf("raw secret: want %q, got %q", raw, string(got))
	}
	// A secret that happens to be valid base64 is NOT decoded — this is the case
	// that previously broke real Polar deliveries.
	b64 := base64.StdEncoding.EncodeToString([]byte("abc")) // "YWJj", valid base64
	if got := decodeSecret(b64); string(got) != b64 {
		t.Errorf("valid-base64 secret must stay verbatim: want %q, got %q", b64, string(got))
	}
}
