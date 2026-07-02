package sdk

import (
	"context"
	"database/sql"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/db/queries"
)

type activateRequest struct {
	LicenseKey  string `json:"license_key"`
	MachineHash string `json:"machine_hash"`
	DeviceLabel string `json:"device_label"`
}

type activationResult struct {
	slotsFull   bool
	activatedAt time.Time
}

func (h *SDKHandler) ActivateLicense(c *app.Ctx) error {
	var req activateRequest
	if err := c.Decode(&req); err != nil || req.LicenseKey == "" || req.MachineHash == "" {
		return apierr.ErrBadRequest()
	}
	if len(req.DeviceLabel) > 255 {
		return apierr.ErrBadRequest()
	}
	if len(req.MachineHash) != 64 {
		return apierr.ErrBadRequest()
	}
	if _, err := hex.DecodeString(req.MachineHash); err != nil {
		return apierr.ErrBadRequest()
	}

	ctx := c.R.Context()

	// Guard: no active signing key → 503.
	activeKeyID, err := h.signer.ActiveKeyID(ctx)
	if err != nil {
		h.log.Printf("activate: check signing key: %v", err)
		return apierr.ErrInternal()
	}
	if activeKeyID == "" {
		return apierr.ErrNoActiveSigningKey
	}

	defaultLocale := c.Locales().Default(ctx)
	license, err := queries.GetLicenseByKey(ctx, c.DB().DB, req.LicenseKey, defaultLocale)
	if err != nil {
		h.log.Printf("activate: get license: %v", err)
		return apierr.ErrInternal()
	}
	if license == nil {
		h.log.Printf("activate: license not found for key prefix %s…", req.LicenseKey[:min(len(req.LicenseKey), 8)])
		return apierr.ErrLicenseInvalid
	}
	if license.Revoked {
		return apierr.ErrLicenseRevoked
	}

	maxActivations := c.Settings().GetInt(ctx, "max_activations", 3)
	if license.MaxActivations != nil {
		maxActivations = *license.MaxActivations
	}

	res, err := h.runActivationTx(ctx, c.DB().DB, license, req, maxActivations, &activeKeyID)
	if err != nil {
		h.log.Printf("activate: upsert: %v", err)
		return apierr.ErrInternal()
	}
	if res.slotsFull {
		return apierr.ErrSlotsFull
	}

	signed, err := h.signer.Sign(ctx, license.AppBundleID, license.Key, req.MachineHash, res.activatedAt)
	if err != nil {
		h.log.Printf("activate: sign: %v", err)
		return apierr.ErrInternal()
	}

	return c.JSON(http.StatusOK, signed)
}

type deactivateRequest struct {
	LicenseKey  string `json:"license_key"`
	MachineHash string `json:"machine_hash"`
}

func (h *SDKHandler) DeactivateLicense(c *app.Ctx) error {
	var req deactivateRequest
	if err := c.Decode(&req); err != nil || req.LicenseKey == "" || req.MachineHash == "" {
		return apierr.ErrBadRequest()
	}
	if len(req.LicenseKey) > 255 {
		return apierr.ErrBadRequest()
	}
	if len(req.MachineHash) != 64 {
		return apierr.ErrBadRequest()
	}
	if _, err := hex.DecodeString(req.MachineHash); err != nil {
		return apierr.ErrBadRequest()
	}

	ctx := c.R.Context()
	defaultLocale := c.Locales().Default(ctx)

	license, err := queries.GetLicenseByKey(ctx, c.DB().DB, req.LicenseKey, defaultLocale)
	if err != nil {
		h.log.Printf("deactivate: get license: %v", err)
		return apierr.ErrInternal()
	}
	if license == nil || license.Revoked {
		return apierr.ErrLicenseInvalid
	}

	deleted, err := queries.DeleteActivationByLicenseAndMachine(ctx, c.DB().DB, license.ID, req.MachineHash)
	if err != nil {
		h.log.Printf("deactivate: delete activation: %v", err)
		return apierr.ErrInternal()
	}
	if !deleted {
		return apierr.ErrLicenseInvalid
	}

	return c.NoContent()
}

func (h *SDKHandler) runActivationTx(ctx context.Context, db *sql.DB, license *models.License, req activateRequest, maxActivations int, keyID *string) (activationResult, error) {
	res := activationResult{activatedAt: license.CreatedAt}

	err := queries.WithTx(ctx, db, func(tx *sql.Tx) error {
		// Lock the license row first so the count-then-insert below is atomic
		// against concurrent activations (otherwise the device cap is bypassable).
		if err := queries.LockLicenseForActivation(ctx, tx, license.ID); err != nil {
			return err
		}
		exists, err := queries.ActivationExistsForMachine(ctx, tx, license.ID, req.MachineHash)
		if err != nil {
			return err
		}
		if !exists {
			count, err := queries.CountActivationsForUpdate(ctx, tx, license.ID)
			if err != nil {
				return err
			}
			if count >= maxActivations {
				res.slotsFull = true
				return nil
			}
		}
		activation, err := queries.UpsertActivation(ctx, tx, license.ID, req.MachineHash, req.DeviceLabel, keyID)
		if err != nil {
			return err
		}
		res.activatedAt = activation.ActivatedAt
		return nil
	})

	return res, err
}
