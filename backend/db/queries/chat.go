package queries

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/dbutil"
)

// --- Conversations ---

func CreateConversation(ctx context.Context, db *sql.DB, userID, displayName string) (*models.ChatConversation, error) {
	id := dbutil.NewID()
	now := time.Now().UTC()
	_, err := db.ExecContext(ctx,
		`INSERT INTO chat_conversations (id, user_id, display_name, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		id, userID, displayName, now, now)
	if err != nil {
		return nil, fmt.Errorf("CreateConversation: %w", err)
	}
	return &models.ChatConversation{
		ID: id, UserID: userID, DisplayName: displayName,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}

func GetConversationByUserID(ctx context.Context, db *sql.DB, userID string) (*models.ChatConversation, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, user_id, display_name, email, notified_at, created_at, updated_at
		 FROM chat_conversations WHERE user_id = $1`, userID)
	return scanConversation(row.Scan)
}

func GetConversationByID(ctx context.Context, db *sql.DB, id string) (*models.ChatConversation, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, user_id, display_name, email, notified_at, created_at, updated_at
		 FROM chat_conversations WHERE id = $1`, id)
	return scanConversation(row.Scan)
}

func scanConversation(scan func(dest ...any) error) (*models.ChatConversation, error) {
	var c models.ChatConversation
	var email sql.NullString
	var notifiedAt sql.NullTime
	err := scan(&c.ID, &c.UserID, &c.DisplayName, &email, &notifiedAt, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if email.Valid {
		c.Email = &email.String
	}
	if notifiedAt.Valid {
		c.NotifiedAt = &notifiedAt.Time
	}
	return &c, nil
}

func ConversationExistsForUser(ctx context.Context, db *sql.DB, userID string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM chat_conversations WHERE user_id = $1`, userID).Scan(&count)
	return count > 0, err
}

func ActiveDisplayNameExists(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM chat_conversations WHERE display_name = $1`, name).Scan(&count)
	return count > 0, err
}

func UpdateConversationEmail(ctx context.Context, db *sql.DB, id, email, displayName string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE chat_conversations SET email = $1, display_name = $2, updated_at = NOW()
		 WHERE id = $3`,
		email, displayName, id)
	return err
}

func UpdateConversationUpdatedAt(ctx context.Context, q querier, id string) error {
	_, err := q.ExecContext(ctx,
		`UPDATE chat_conversations SET updated_at = NOW() WHERE id = $1`, id)
	return err
}

func SetConversationNotified(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE chat_conversations SET notified_at = NOW() WHERE id = $1`, id)
	return err
}

func DeleteConversation(ctx context.Context, tx *sql.Tx, id string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM chat_messages WHERE conversation_id = $1`, id); err != nil {
		return fmt.Errorf("delete chat messages: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM chat_conversations WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete chat conversation: %w", err)
	}
	return nil
}

// --- Messages ---

func CreateMessage(ctx context.Context, q querier, conversationID, sender, body string) (*models.ChatMessage, error) {
	id := dbutil.NewID()
	now := time.Now().UTC()
	_, err := q.ExecContext(ctx,
		`INSERT INTO chat_messages (id, conversation_id, sender, body, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		id, conversationID, sender, body, now)
	if err != nil {
		return nil, fmt.Errorf("CreateMessage: %w", err)
	}
	return &models.ChatMessage{
		ID: id, ConversationID: conversationID, Sender: sender, Body: body, CreatedAt: now,
	}, nil
}

func GetMessagesByConversationID(ctx context.Context, db *sql.DB, conversationID string) ([]models.ChatMessage, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, conversation_id, sender, body, read_at, created_at
		 FROM chat_messages WHERE conversation_id = $1 ORDER BY created_at ASC`, conversationID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var msgs []models.ChatMessage
	for rows.Next() {
		var m models.ChatMessage
		var readAt sql.NullString
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Sender, &m.Body, &readAt, &m.CreatedAt); err != nil {
			return nil, err
		}
		if readAt.Valid {
			m.ReadAt = &readAt.String
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

func MarkMessagesRead(ctx context.Context, db *sql.DB, conversationID, sender string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE chat_messages SET read_at = NOW() WHERE conversation_id = $1 AND sender = $2 AND read_at IS NULL`,
		conversationID, sender)
	return err
}

func HasUnreadMessages(ctx context.Context, db *sql.DB, conversationID, sender string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM chat_messages WHERE conversation_id = $1 AND sender = $2 AND read_at IS NULL`,
		conversationID, sender).Scan(&count)
	return count > 0, err
}

// UnreadConversationCount returns the number of conversations with unread user messages (for admin badge).
func UnreadConversationCount(ctx context.Context, db *sql.DB) (int, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT cm.conversation_id) FROM chat_messages cm
		 JOIN chat_conversations cc ON cc.id = cm.conversation_id
		 WHERE cm.sender = 'user' AND cm.read_at IS NULL`).Scan(&count)
	return count, err
}

// --- Admin list ---

type ChatListFilter struct {
	Page    int
	PerPage int
}

type ChatListItem struct {
	ID                 string    `json:"id"`
	DisplayName        string    `json:"display_name"`
	Email              *string   `json:"email"`
	HasUnread          bool      `json:"has_unread"`
	LastMessagePreview string    `json:"last_message_preview"`
	MessageCount       int       `json:"message_count"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type ChatListResult struct {
	Items []ChatListItem `json:"items"`
	Total int            `json:"total"`
}

func ListConversations(ctx context.Context, db *sql.DB, filter ChatListFilter) (*ChatListResult, error) {
	filter.Page, filter.PerPage = dbutil.ClampPagination(filter.Page, filter.PerPage)

	var total int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM chat_conversations`).Scan(&total); err != nil {
		return nil, fmt.Errorf("ListConversations count: %w", err)
	}

	offset := (filter.Page - 1) * filter.PerPage
	rows, err := db.QueryContext(ctx, `
		SELECT cc.id, cc.display_name, cc.email,
		       (SELECT COUNT(*) FROM chat_messages cm WHERE cm.conversation_id = cc.id AND cm.sender = 'user' AND cm.read_at IS NULL) > 0 AS has_unread,
		       COALESCE((SELECT SUBSTR(cm2.body, 1, 50) FROM chat_messages cm2 WHERE cm2.conversation_id = cc.id ORDER BY cm2.created_at DESC LIMIT 1), '') AS last_preview,
		       (SELECT COUNT(*) FROM chat_messages cm3 WHERE cm3.conversation_id = cc.id) AS msg_count,
		       cc.created_at, cc.updated_at
		FROM chat_conversations cc
		ORDER BY cc.updated_at DESC
		LIMIT $1 OFFSET $2`, filter.PerPage, offset)
	if err != nil {
		return nil, fmt.Errorf("ListConversations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]ChatListItem, 0)
	for rows.Next() {
		var item ChatListItem
		var hasUnread bool
		if err := rows.Scan(&item.ID, &item.DisplayName, &item.Email, &hasUnread, &item.LastMessagePreview, &item.MessageCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		item.HasUnread = hasUnread
		items = append(items, item)
	}
	return &ChatListResult{Items: items, Total: total}, rows.Err()
}

// --- Bans ---

func IsUserBanned(ctx context.Context, db *sql.DB, userID string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM chat_bans WHERE user_id = $1`, userID).Scan(&count)
	return count > 0, err
}

func BanUser(ctx context.Context, db *sql.DB, userID, reason string) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO chat_bans (user_id, reason, created_at) VALUES ($1, $2, NOW())
		 ON CONFLICT (user_id) DO UPDATE SET reason = $3, created_at = NOW()`,
		userID, reason, reason)
	return err
}

func UnbanUser(ctx context.Context, db *sql.DB, userID string) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM chat_bans WHERE user_id = $1`, userID)
	return err
}

// GetUserIDByConversationID returns the user_id for the given conversation.
func GetUserIDByConversationID(ctx context.Context, db *sql.DB, id string) (string, error) {
	var userID string
	err := db.QueryRowContext(ctx,
		`SELECT user_id FROM chat_conversations WHERE id = $1`, id).Scan(&userID)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return userID, err
}

// LastMessagePreview returns first 50 chars of the most recent message.
func LastMessagePreview(ctx context.Context, db *sql.DB, conversationID string) (string, error) {
	var preview string
	err := db.QueryRowContext(ctx,
		`SELECT SUBSTR(body, 1, 50) FROM chat_messages WHERE conversation_id = $1 ORDER BY created_at DESC LIMIT 1`,
		conversationID).Scan(&preview)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return preview, err
}

// DeleteConversationsByUserID deletes all conversations and messages for a user (for ban cleanup).
func DeleteConversationsByUserID(ctx context.Context, tx *sql.Tx, userID string) error {
	rows, err := tx.QueryContext(ctx, `SELECT id FROM chat_conversations WHERE user_id = $1`, userID)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `DELETE FROM chat_messages WHERE conversation_id = $1`, id); err != nil {
			return err
		}
	}
	if len(ids) > 0 {
		args := make([]any, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		placeholders := dbutil.InPlaceholders(len(ids), 0)
		if _, err := tx.ExecContext(ctx, `DELETE FROM chat_conversations WHERE id IN (`+placeholders+`)`, args...); err != nil {
			return err
		}
	}
	return nil
}
