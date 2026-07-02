package handlers_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/SteVio89/stevio-home/db/queries"
)

// setupCheckoutEnv initialises a test environment with the mock payment provider enabled.
func setupCheckoutEnv(t *testing.T) (*testEnv, string) {
	t.Helper()
	env := setupTestEnv(t)
	ctx := context.Background()
	// Enable mock payment provider so the MockComplete handler is active.
	if _, err := env.db.ExecContext(ctx,
		`INSERT INTO site_settings (key, value) VALUES ('payment_provider', 'mock')
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`); err != nil {
		t.Fatalf("set payment_provider: %v", err)
	}
	appID := seedApp(t, env.db)
	return env, appID
}

// TestCheckoutHappyPath verifies that completing a checkout creates exactly one
// order and one license in the database (HARD-03). Exercises the full mock
// webhook round-trip: sign → processWebhook → ParseWebhook → fulfillOrder.
func TestCheckoutHappyPath(t *testing.T) {
	env, appID := setupCheckoutEnv(t)
	ctx := context.Background()

	rec := doJSON(t, env.handler, "GET",
		"/api/checkout/mock/trigger?action=pay&session_id=sess-happy&app_id="+appID+"&email=buyer@test.com",
		nil)
	if rec.Code != 302 {
		t.Fatalf("expected 302 redirect, got %d: %s", rec.Code, rec.Body.String())
	}

	var orderCount int
	if err := env.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM orders WHERE payment_session = 'sess-happy'`).Scan(&orderCount); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if orderCount != 1 {
		t.Errorf("expected 1 order, got %d", orderCount)
	}

	var licenseCount int
	if err := env.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM licenses l
		JOIN orders o ON l.order_id = o.id
		WHERE o.payment_session = 'sess-happy'`).Scan(&licenseCount); err != nil {
		t.Fatalf("count licenses: %v", err)
	}
	if licenseCount != 1 {
		t.Errorf("expected 1 license, got %d", licenseCount)
	}
}

// TestCheckoutDuplicateWebhook verifies that sending the same session_id twice
// results in exactly one order and one license — idempotent webhook delivery (HARD-01).
func TestCheckoutDuplicateWebhook(t *testing.T) {
	env, appID := setupCheckoutEnv(t)
	ctx := context.Background()

	url := "/api/checkout/mock/trigger?action=pay&session_id=sess-dup&app_id=" + appID + "&email=buyer@test.com"

	rec1 := doJSON(t, env.handler, "GET", url, nil)
	if rec1.Code != 302 {
		t.Fatalf("first call: expected 302, got %d: %s", rec1.Code, rec1.Body.String())
	}

	// Second call with same session_id — must not produce another order or license.
	rec2 := doJSON(t, env.handler, "GET", url, nil)
	if rec2.Code != 302 {
		t.Fatalf("second call: expected 302, got %d: %s", rec2.Code, rec2.Body.String())
	}

	var orderCount int
	if err := env.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM orders WHERE payment_session = 'sess-dup'`).Scan(&orderCount); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if orderCount != 1 {
		t.Errorf("expected exactly 1 order after duplicate delivery, got %d", orderCount)
	}

	var licenseCount int
	if err := env.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM licenses l
		JOIN orders o ON l.order_id = o.id
		WHERE o.payment_session = 'sess-dup'`).Scan(&licenseCount); err != nil {
		t.Fatalf("count licenses: %v", err)
	}
	if licenseCount != 1 {
		t.Errorf("expected exactly 1 license after duplicate delivery, got %d", licenseCount)
	}
}

// TestCheckoutRefundNormal verifies that completing a checkout and then triggering
// a mock refund webhook revokes the license (HARD-04 normal path). Exercises the
// entire refund pipeline — signed envelope → ParseWebhook → handleRefund — via
// the same HTTP endpoint real flows use.
func TestCheckoutRefundNormal(t *testing.T) {
	env, appID := setupCheckoutEnv(t)
	ctx := context.Background()

	// Complete checkout to create order + license.
	rec := doJSON(t, env.handler, "GET",
		"/api/checkout/mock/trigger?action=pay&session_id=sess-refund&app_id="+appID+"&email=buyer@test.com",
		nil)
	if rec.Code != 302 {
		t.Fatalf("checkout: expected 302, got %d: %s", rec.Code, rec.Body.String())
	}

	var revokedBefore int
	if err := env.db.QueryRowContext(ctx, `
		SELECT l.revoked FROM licenses l
		JOIN orders o ON l.order_id = o.id
		WHERE o.payment_session = 'sess-refund'`).Scan(&revokedBefore); err != nil {
		t.Fatalf("query revoked before: %v", err)
	}
	if revokedBefore != 0 {
		t.Errorf("expected revoked=0 before refund, got %d", revokedBefore)
	}

	// Emit a refund webhook through the full HTTP path — same code path a real
	// Paddle adjustment.refund webhook would take.
	recR := doJSON(t, env.handler, "GET",
		"/api/checkout/mock/trigger?action=refund&session_id=sess-refund",
		nil)
	if recR.Code != 204 {
		t.Fatalf("refund: expected 204, got %d: %s", recR.Code, recR.Body.String())
	}

	var revokedAfter int
	if err := env.db.QueryRowContext(ctx, `
		SELECT l.revoked FROM licenses l
		JOIN orders o ON l.order_id = o.id
		WHERE o.payment_session = 'sess-refund'`).Scan(&revokedAfter); err != nil {
		t.Fatalf("query revoked after: %v", err)
	}
	if revokedAfter != 1 {
		t.Errorf("expected revoked=1 after refund, got %d", revokedAfter)
	}
}

// TestCheckoutRefundOutOfOrder verifies the poison pill stub pattern: a refund
// webhook arriving before the order webhook should cause the later order fulfillment
// to immediately revoke the license in the same transaction (HARD-04).
func TestCheckoutRefundOutOfOrder(t *testing.T) {
	env, appID := setupCheckoutEnv(t)
	ctx := context.Background()

	// Insert refund stub BEFORE the order arrives (simulating out-of-order delivery).
	if err := queries.InsertRefundStub(ctx, env.db, "sess-oor"); err != nil {
		t.Fatalf("InsertRefundStub: %v", err)
	}

	stubOrder, err := queries.GetOrderByPaymentSession(ctx, env.db, "sess-oor")
	if err != nil {
		t.Fatalf("GetOrderByPaymentSession: %v", err)
	}
	if stubOrder == nil {
		t.Fatal("expected stub order, got nil")
	}
	if !stubOrder.Refunded {
		t.Error("expected stub order to have Refunded=true")
	}

	rec := doJSON(t, env.handler, "GET",
		"/api/checkout/mock/trigger?action=pay&session_id=sess-oor&app_id="+appID+"&email=buyer@test.com",
		nil)
	if rec.Code != 302 {
		t.Fatalf("checkout: expected 302, got %d: %s", rec.Code, rec.Body.String())
	}

	var revoked int
	if err := env.db.QueryRowContext(ctx, `
		SELECT l.revoked FROM licenses l
		JOIN orders o ON l.order_id = o.id
		WHERE o.payment_session = 'sess-oor'`).Scan(&revoked); err != nil {
		t.Fatalf("query revoked: %v", err)
	}
	if revoked != 1 {
		t.Errorf("expected revoked=1 for out-of-order refunded order, got %d", revoked)
	}

	var orderEmail sql.NullString
	var orderAppID sql.NullString
	if err := env.db.QueryRowContext(ctx,
		`SELECT email, app_id FROM orders WHERE payment_session = 'sess-oor'`).
		Scan(&orderEmail, &orderAppID); err != nil {
		t.Fatalf("query order data: %v", err)
	}
	if !orderEmail.Valid || orderEmail.String == "" {
		t.Error("expected order email to be filled in after out-of-order fulfillment")
	}
	if !orderAppID.Valid || orderAppID.String == "" {
		t.Error("expected order app_id to be filled in after out-of-order fulfillment")
	}
}

// TestCheckoutDiscountSnapshot verifies that completing a checkout with a discount
// code records the discount information in the order row (HARD-02).
func TestCheckoutDiscountSnapshot(t *testing.T) {
	env, appID := setupCheckoutEnv(t)
	ctx := context.Background()

	_, err := queries.InsertDiscountCode(ctx, env.db, queries.InsertDiscountCodeParams{
		Code:          "SAVE10",
		Label:         "Save 10%",
		DiscountType:  "percent",
		DiscountValue: 10,
		AppID:         &appID,
	})
	if err != nil {
		t.Fatalf("InsertDiscountCode: %v", err)
	}

	rec := doJSON(t, env.handler, "GET",
		"/api/checkout/mock/trigger?action=pay&session_id=sess-disc&app_id="+appID+"&email=buyer@test.com&discount_code=SAVE10",
		nil)
	if rec.Code != 302 {
		t.Fatalf("checkout: expected 302, got %d: %s", rec.Code, rec.Body.String())
	}

	// Allow the fire-and-forget IncrementDiscountUses goroutine to complete.
	time.Sleep(100 * time.Millisecond)

	var discountLabel sql.NullString
	if err := env.db.QueryRowContext(ctx,
		`SELECT discount_label FROM orders WHERE payment_session = 'sess-disc'`).
		Scan(&discountLabel); err != nil {
		t.Fatalf("query discount_label: %v", err)
	}
	if !discountLabel.Valid || discountLabel.String == "" {
		t.Error("expected discount_label to be set in order after checkout with discount code")
	}
}

// TestMockWebhookBadSignature verifies the mock ParseWebhook rejects any POST
// to /api/payment/webhook whose X-Mock-Signature does not match the expected
// HMAC — guards against a caller forging a webhook without the signing key.
func TestMockWebhookBadSignature(t *testing.T) {
	env, appID := setupCheckoutEnv(t)
	_ = appID
	ctx := context.Background()

	body := []byte(`{"event_type":"order","session_id":"sess-bad","app_id":"whatever","price_cents":999}`)
	headers := map[string]string{
		"Content-Type":     "application/json",
		"X-Mock-Signature": "00000000000000000000000000000000000000000000000000000000000000ff",
	}
	rec := doJSONWithHeaders(t, env.handler, "POST", "/api/payment/webhook", body, headers)
	if rec.Code != 400 {
		t.Fatalf("expected 400 for bad signature, got %d: %s", rec.Code, rec.Body.String())
	}

	var count int
	if err := env.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM orders WHERE payment_session = 'sess-bad'`).Scan(&count); err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 orders after bad-signature webhook, got %d", count)
	}
}
