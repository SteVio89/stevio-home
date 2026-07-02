package discounts

import (
	"errors"
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/queries"
)

func (h *DiscountsHandler) GetAutoDiscount(c *app.Ctx) error {
	appID := c.R.URL.Query().Get("app_id")
	if appID == "" {
		return apierr.ErrBadRequest()
	}

	originalCents, err := queries.GetAppPriceCents(c.R.Context(), c.DB().DB, appID)
	if err != nil {
		if errors.Is(err, queries.ErrDiscountNotFound) {
			return apierr.ErrNotFound()
		}
		h.log.Printf("discounts: get app price for auto-discount (app %q): %v", appID, err)
		return apierr.ErrInternal()
	}

	discount, err := queries.GetActiveAutoDiscount(c.R.Context(), c.DB().DB, appID)
	if err != nil {
		if errors.Is(err, queries.ErrAutoDiscountNotFound) {
			return apierr.ErrNotFound()
		}
		h.log.Printf("discounts: get active auto-discount for app %q: %v", appID, err)
		return apierr.ErrInternal()
	}

	finalCents := queries.ApplyDiscount(discount.DiscountType, discount.DiscountValue, originalCents)

	return c.JSON(http.StatusOK, map[string]any{
		"discount_type":        discount.DiscountType,
		"discount_value":       discount.DiscountValue,
		"original_price_cents": originalCents,
		"final_price_cents":    finalCents,
	})
}

func (h *DiscountsHandler) ValidateDiscount(c *app.Ctx) error {
	var req struct {
		Code  string `json:"code"`
		AppID string `json:"app_id"`
	}
	if err := c.Decode(&req); err != nil || req.Code == "" || req.AppID == "" {
		return apierr.ErrBadRequest()
	}

	originalCents, err := queries.GetAppPriceCents(c.R.Context(), c.DB().DB, req.AppID)
	if err != nil {
		if errors.Is(err, queries.ErrDiscountNotFound) {
			return apierr.ErrDiscountInvalid
		}
		h.log.Printf("discounts: get app price for %q: %v", req.AppID, err)
		return apierr.ErrInternal()
	}

	discount, err := queries.ValidateDiscountCode(c.R.Context(), c.DB().DB, req.Code, req.AppID)
	if err != nil {
		if errors.Is(err, queries.ErrDiscountNotFound) {
			return apierr.ErrDiscountInvalid
		}
		h.log.Printf("discounts: validate %q for app %q: %v", req.Code, req.AppID, err)
		return apierr.ErrInternal()
	}

	stackedWithAuto := false
	baseForCode := originalCents
	if discount.Stackable {
		autoDiscount, aerr := queries.GetActiveAutoDiscount(c.R.Context(), c.DB().DB, req.AppID)
		if aerr == nil {
			baseForCode = queries.ApplyDiscount(autoDiscount.DiscountType, autoDiscount.DiscountValue, originalCents)
			stackedWithAuto = true
		}
	}

	finalCents := queries.ApplyDiscount(discount.DiscountType, discount.DiscountValue, baseForCode)

	return c.JSON(http.StatusOK, map[string]any{
		"discount_type":        discount.DiscountType,
		"discount_value":       discount.DiscountValue,
		"original_price_cents": originalCents,
		"final_price_cents":    finalCents,
		"stacked_with_auto":    stackedWithAuto,
	})
}
