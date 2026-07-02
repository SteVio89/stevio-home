package admin

import (
	"encoding/base64"
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/crypto"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/dbutil"
)

// AdminGetSigningKey returns the currently active signing key, or null if none.
func (h *AdminHandler) AdminGetSigningKey(c *app.Ctx) error {
	key, err := queries.GetActiveSigningKey(c.R.Context(), c.DB().DB)
	if err != nil {
		h.log.Printf("admin: get active signing key: %v", err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusOK, map[string]any{"key": key})
}

// AdminGenerateSigningKey generates a new Ed25519 keypair, encrypts the private key,
// stores it in the DB, activates it, and deletes any previous inactive keys.
func (h *AdminHandler) AdminGenerateSigningKey(c *app.Ctx) error {
	privB64, pubB64, err := crypto.GenerateKey()
	if err != nil {
		h.log.Printf("admin: generate key: %v", err)
		return apierr.ErrInternal()
	}

	privBytes, err := base64.StdEncoding.DecodeString(privB64)
	if err != nil {
		h.log.Printf("admin: decode private key: %v", err)
		return apierr.ErrInternal()
	}
	pubBytes, err := base64.StdEncoding.DecodeString(pubB64)
	if err != nil {
		h.log.Printf("admin: decode public key: %v", err)
		return apierr.ErrInternal()
	}
	keyID := crypto.DeriveKeyID(pubBytes)

	encrypted, err := crypto.EncryptPrivateKey(h.signingKeySecret, privBytes)
	if err != nil {
		h.log.Printf("admin: encrypt key: %v", err)
		return apierr.ErrInternal()
	}

	ctx := c.R.Context()
	db := c.DB().DB
	id := dbutil.NewID()

	key, err := queries.InsertSigningKey(ctx, db, id, keyID, encrypted, pubB64)
	if err != nil {
		h.log.Printf("admin: insert signing key: %v", err)
		return apierr.ErrInternal()
	}

	if err := queries.ActivateSigningKey(ctx, db, key.ID); err != nil {
		h.log.Printf("admin: activate signing key: %v", err)
		return apierr.ErrInternal()
	}
	key.Active = true

	// Clean up old inactive keys — with pinned public keys on the client,
	// old signing keys serve no purpose.
	if err := queries.DeleteInactiveSigningKeys(ctx, db); err != nil {
		h.log.Printf("admin: delete inactive keys: %v", err)
		// Non-fatal — the new key is already active.
	}

	h.log.Printf("admin: generated and activated signing key %s (key_id=%s)", key.ID, key.KeyID)
	return c.JSON(http.StatusCreated, key)
}

// AdminExportPublicKey returns the active public key for embedding in Mac apps.
func (h *AdminHandler) AdminExportPublicKey(c *app.Ctx) error {
	key, err := queries.GetActiveSigningKey(c.R.Context(), c.DB().DB)
	if err != nil {
		h.log.Printf("admin: get active signing key: %v", err)
		return apierr.ErrInternal()
	}
	if key == nil {
		return apierr.ErrNotFound()
	}

	return c.JSON(http.StatusOK, map[string]string{
		"key_id":         key.KeyID,
		"public_key_b64": key.PublicKeyB64,
	})
}
