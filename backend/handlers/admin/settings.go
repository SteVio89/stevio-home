package admin

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/crypto"
)

// validSettingKeys is the allowlist of setting keys that can be read or written
// by the admin. Prevents creation of arbitrary key-value pairs.
var validSettingKeys = map[string]bool{
	"currency_symbol":            true,
	"currency_code":              true,
	"site_name":                  true,
	"max_activations":            true,
	"download_token_ttl_min":     true,
	"magic_link_ttl_min":         true,
	"payment_provider":           true,
	"paddle_api_key":             true,
	"paddle_webhook_secret":      true,
	"paddle_client_token":        true,
	"paddle_environment":         true,
	"polar_api_key":              true,
	"polar_webhook_secret":       true,
	"polar_environment":          true,
	"maintenance_mode":           true,
	"support_notification_email": true,
	"chat_rate_limit":            true,
	"chat_max_message_length":    true,
}

// secretSettingKeys are stored as AES-256-GCM ciphertext (via crypto.EncryptSetting,
// keyed by SIGNING_KEY_SECRET). They are masked in GET responses and only written
// on PATCH when the incoming value is non-empty and not the mask sentinel.
var secretSettingKeys = map[string]bool{
	"paddle_api_key":        true,
	"paddle_webhook_secret": true,
	"polar_api_key":         true,
	"polar_webhook_secret":  true,
}

// secretSetSentinel is what AdminGetSettings returns in place of the ciphertext
// when a secret is configured. The UI echoes it back on save to mean "no change".
const secretSetSentinel = "********"

func (h *AdminHandler) AdminGetSettings(c *app.Ctx) error {
	s, err := c.Settings().GetAll(c.R.Context())
	if err != nil {
		h.log.Printf("admin-settings: get: %v", err)
		return apierr.ErrInternal()
	}
	filtered := make(map[string]string, len(validSettingKeys))
	for k, v := range s {
		if !validSettingKeys[k] {
			continue
		}
		if secretSettingKeys[k] {
			if v == "" {
				filtered[k] = ""
			} else {
				filtered[k] = secretSetSentinel
			}
			continue
		}
		filtered[k] = v
	}
	return c.JSON(http.StatusOK, filtered)
}

func (h *AdminHandler) AdminUpdateSetting(c *app.Ctx) error {
	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := c.Decode(&req); err != nil {
		return apierr.ErrBadRequest()
	}
	if !h.isValidSettingKey(req.Key) {
		return apierr.ErrBadRequest()
	}
	if err := h.validateSettingValue(req.Key, req.Value); err != nil {
		return apierr.ErrBadRequest()
	}

	if secretSettingKeys[req.Key] {
		// Empty or sentinel both mean "leave as-is". Prevents an admin form
		// from overwriting a configured secret with the masked placeholder.
		if req.Value == "" || req.Value == secretSetSentinel {
			return c.NoContent()
		}
		ct, err := crypto.EncryptSetting(h.signingKeySecret, req.Value)
		if err != nil {
			h.log.Printf("admin-settings: encrypt %q: %v", req.Key, err)
			return apierr.ErrInternal()
		}
		if err := c.Settings().Upsert(c.R.Context(), req.Key, ct); err != nil {
			h.log.Printf("admin-settings: upsert %q: %v", req.Key, err)
			return apierr.ErrInternal()
		}
		return c.NoContent()
	}

	if err := c.Settings().Upsert(c.R.Context(), req.Key, req.Value); err != nil {
		h.log.Printf("admin-settings: upsert %q: %v", req.Key, err)
		return apierr.ErrInternal()
	}
	return c.NoContent()
}

func (h *AdminHandler) isValidSettingKey(key string) bool {
	return validSettingKeys[key]
}

func validateIntRange(value string, min, max int) error {
	n, err := strconv.Atoi(value)
	if err != nil || n < min || n > max {
		return fmt.Errorf("must be between %d and %d", min, max)
	}
	return nil
}

func (h *AdminHandler) validateSettingValue(key, value string) error {
	// PATCH values are plaintext on ingress (secrets are encrypted server-side
	// before Upsert). 4096 accommodates even a long API key while still being
	// strict enough to reject obvious abuse.
	if len(value) > 4096 {
		return fmt.Errorf("value too long")
	}
	switch key {
	case "max_activations":
		return validateIntRange(value, 1, 100)
	case "download_token_ttl_min", "magic_link_ttl_min":
		return validateIntRange(value, 1, 1440)
	case "currency_symbol", "currency_code":
		if value == "" || len(value) > 10 {
			return fmt.Errorf("%s must be 1-10 characters", key)
		}
	case "site_name":
		if value == "" || len(value) > 100 {
			return fmt.Errorf("site_name must be 1-100 characters")
		}
	case "payment_provider":
		if value != "" {
			if _, ok := h.payments[value]; !ok {
				return fmt.Errorf("unsupported payment provider")
			}
		}
	case "paddle_api_key", "paddle_webhook_secret", "polar_api_key", "polar_webhook_secret":
		// Plaintext length cap applied above; nothing provider-specific to check.
	case "paddle_environment", "polar_environment":
		if value != "" && value != "sandbox" && value != "production" {
			return fmt.Errorf("%s must be 'sandbox' or 'production'", key)
		}
	case "maintenance_mode":
		if value != "0" && value != "1" {
			return fmt.Errorf("maintenance_mode must be 0 or 1")
		}
	case "chat_rate_limit":
		return validateIntRange(value, 1, 600)
	case "chat_max_message_length":
		return validateIntRange(value, 100, 10000)
	case "support_notification_email":
		if len(value) > 255 {
			return fmt.Errorf("email too long")
		}
	}
	return nil
}
