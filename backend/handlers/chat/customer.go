package chat

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/crypto"
	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/db/queries"
)

type chatResponse struct {
	Conversation conversationView     `json:"conversation"`
	Messages     []models.ChatMessage `json:"messages"`
	HasUnread    bool                 `json:"has_unread"`
}

type conversationView struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	EmailShared bool   `json:"email_shared"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func (h *ChatHandler) GetChat(c *app.Ctx) error {
	ctx := c.R.Context()
	userID := c.User().ID

	conv, err := queries.GetConversationByUserID(ctx, c.DB().DB, userID)
	if err != nil {
		h.log.Printf("chat: get conversation: %v", err)
		return apierr.ErrInternal()
	}
	if conv == nil {
		return apierr.ErrNotFound()
	}

	msgs, err := queries.GetMessagesByConversationID(ctx, c.DB().DB, conv.ID)
	if err != nil {
		h.log.Printf("chat: get messages: %v", err)
		return apierr.ErrInternal()
	}
	if msgs == nil {
		msgs = []models.ChatMessage{}
	}

	// Mark admin messages as read
	if err := queries.MarkMessagesRead(ctx, c.DB().DB, conv.ID, "admin"); err != nil {
		h.log.Printf("chat: mark read: %v", err)
	}

	hasUnread, err := queries.HasUnreadMessages(ctx, c.DB().DB, conv.ID, "admin")
	if err != nil {
		h.log.Printf("chat: has unread: %v", err)
	}

	return c.JSON(http.StatusOK, chatResponse{
		Conversation: conversationView{
			ID:          conv.ID,
			DisplayName: conv.DisplayName,
			EmailShared: conv.Email != nil,
			CreatedAt:   conv.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:   conv.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		},
		Messages:  msgs,
		HasUnread: hasUnread,
	})
}

func (h *ChatHandler) CreateChat(c *app.Ctx) error {
	ctx := c.R.Context()
	userID := c.User().ID

	banned, err := queries.IsUserBanned(ctx, c.DB().DB, userID)
	if err != nil {
		h.log.Printf("chat: check ban: %v", err)
		return apierr.ErrInternal()
	}
	if banned {
		return apierr.ErrForbidden()
	}

	exists, err := queries.ConversationExistsForUser(ctx, c.DB().DB, userID)
	if err != nil {
		h.log.Printf("chat: check existing: %v", err)
		return apierr.ErrInternal()
	}
	if exists {
		return apierr.ErrConflict()
	}

	displayName, err := h.generateDisplayName(ctx, c.DB().DB)
	if err != nil {
		h.log.Printf("chat: generate name: %v", err)
		return apierr.ErrInternal()
	}

	conv, err := queries.CreateConversation(ctx, c.DB().DB, userID, displayName)
	if err != nil {
		h.log.Printf("chat: create: %v", err)
		return apierr.ErrInternal()
	}

	return c.JSON(http.StatusCreated, conversationView{
		ID:          conv.ID,
		DisplayName: conv.DisplayName,
		EmailShared: false,
		CreatedAt:   conv.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   conv.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

func (h *ChatHandler) SendMessage(c *app.Ctx) error {
	ctx := c.R.Context()
	userID := c.User().ID

	banned, err := queries.IsUserBanned(ctx, c.DB().DB, userID)
	if err != nil {
		h.log.Printf("chat: check ban: %v", err)
		return apierr.ErrInternal()
	}
	if banned {
		return apierr.ErrForbidden()
	}

	var req struct {
		Body string `json:"body"`
	}
	if err := c.Decode(&req); err != nil {
		return apierr.ErrBadRequest()
	}
	if req.Body == "" {
		return apierr.ErrBadRequest()
	}

	maxLen := c.Settings().GetInt(ctx, "chat_max_message_length", 2000)
	if len(req.Body) > maxLen {
		return apierr.ErrBadRequest()
	}

	conv, err := queries.GetConversationByUserID(ctx, c.DB().DB, userID)
	if err != nil {
		h.log.Printf("chat: get conversation: %v", err)
		return apierr.ErrInternal()
	}
	if conv == nil {
		return apierr.ErrNotFound()
	}

	body := sanitizeMessageBody(req.Body)

	msg, err := queries.CreateMessage(ctx, c.DB().DB, conv.ID, "user", body)
	if err != nil {
		h.log.Printf("chat: create message: %v", err)
		return apierr.ErrInternal()
	}

	if err := queries.UpdateConversationUpdatedAt(ctx, c.DB().DB, conv.ID); err != nil {
		h.log.Printf("chat: update timestamp: %v", err)
	}

	// Send notification email (once per conversation)
	if conv.NotifiedAt == nil {
		h.sendNotification(c, conv, body)
	}

	return c.JSON(http.StatusCreated, msg)
}

func (h *ChatHandler) sendNotification(c *app.Ctx, conv *models.ChatConversation, body string) {
	if h.mailer == nil {
		return
	}
	ctx := c.R.Context()
	notifEmail, _ := c.Settings().Get(ctx, "support_notification_email")
	if notifEmail == "" {
		return
	}

	siteName, _ := c.Settings().Get(ctx, "site_name")
	if siteName == "" {
		siteName = "Store"
	}

	subject := fmt.Sprintf("[%s] New support message from %s", siteName, conv.DisplayName)
	if err := h.mailer.Send(notifEmail, subject, body); err != nil {
		h.log.Printf("chat: send notification: %v", err)
		return
	}

	// Use background context for the flag update to ensure it persists
	// even if the HTTP request context is cancelled.
	if err := queries.SetConversationNotified(context.Background(), c.DB().DB, conv.ID); err != nil {
		h.log.Printf("chat: set notified: %v", err)
	}
}

func (h *ChatHandler) ShareEmail(c *app.Ctx) error {
	ctx := c.R.Context()
	userID := c.User().ID

	var req struct {
		Email string `json:"email"`
	}
	if err := c.Decode(&req); err != nil || req.Email == "" {
		return apierr.ErrBadRequest()
	}

	conv, err := queries.GetConversationByUserID(ctx, c.DB().DB, userID)
	if err != nil {
		h.log.Printf("chat: get conversation: %v", err)
		return apierr.ErrInternal()
	}
	if conv == nil {
		return apierr.ErrNotFound()
	}
	if conv.Email != nil {
		return apierr.ErrConflict()
	}

	// Validate: hash the email and compare to the user's stored hash
	hashedEmail := crypto.HashEmail(req.Email, h.cfg.EmailHashSalt)
	if hashedEmail != c.User().EmailHash {
		return apierr.ErrBadRequest()
	}

	if err := queries.UpdateConversationEmail(ctx, c.DB().DB, conv.ID, req.Email, req.Email); err != nil {
		h.log.Printf("chat: update email: %v", err)
		return apierr.ErrInternal()
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "shared"})
}

func (h *ChatHandler) DeleteChat(c *app.Ctx) error {
	ctx := c.R.Context()
	userID := c.User().ID

	conv, err := queries.GetConversationByUserID(ctx, c.DB().DB, userID)
	if err != nil {
		h.log.Printf("chat: get conversation: %v", err)
		return apierr.ErrInternal()
	}
	if conv == nil {
		return apierr.ErrNotFound()
	}

	if err := queries.WithTx(ctx, c.DB().DB, func(tx *sql.Tx) error {
		return queries.DeleteConversation(ctx, tx, conv.ID)
	}); err != nil {
		h.log.Printf("chat: delete: %v", err)
		return apierr.ErrInternal()
	}

	return c.NoContent()
}
