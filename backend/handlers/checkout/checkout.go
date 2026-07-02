package checkout

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/auth"
	"github.com/SteVio89/stevio-home/crypto"
	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/dbutil"
	"github.com/SteVio89/stevio-home/handlers/common"
	"github.com/SteVio89/stevio-home/i18n"
	"github.com/SteVio89/stevio-home/payment"
	"github.com/SteVio89/stevio-home/payment/mock"
)

// CreateCheckout handles POST /api/checkout/session.
func (h *CheckoutHandler) CreateCheckout(c *app.Ctx) error {
	var req struct {
		AppID            string `json:"app_id"`
		DiscountCode     string `json:"discount_code"`
		ConsentTimestamp string `json:"consent_timestamp"`
	}
	if err := c.Decode(&req); err != nil || req.AppID == "" {
		return apierr.ErrBadRequest()
	}
	if len(req.DiscountCode) > 64 {
		return apierr.ErrBadRequest()
	}
	// Validate consent timestamp format if provided.
	if req.ConsentTimestamp != "" {
		if _, err := time.Parse(time.RFC3339Nano, req.ConsentTimestamp); err != nil {
			return apierr.ErrBadRequest()
		}
	}

	ctx := c.R.Context()
	db := c.DB().DB

	// Guard: no active signing key → 503 (user can't activate after purchase).
	activeKey, err := queries.GetActiveSigningKey(ctx, db)
	if err != nil {
		h.log.Printf("checkout: check signing key: %v", err)
		return apierr.ErrInternal()
	}
	if activeKey == nil {
		return apierr.ErrNoActiveSigningKey
	}

	provider, err := h.buildProvider(ctx, c)
	if err != nil {
		if isProviderConfigError(err) {
			return errPaymentNotConfigured
		}
		var ae *apierr.APIError
		if errors.As(err, &ae) {
			return ae
		}
		h.log.Printf("checkout: build provider: %v", err)
		return apierr.ErrInternal()
	}
	commerceApp, err := queries.GetAppByID(ctx, db, req.AppID)
	if err != nil {
		h.log.Printf("checkout: get app %q: %v", req.AppID, err)
		return apierr.ErrInternal()
	}
	if commerceApp == nil || commerceApp.DeletedAt != nil {
		return apierr.ErrNotFound()
	}

	// Display name + slug live on the parent project (translation overlay).
	defaultLocale := c.Locales().Default(ctx)
	appName, projectSlug := h.resolveAppDisplay(ctx, db, commerceApp, defaultLocale)

	if commerceApp.PurchaseMode == "coming_soon" {
		return apierr.ErrPurchaseNotAvailable
	}

	if commerceApp.PriceCents <= 0 {
		return apierr.ErrBadRequest()
	}

	if commerceApp.PurchaseMode == "one_time_only" || commerceApp.PurchaseMode == "install_plus" {
		emailHash := h.resolveCheckoutEmail(c.R, db)
		if emailHash == "" {
			return apierr.ErrLoginRequired
		}
		if commerceApp.PurchaseMode == "one_time_only" {
			status, oerr := queries.GetOwnershipStatus(ctx, db, commerceApp.ID, emailHash)
			if oerr != nil {
				h.log.Printf("checkout: ownership check for %q: %v", commerceApp.ID, oerr)
				return apierr.ErrInternal()
			}
			if status.HasLicense {
				return apierr.ErrAlreadyOwned
			}
		}
	}

	if req.DiscountCode != "" {
		_, derr := queries.ValidateDiscountCode(ctx, db, req.DiscountCode, commerceApp.ID)
		if derr != nil {
			if errors.Is(derr, queries.ErrDiscountNotFound) {
				return apierr.ErrDiscountInvalid
			}
			h.log.Printf("checkout: validate discount %q for app %q: %v", req.DiscountCode, commerceApp.ID, derr)
			return apierr.ErrInternal()
		}
	}
	finalPrice := h.computeDiscountedPrice(ctx, db, commerceApp.PriceCents, commerceApp.ID, req.DiscountCode)

	currencyCode, _ := c.Settings().Get(ctx, "currency_code")
	if currencyCode == "" {
		currencyCode = "EUR"
	}

	if projectSlug == "" {
		projectSlug = commerceApp.ID
	}

	params := payment.CheckoutParams{
		AppID:          commerceApp.ID,
		AppName:        appName,
		PriceCents:     finalPrice,
		CurrencyCode:   currencyCode,
		TaxCategory:    commerceApp.TaxCategory,
		SuccessURL:     h.cfg.BaseURL + "/success?session_id={SESSION_ID}",
		CancelURL:      h.cfg.BaseURL + "/project/" + projectSlug,
		DiscountCode:   req.DiscountCode,
		ConsentGivenAt: req.ConsentTimestamp,
	}

	session, err := provider.CreateCheckout(ctx, params)
	if err != nil {
		h.log.Printf("checkout: create session for app %q: %v", commerceApp.ID, err)
		return apierr.ErrInternal()
	}

	return c.JSON(http.StatusOK, map[string]string{
		"url": session.URL,
	})
}

// processWebhookResult is the outcome of processing a signed webhook body.
// HTTP handlers translate this to status codes; the mock trigger endpoint
// uses it directly to decide what to redirect to.
type processWebhookResult struct {
	// Status is the HTTP status the outer handler should return: 200 (handled
	// or ignored), 400 (malformed body / bad signature), or 500 (DB error).
	Status int
	// Event, when non-nil, is the parsed event we acted on. Useful for logging.
	Event *payment.Event
}

// processWebhook runs the full webhook pipeline on a raw signed body:
// verify signature via the active provider, parse the event, dispatch to
// refund or order-fulfillment. Factored out so both the real HTTP handler
// and the in-process mock trigger call exactly the same code.
func (h *CheckoutHandler) processWebhook(ctx context.Context, c *app.Ctx, body []byte, headers http.Header) processWebhookResult {
	db := c.DB().DB

	provider, err := h.buildProvider(ctx, c)
	if err != nil {
		if !isProviderConfigError(err) {
			h.log.Printf("webhook: build provider: %v", err)
		}
		// No provider means no verification — we accept (200) so Paddle doesn't
		// retry a deliberately-disabled endpoint, but do nothing.
		return processWebhookResult{Status: http.StatusOK}
	}

	event, err := provider.ParseWebhook(body, headers)
	if err != nil {
		h.log.Printf("webhook: parse: %v", err)
		return processWebhookResult{Status: http.StatusBadRequest}
	}
	if event == nil {
		return processWebhookResult{Status: http.StatusOK}
	}

	if event.Type == "refund" {
		if err := h.handleRefund(ctx, db, event.SessionID); err != nil {
			h.log.Printf("webhook: refund for session %s: %v", event.SessionID, err)
			return processWebhookResult{Status: http.StatusInternalServerError, Event: event}
		}
		h.log.Printf("webhook: license revoked for refunded session %s", event.SessionID)
		return processWebhookResult{Status: http.StatusOK, Event: event}
	}

	existingOrder, err := queries.GetOrderByPaymentSession(ctx, db, event.SessionID)
	if err != nil {
		h.log.Printf("webhook: check existing order: %v", err)
		return processWebhookResult{Status: http.StatusInternalServerError, Event: event}
	}
	if existingOrder != nil && !existingOrder.Refunded {
		// Already fulfilled — idempotent return.
		return processWebhookResult{Status: http.StatusOK, Event: event}
	}
	// existingOrder == nil: normal path (no prior record)
	// existingOrder != nil && existingOrder.Refunded: stub exists, fulfill then revoke

	emailHash := crypto.HashEmail(event.Email, h.cfg.EmailHashSalt)

	defaultLocale := c.Locales().Default(ctx)
	maxActivations := c.Settings().GetInt(ctx, "max_activations", 3)
	if err := h.fulfillOrder(ctx, db, event, emailHash, maxActivations, defaultLocale, existingOrder); err != nil {
		if dbutil.IsUniqueViolation(err) {
			h.log.Printf("webhook: duplicate delivery for session %s (UNIQUE constraint)", event.SessionID)
			return processWebhookResult{Status: http.StatusOK, Event: event}
		}
		h.log.Printf("webhook: fulfill order: %v", err)
		return processWebhookResult{Status: http.StatusInternalServerError, Event: event}
	}

	if event.DiscountCode != "" {
		go func() {
			if err := queries.IncrementDiscountUses(context.Background(), db, event.DiscountCode, event.AppID); err != nil {
				h.log.Printf("webhook: increment discount uses for %q: %v (non-fatal)", event.DiscountCode, err)
			}
		}()
	}

	h.log.Printf("webhook: order fulfilled for session %s, app %s", event.SessionID, event.AppID)
	return processWebhookResult{Status: http.StatusOK, Event: event}
}

// WebhookReceive handles POST /api/payment/webhook. Thin wrapper around
// processWebhook that reads the body and writes the resulting status.
func (h *CheckoutHandler) WebhookReceive(c *app.Ctx) error {
	ctx := c.R.Context()

	body, err := io.ReadAll(io.LimitReader(c.R.Body, 2<<20))
	if err != nil {
		h.log.Printf("webhook: read body: %v", err)
		c.W.WriteHeader(http.StatusBadRequest)
		return nil
	}

	result := h.processWebhook(ctx, c, body, c.R.Header)
	c.W.WriteHeader(result.Status)
	return nil
}

func (h *CheckoutHandler) fulfillOrder(ctx context.Context, db *sql.DB, event *payment.Event, emailHash string, maxActivations int, defaultLocale string, existingOrder *models.Order) error {
	// All read-only lookups BEFORE the transaction to avoid deadlock with MaxOpenConns=1.
	var snapshot queries.OrderDiscountSnapshot
	originalPrice, perr := queries.GetAppPriceCents(ctx, db, event.AppID)
	if perr == nil && originalPrice > 0 && originalPrice != event.PriceCents {
		snapshot.OriginalPriceCents = &originalPrice
	}

	var autoDiscountID *string
	autoDiscount, aerr := queries.GetActiveAutoDiscount(ctx, db, event.AppID)
	if aerr == nil {
		autoDiscountID = &autoDiscount.ID
	}

	commerceApp, _ := queries.GetAppByID(ctx, db, event.AppID)
	purchaseMode := "always_new_license"
	if commerceApp != nil {
		purchaseMode = commerceApp.PurchaseMode
	}

	// Build discount snapshot BEFORE the transaction — the fulfillment tx must
	// contain no discount writes (no TOCTOU, no transactional re-validation).
	// Per D-05: always record a snapshot reflecting what the customer paid.
	// If the code expired or was deleted since payment, discountCodeID stays nil
	// but we still record whatever we can (auto-discount snapshot is independent).
	var discountCodeID *string
	if event.DiscountCode != "" {
		discount, derr := queries.ValidateDiscountCode(ctx, db, event.DiscountCode, event.AppID)
		if derr == nil {
			discountCodeID = &discount.ID
			snapshot.DiscountLabel = &discount.Label
			snapshot.DiscountType = &discount.DiscountType
			snapshot.DiscountValue = &discount.DiscountValue
		} else {
			h.log.Printf("webhook: discount %q no longer valid: %v (snapshot omitted)", event.DiscountCode, derr)
		}
	}

	if autoDiscountID != nil {
		if discountCodeID != nil {
			combined := autoDiscount.Label + " + " + *snapshot.DiscountLabel
			snapshot.DiscountLabel = &combined
		} else if snapshot.OriginalPriceCents != nil {
			snapshot.DiscountLabel = &autoDiscount.Label
			snapshot.DiscountType = &autoDiscount.DiscountType
			snapshot.DiscountValue = &autoDiscount.DiscountValue
		}
	}

	return queries.WithTx(ctx, db, func(tx *sql.Tx) error {
		var order *models.Order
		var err error

		if existingOrder != nil && existingOrder.Refunded {
			// Stub exists — UPDATE with real data (per D-08).
			order, err = queries.FulfillStubOrder(ctx, tx, event.SessionID, emailHash, event.AppID, event.PriceCents, discountCodeID, autoDiscountID, snapshot, event.ConsentGivenAt)
		} else {
			// Normal path — INSERT new order.
			order, err = queries.InsertOrder(ctx, tx, event.SessionID, emailHash, event.AppID, event.PriceCents, discountCodeID, autoDiscountID, snapshot, event.ConsentGivenAt)
		}
		if err != nil {
			return err
		}

		if err := h.fulfillLicense(ctx, tx, purchaseMode, order.ID, event.AppID, emailHash, maxActivations, defaultLocale); err != nil {
			return err
		}

		// Per D-09: if this was a pre-refunded order, immediately revoke the license.
		if existingOrder != nil && existingOrder.Refunded {
			if _, err := tx.ExecContext(ctx, `UPDATE licenses SET revoked = TRUE WHERE order_id = $1`, order.ID); err != nil {
				return err
			}
		}

		return nil
	})
}

func (h *CheckoutHandler) fulfillLicense(ctx context.Context, tx *sql.Tx, purchaseMode, orderID, appID, emailHash string, maxActivations int, defaultLocale string) error {
	if purchaseMode == "install_plus" {
		return h.fulfillInstallPlus(ctx, tx, orderID, appID, emailHash, maxActivations, defaultLocale)
	}
	licenseKey := queries.NewLicenseKey()
	_, err := queries.InsertLicense(ctx, tx, licenseKey, orderID, appID, nil)
	return err
}

func (h *CheckoutHandler) fulfillInstallPlus(ctx context.Context, tx *sql.Tx, orderID, appID, emailHash string, slotsToAdd int, defaultLocale string) error {
	existing, err := queries.GetLicenseForAppAndEmail(ctx, tx, appID, emailHash, defaultLocale)
	if err != nil {
		return err
	}
	if existing == nil {
		licenseKey := queries.NewLicenseKey()
		_, err = queries.InsertLicense(ctx, tx, licenseKey, orderID, appID, &slotsToAdd)
		return err
	}
	return queries.BumpLicenseMaxActivations(ctx, tx, existing.ID, slotsToAdd)
}

func (h *CheckoutHandler) handleRefund(ctx context.Context, db *sql.DB, paymentSession string) error {
	order, err := queries.GetOrderByPaymentSession(ctx, db, paymentSession)
	if err != nil {
		return err
	}
	if order == nil {
		// Refund arrived before order — insert poison pill stub (per D-07).
		return queries.InsertRefundStub(ctx, db, paymentSession)
	}
	if order.Refunded {
		return nil // already handled (duplicate refund webhook)
	}
	return queries.RevokeLicenseByOrderID(ctx, db, order.ID)
}

// VerifyCheckout handles GET /api/checkout/verify?session_id=X.
func (h *CheckoutHandler) VerifyCheckout(c *app.Ctx) error {
	sessionID := c.R.URL.Query().Get("session_id")
	if sessionID == "" || len(sessionID) > 128 {
		return apierr.ErrBadRequest()
	}

	defaultLocale := c.Locales().Default(c.R.Context())
	result, err := queries.GetOrderAndLicenseByPaymentSession(c.R.Context(), c.DB().DB, sessionID, defaultLocale)
	if err != nil {
		h.log.Printf("checkout-verify: %v", err)
		return apierr.ErrNotFound()
	}
	if result == nil {
		return apierr.ErrNotFound()
	}

	return c.JSON(http.StatusOK, map[string]string{
		"license_key": result.LicenseKey,
		"app_name":    result.AppName,
		"bundle_id":   result.BundleID,
	})
}

// MockComplete handles GET /api/checkout/mock/trigger?action=pay|refund|cancel.
// It constructs a signed mock webhook envelope and invokes the same
// processWebhook pipeline real Paddle webhooks hit, so the mock exercises
// signature verification, ParseWebhook, and fulfillOrder end-to-end.
//
// The mock webhook secret is derived deterministically from SIGNING_KEY_SECRET
// (same derivation as mock.New), so this endpoint can sign payloads the active
// mock provider will accept. Guarded by payment_provider="mock" so this path
// is a 404 in any environment where Mock isn't the active provider.
func (h *CheckoutHandler) MockComplete(c *app.Ctx) error {
	ctx := c.R.Context()
	activeProvider, _ := c.Settings().Get(ctx, "payment_provider")
	if activeProvider != "mock" {
		return apierr.ErrNotFound()
	}

	q := c.R.URL.Query()
	action := q.Get("action")
	if action == "" {
		action = "pay"
	}
	sessionID := q.Get("session_id")
	if sessionID == "" || len(sessionID) > 128 {
		return apierr.ErrBadRequest()
	}

	switch action {
	case "cancel":
		// No webhook — just redirect. The cancel_url on CheckoutParams would
		// normally land on the project page; we mirror that by redirecting to
		// root if no explicit cancel target was supplied.
		target := q.Get("cancel_url")
		if target == "" {
			target = "/"
		}
		http.Redirect(c.W, c.R, target, http.StatusFound)
		return nil

	case "pay":
		return h.mockEmitAndDispatch(c, "pay", sessionID, q)

	case "refund":
		return h.mockEmitAndDispatch(c, "refund", sessionID, q)
	}

	return apierr.ErrBadRequest()
}

// mockEmitAndDispatch builds a signed Envelope matching the given action,
// calls processWebhook in-process, and produces the appropriate response.
// Exported route behavior:
//   - pay   → redirect to /success?session_id=...
//   - refund → 204 No Content (called from Success page's dev-only button)
func (h *CheckoutHandler) mockEmitAndDispatch(c *app.Ctx, action, sessionID string, q url.Values) error {
	ctx := c.R.Context()

	env := mock.Envelope{SessionID: sessionID}
	switch action {
	case "pay":
		env.EventType = "order"
		env.AppID = q.Get("app_id")
		if env.AppID == "" || len(env.AppID) > 128 {
			return apierr.ErrBadRequest()
		}
		env.Email = q.Get("email")
		if env.Email == "" {
			env.Email = "mock@example.com"
		}
		if len(env.Email) > 255 {
			return apierr.ErrBadRequest()
		}
		if p, err := strconv.Atoi(q.Get("price_cents")); err == nil {
			env.PriceCents = p
		} else {
			// Fall back to the app's configured price if not supplied. Lets the
			// /mock-checkout page punt on discount math.
			app, aerr := queries.GetAppByID(ctx, c.DB().DB, env.AppID)
			if aerr != nil || app == nil {
				return apierr.ErrBadRequest()
			}
			env.PriceCents = app.PriceCents
		}
		env.CurrencyCode = q.Get("currency_code")
		env.DiscountCode = q.Get("discount_code")
		env.ConsentGivenAt = q.Get("consent_given_at")
		if len(env.DiscountCode) > 64 || len(env.ConsentGivenAt) > 64 {
			return apierr.ErrBadRequest()
		}
	case "refund":
		env.EventType = "refund"
	}

	body, err := json.Marshal(env)
	if err != nil {
		h.log.Printf("mock-complete: marshal envelope: %v", err)
		return apierr.ErrInternal()
	}

	headers := http.Header{}
	headers.Set("Content-Type", "application/json")
	headers.Set(mock.SignatureHeader, mock.Sign(mock.DeriveSecret(h.signingKeySecret), body))

	result := h.processWebhook(ctx, c, body, headers)
	if result.Status >= 500 {
		return apierr.ErrInternal()
	}
	if result.Status >= 400 {
		return apierr.ErrBadRequest()
	}

	if action == "refund" {
		return c.NoContent()
	}

	http.Redirect(c.W, c.R, h.cfg.BaseURL+"/success?session_id="+sessionID, http.StatusFound)
	return nil
}

func (h *CheckoutHandler) computeDiscountedPrice(ctx context.Context, db *sql.DB, originalPrice int, appID, discountCode string) int {
	finalPrice := originalPrice
	if discountCode != "" {
		discount, derr := queries.ValidateDiscountCode(ctx, db, discountCode, appID)
		if derr == nil {
			baseForCode := originalPrice
			if discount.Stackable {
				autoDiscount, aerr := queries.GetActiveAutoDiscount(ctx, db, appID)
				if aerr == nil {
					baseForCode = queries.ApplyDiscount(autoDiscount.DiscountType, autoDiscount.DiscountValue, originalPrice)
				}
			}
			finalPrice = queries.ApplyDiscount(discount.DiscountType, discount.DiscountValue, baseForCode)
		}
	} else {
		autoDiscount, aerr := queries.GetActiveAutoDiscount(ctx, db, appID)
		if aerr == nil {
			finalPrice = queries.ApplyDiscount(autoDiscount.DiscountType, autoDiscount.DiscountValue, originalPrice)
		}
	}
	return finalPrice
}

func (h *CheckoutHandler) resolveCheckoutEmail(r *http.Request, db *sql.DB) string {
	cookie, err := r.Cookie("sid")
	if err != nil {
		return ""
	}
	sessionID, ok := crypto.VerifySession(cookie.Value, h.cfg.SessionSecretBytes)
	if !ok {
		return ""
	}
	session, serr := auth.GetSession(r.Context(), db, sessionID)
	if serr != nil || session == nil {
		return ""
	}
	return session.EmailHash
}

// resolveAppDisplay returns the project-derived display name and slug for a
// commerce app. Display text moved to projects in the schema refactor — this
// helper hops apps → projects and overlays the requested locale on the project's
// translated title.
func (h *CheckoutHandler) resolveAppDisplay(ctx context.Context, db *sql.DB, a *models.App, locale string) (name, slug string) {
	if a == nil || a.ProjectID == "" {
		return "", ""
	}
	project, err := queries.GetProjectByID(ctx, db, a.ProjectID)
	if err != nil || project == nil {
		return "", ""
	}
	slug = project.Slug
	if fields, err := i18n.GetEntityTranslation(ctx, db, common.EntityTypeProject, project.ID, locale); err == nil {
		i18n.NewOverlay(fields).Apply("title", &name)
	}
	return name, slug
}
