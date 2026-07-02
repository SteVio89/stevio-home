package paddle

import (
	"testing"
)

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
