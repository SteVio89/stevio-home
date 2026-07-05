package paddle

import (
	"encoding/json"
	"testing"

	"github.com/SteVio89/stevio-home/payment"
)

// TestBuildTransactionItems_QuantityShape guards the non-catalog line-item JSON
// against a regression that only surfaces against the live Paddle API: the SDK's
// PriceQuantity is a struct value with a `json:"quantity,omitempty"` tag, and
// omitempty is a no-op on structs — so a zero value marshals to `"quantity":{}`,
// which Paddle rejects with "minimum/maximum is required". This test asserts the
// serialized price.quantity is a fully-specified 1..1 object, never empty.
func TestBuildTransactionItems_QuantityShape(t *testing.T) {
	items := buildTransactionItems(payment.CheckoutParams{
		AppName:      "MyApp",
		PriceCents:   1900,
		CurrencyCode: "EUR",
	}, "standard")

	raw, err := json.Marshal(items[0])
	if err != nil {
		t.Fatalf("marshal item: %v", err)
	}

	var item struct {
		Quantity int `json:"quantity"`
		Price    struct {
			Quantity *struct {
				Minimum *int `json:"minimum"`
				Maximum *int `json:"maximum"`
			} `json:"quantity"`
		} `json:"price"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatalf("unmarshal item: %v", err)
	}

	if item.Quantity != 1 {
		t.Errorf("item quantity: want 1, got %d", item.Quantity)
	}
	if item.Price.Quantity == nil {
		t.Fatalf("price.quantity missing from payload: %s", raw)
	}
	if item.Price.Quantity.Minimum == nil || item.Price.Quantity.Maximum == nil {
		t.Fatalf("price.quantity must have minimum and maximum (Paddle rejects empty {}): %s", raw)
	}
	if *item.Price.Quantity.Minimum != 1 || *item.Price.Quantity.Maximum != 1 {
		t.Errorf("price.quantity: want {1,1}, got {%d,%d}",
			*item.Price.Quantity.Minimum, *item.Price.Quantity.Maximum)
	}
}

// These tests focus on the JSON-parsing helpers that do not require the SDK
// client (network). Signature-verification behavior is fully owned by the SDK;
// we cover the error-mapping branches via a mocked verifier at a later stage
// (or an integration test that hits the webhook handler with a signed fixture).

func TestParseTransactionCompleted_Happy(t *testing.T) {
	body := []byte(`{
		"event_type": "transaction.completed",
		"data": {
			"id": "txn_01h",
			"status": "completed",
			"custom_data": {
				"app_id": "my-app",
				"session_id": "sess-123",
				"discount_code": "SAVE10",
				"consent_given_at": "2026-04-16T12:30:00Z"
			},
			"customer": { "email": "buyer@example.com" },
			"details": { "totals": { "total": "2990" } }
		}
	}`)
	ev, err := parseTransactionCompleted(body)
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
		t.Errorf("PriceCents: want 2990, got %d", ev.PriceCents)
	}
	if ev.DiscountCode != "SAVE10" {
		t.Errorf("DiscountCode: want 'SAVE10', got %q", ev.DiscountCode)
	}
	if ev.ConsentGivenAt != "2026-04-16T12:30:00Z" {
		t.Errorf("ConsentGivenAt: want '2026-04-16T12:30:00Z', got %q", ev.ConsentGivenAt)
	}
}

func TestParseTransactionCompleted_NotCompleted(t *testing.T) {
	body := []byte(`{
		"event_type": "transaction.completed",
		"data": {
			"status": "past_due",
			"custom_data": {"app_id":"a","session_id":"s"},
			"customer": {"email":"x@example.com"},
			"details": {"totals":{"total":"0"}}
		}
	}`)
	ev, err := parseTransactionCompleted(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ev != nil {
		t.Errorf("expected nil for non-completed status, got %+v", ev)
	}
}

func TestParseTransactionCompleted_MissingCustomData(t *testing.T) {
	body := []byte(`{
		"data": {
			"status": "completed",
			"custom_data": {},
			"customer": {"email":"x@example.com"},
			"details": {"totals":{"total":"100"}}
		}
	}`)
	if _, err := parseTransactionCompleted(body); err == nil {
		t.Fatal("expected error for missing session_id/app_id")
	}
}

func TestParseTransactionCompleted_NonIntTotal(t *testing.T) {
	body := []byte(`{
		"data": {
			"status": "completed",
			"custom_data": {"app_id":"a","session_id":"s"},
			"customer": {"email":"x@example.com"},
			"details": {"totals":{"total":"oops"}}
		}
	}`)
	if _, err := parseTransactionCompleted(body); err == nil {
		t.Fatal("expected error parsing non-integer total")
	}
}
