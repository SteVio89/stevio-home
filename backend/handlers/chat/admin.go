package chat

import (
	"database/sql"
	"net/http"

	"github.com/SteVio89/stevio-home/apierr"
	"github.com/SteVio89/stevio-home/app"
	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/db/queries"
	"github.com/SteVio89/stevio-home/handlers/common"
)

func (h *ChatHandler) AdminListChats(c *app.Ctx) error {
	ctx := c.R.Context()

	page, perPage, err := common.ParsePaginationParams(c.R.URL.Query())
	if err != nil {
		return apierr.ErrBadRequest()
	}

	result, err := queries.ListConversations(ctx, c.DB().DB, queries.ChatListFilter{
		Page:    page,
		PerPage: perPage,
	})
	if err != nil {
		h.log.Printf("chat-admin: list: %v", err)
		return apierr.ErrInternal()
	}

	return c.JSON(http.StatusOK, result)
}

func (h *ChatHandler) AdminGetChat(c *app.Ctx) error {
	ctx := c.R.Context()
	id := c.Param("id")

	conv, err := queries.GetConversationByID(ctx, c.DB().DB, id)
	if err != nil {
		h.log.Printf("chat-admin: get: %v", err)
		return apierr.ErrInternal()
	}
	if conv == nil {
		return apierr.ErrNotFound()
	}

	msgs, err := queries.GetMessagesByConversationID(ctx, c.DB().DB, conv.ID)
	if err != nil {
		h.log.Printf("chat-admin: get messages: %v", err)
		return apierr.ErrInternal()
	}
	if msgs == nil {
		msgs = []models.ChatMessage{}
	}

	// Mark user messages as read
	if err := queries.MarkMessagesRead(ctx, c.DB().DB, conv.ID, "user"); err != nil {
		h.log.Printf("chat-admin: mark read: %v", err)
	}

	hasUnread, err := queries.HasUnreadMessages(ctx, c.DB().DB, conv.ID, "user")
	if err != nil {
		h.log.Printf("chat-admin: has unread: %v", err)
	}

	banned, _ := queries.IsUserBanned(ctx, c.DB().DB, conv.UserID)

	type response struct {
		Conversation *models.ChatConversation `json:"conversation"`
		Messages     []models.ChatMessage     `json:"messages"`
		HasUnread    bool                     `json:"has_unread"`
		IsBanned     bool                     `json:"is_banned"`
	}

	return c.JSON(http.StatusOK, response{
		Conversation: conv,
		Messages:     msgs,
		HasUnread:    hasUnread,
		IsBanned:     banned,
	})
}

func (h *ChatHandler) AdminSendMessage(c *app.Ctx) error {
	ctx := c.R.Context()
	id := c.Param("id")

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

	conv, err := queries.GetConversationByID(ctx, c.DB().DB, id)
	if err != nil {
		h.log.Printf("chat-admin: get conversation: %v", err)
		return apierr.ErrInternal()
	}
	if conv == nil {
		return apierr.ErrNotFound()
	}

	body := sanitizeMessageBody(req.Body)

	msg, err := queries.CreateMessage(ctx, c.DB().DB, conv.ID, "admin", body)
	if err != nil {
		h.log.Printf("chat-admin: create message: %v", err)
		return apierr.ErrInternal()
	}

	if err := queries.UpdateConversationUpdatedAt(ctx, c.DB().DB, conv.ID); err != nil {
		h.log.Printf("chat-admin: update timestamp: %v", err)
	}

	return c.JSON(http.StatusCreated, msg)
}

func (h *ChatHandler) AdminUnreadCount(c *app.Ctx) error {
	count, err := queries.UnreadConversationCount(c.R.Context(), c.DB().DB)
	if err != nil {
		h.log.Printf("chat-admin: unread count: %v", err)
		return apierr.ErrInternal()
	}
	return c.JSON(http.StatusOK, map[string]int{"count": count})
}

func (h *ChatHandler) AdminDeleteChat(c *app.Ctx) error {
	ctx := c.R.Context()
	id := c.Param("id")

	conv, err := queries.GetConversationByID(ctx, c.DB().DB, id)
	if err != nil {
		h.log.Printf("chat-admin: get conversation: %v", err)
		return apierr.ErrInternal()
	}
	if conv == nil {
		return apierr.ErrNotFound()
	}

	if err := queries.WithTx(ctx, c.DB().DB, func(tx *sql.Tx) error {
		return queries.DeleteConversation(ctx, tx, conv.ID)
	}); err != nil {
		h.log.Printf("chat-admin: delete: %v", err)
		return apierr.ErrInternal()
	}

	return c.NoContent()
}

func (h *ChatHandler) AdminBanUser(c *app.Ctx) error {
	ctx := c.R.Context()
	id := c.Param("id")

	var req struct {
		Reason string `json:"reason"`
	}
	if err := c.Decode(&req); err != nil {
		return apierr.ErrBadRequest()
	}

	userID, err := queries.GetUserIDByConversationID(ctx, c.DB().DB, id)
	if err != nil {
		h.log.Printf("chat-admin: get user: %v", err)
		return apierr.ErrInternal()
	}
	if userID == "" {
		return apierr.ErrNotFound()
	}

	if err := queries.BanUser(ctx, c.DB().DB, userID, req.Reason); err != nil {
		h.log.Printf("chat-admin: ban: %v", err)
		return apierr.ErrInternal()
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "banned"})
}

func (h *ChatHandler) AdminUnbanUser(c *app.Ctx) error {
	ctx := c.R.Context()
	id := c.Param("id")

	userID, err := queries.GetUserIDByConversationID(ctx, c.DB().DB, id)
	if err != nil {
		h.log.Printf("chat-admin: get user: %v", err)
		return apierr.ErrInternal()
	}
	if userID == "" {
		return apierr.ErrNotFound()
	}

	if err := queries.UnbanUser(ctx, c.DB().DB, userID); err != nil {
		h.log.Printf("chat-admin: unban: %v", err)
		return apierr.ErrInternal()
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "unbanned"})
}
