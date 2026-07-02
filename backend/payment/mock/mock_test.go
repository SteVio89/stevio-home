package mock

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/SteVio89/stevio-home/payment"
)

var testSecret = [32]byte{
	0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
	0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10,
	0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18,
	0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20,
}

func TestEnvelopeOrderRoundTrip(t *testing.T) {
	p := New("http://localhost:3000", testSecret)
	env := Envelope{
		EventType:      "order",
		SessionID:      "sess-abc",
		AppID:          "app-xyz",
		Email:          "buyer@example.com",
		PriceCents:     1999,
		CurrencyCode:   "EUR",
		DiscountCode:   "SAVE10",
		ConsentGivenAt: "2026-04-16T12:00:00Z",
	}
	body, _ := json.Marshal(env)

	headers := http.Header{}
	headers.Set(SignatureHeader, Sign(DeriveSecret(testSecret), body))

	ev, err := p.ParseWebhook(body, headers)
	if err != nil {
		t.Fatalf("ParseWebhook: %v", err)
	}
	if ev == nil {
		t.Fatal("expected event, got nil")
	}
	if ev.Type != "order" || ev.SessionID != "sess-abc" || ev.AppID != "app-xyz" {
		t.Errorf("event mismatch: %+v", ev)
	}
	if ev.Email != "buyer@example.com" || ev.PriceCents != 1999 {
		t.Errorf("event mismatch: %+v", ev)
	}
	if ev.DiscountCode != "SAVE10" || ev.ConsentGivenAt != "2026-04-16T12:00:00Z" {
		t.Errorf("event mismatch: %+v", ev)
	}
}

func TestEnvelopeRefundRoundTrip(t *testing.T) {
	p := New("", testSecret)
	body, _ := json.Marshal(Envelope{EventType: "refund", SessionID: "sess-abc"})
	headers := http.Header{}
	headers.Set(SignatureHeader, Sign(DeriveSecret(testSecret), body))

	ev, err := p.ParseWebhook(body, headers)
	if err != nil || ev == nil {
		t.Fatalf("ParseWebhook: ev=%v err=%v", ev, err)
	}
	if ev.Type != "refund" || ev.SessionID != "sess-abc" {
		t.Errorf("event mismatch: %+v", ev)
	}
}

func TestSignatureMismatch(t *testing.T) {
	p := New("", testSecret)
	body, _ := json.Marshal(Envelope{EventType: "order", SessionID: "s", AppID: "a"})

	// Sign with a different secret.
	other := [32]byte{}
	headers := http.Header{}
	headers.Set(SignatureHeader, Sign(DeriveSecret(other), body))

	if _, err := p.ParseWebhook(body, headers); err != ErrSignatureInvalid {
		t.Fatalf("want ErrSignatureInvalid, got %v", err)
	}
}

func TestMissingSignatureHeader(t *testing.T) {
	p := New("", testSecret)
	body, _ := json.Marshal(Envelope{EventType: "order", SessionID: "s", AppID: "a"})
	if _, err := p.ParseWebhook(body, http.Header{}); err != ErrSignatureInvalid {
		t.Fatalf("want ErrSignatureInvalid, got %v", err)
	}
}

func TestMalformedSignatureEncoding(t *testing.T) {
	p := New("", testSecret)
	body, _ := json.Marshal(Envelope{EventType: "order", SessionID: "s", AppID: "a"})
	h := http.Header{}
	h.Set(SignatureHeader, "not-hex-xyz")
	if _, err := p.ParseWebhook(body, h); err != ErrSignatureInvalid {
		t.Fatalf("want ErrSignatureInvalid, got %v", err)
	}
}

func TestUnknownEventTypeReturnsNil(t *testing.T) {
	p := New("", testSecret)
	body, _ := json.Marshal(Envelope{EventType: "something.else", SessionID: "s"})
	h := http.Header{}
	h.Set(SignatureHeader, Sign(DeriveSecret(testSecret), body))

	ev, err := p.ParseWebhook(body, h)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ev != nil {
		t.Errorf("expected nil for unknown event, got %+v", ev)
	}
}

func TestOrderMissingAppIDRejected(t *testing.T) {
	p := New("", testSecret)
	body, _ := json.Marshal(Envelope{EventType: "order", SessionID: "s"}) // no AppID
	h := http.Header{}
	h.Set(SignatureHeader, Sign(DeriveSecret(testSecret), body))

	if _, err := p.ParseWebhook(body, h); err == nil {
		t.Fatal("expected error for missing app_id, got nil")
	}
}

func TestCreateCheckoutURLContainsFields(t *testing.T) {
	p := New("http://localhost:3000", testSecret)
	sess, err := p.CreateCheckout(context.Background(), payment.CheckoutParams{
		AppID:        "app-1",
		AppName:      "Example",
		PriceCents:   2990,
		CurrencyCode: "EUR",
		DiscountCode: "SAVE5",
	})
	if err != nil {
		t.Fatalf("CreateCheckout: %v", err)
	}
	if sess.SessionID == "" {
		t.Error("empty SessionID")
	}
	// The URL should be a same-origin /mock-checkout path carrying the params.
	wantPrefix := "/mock-checkout?"
	if sess.URL[:len(wantPrefix)] != wantPrefix {
		t.Errorf("URL prefix: got %q, want %q…", sess.URL, wantPrefix)
	}
	for _, want := range []string{"app_id=app-1", "price_cents=2990", "currency_code=EUR", "discount_code=SAVE5"} {
		if !contains(sess.URL, want) {
			t.Errorf("URL missing %q: %s", want, sess.URL)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
