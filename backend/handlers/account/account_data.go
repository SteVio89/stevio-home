package account

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/crypto"
	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/db/queries"
)

type userDataExport struct {
	ExportedAt       time.Time                   `json:"exported_at"`
	Sessions         []queries.UserSession       `json:"sessions"`
	Orders           []queries.UserOrder         `json:"orders"`
	Licenses         []models.License            `json:"licenses"`
	Activations      []models.Activation         `json:"activations"`
	DownloadTokens   []queries.UserDownloadToken `json:"download_tokens"`
	ChatConversation *models.ChatConversation    `json:"chat_conversation,omitempty"`
	ChatMessages     []models.ChatMessage        `json:"chat_messages,omitempty"`
}

func (h *AccountHandler) ExportUserData(c *app.Ctx) error {
	emailHash := c.User().EmailHash

	sessions, err := queries.GetUserSessions(c.R.Context(), c.DB().DB, emailHash)
	if err != nil {
		h.log.Printf("export: sessions: %v", err)
		return apierr.ErrInternal()
	}

	defaultLocale := c.Locales().Default(c.R.Context())

	orders, err := queries.GetUserOrders(c.R.Context(), c.DB().DB, emailHash, defaultLocale)
	if err != nil {
		h.log.Printf("export: orders: %v", err)
		return apierr.ErrInternal()
	}

	licenses, err := queries.GetLicensesByEmail(c.R.Context(), c.DB().DB, emailHash, defaultLocale)
	if err != nil {
		h.log.Printf("export: licenses: %v", err)
		return apierr.ErrInternal()
	}
	if licenses == nil {
		licenses = []models.License{}
	}

	activations, err := queries.GetUserActivationsByEmail(c.R.Context(), c.DB().DB, emailHash)
	if err != nil {
		h.log.Printf("export: activations: %v", err)
		return apierr.ErrInternal()
	}

	tokens, err := queries.GetUserDownloadTokensByEmail(c.R.Context(), c.DB().DB, emailHash)
	if err != nil {
		h.log.Printf("export: download tokens: %v", err)
		return apierr.ErrInternal()
	}

	// Support chat is keyed by user ID (not email hash). Include the conversation
	// and messages so the export is complete under GDPR Art. 15/20.
	conversation, err := queries.GetConversationByUserID(c.R.Context(), c.DB().DB, c.User().ID)
	if err != nil {
		h.log.Printf("export: chat conversation: %v", err)
		return apierr.ErrInternal()
	}
	var chatMessages []models.ChatMessage
	if conversation != nil {
		chatMessages, err = queries.GetMessagesByConversationID(c.R.Context(), c.DB().DB, conversation.ID)
		if err != nil {
			h.log.Printf("export: chat messages: %v", err)
			return apierr.ErrInternal()
		}
	}

	export := userDataExport{
		ExportedAt:       time.Now().UTC(),
		Sessions:         sessions,
		Orders:           orders,
		Licenses:         licenses,
		Activations:      activations,
		DownloadTokens:   tokens,
		ChatConversation: conversation,
		ChatMessages:     chatMessages,
	}

	c.W.Header().Set("Content-Type", "application/json")
	c.W.Header().Set("Content-Disposition", `attachment; filename="account-data.json"`)
	if err := json.NewEncoder(c.W).Encode(export); err != nil {
		h.log.Printf("export: encode: %v", err)
	}
	return nil
}

type deleteDataRequest struct {
	Email string `json:"email"`
}

func (h *AccountHandler) DeleteUserData(c *app.Ctx) error {
	emailHash := c.User().EmailHash

	var req deleteDataRequest
	if err := c.Decode(&req); err != nil || req.Email == "" {
		return apierr.ErrBadRequest()
	}

	if crypto.HashEmail(req.Email, h.cfg.EmailHashSalt) != emailHash {
		return apierr.ErrBadRequest()
	}

	if err := queries.EraseUserData(c.R.Context(), c.DB().DB, emailHash, c.User().ID); err != nil {
		h.log.Printf("delete data: erase: %v", err)
		return apierr.ErrInternal()
	}

	http.SetCookie(c.W, &http.Cookie{
		Name:     "sid",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.Env == "production",
		SameSite: http.SameSiteStrictMode,
	})
	return c.NoContent()
}
