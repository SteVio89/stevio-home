package queries

import (
	"context"
	"database/sql"
	"time"

	"github.com/SteVio89/stevio-home/db/models"
)

// UserSession is a session record safe to expose in data exports (no email field).
type UserSession struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// UserOrder is an order record joined with the app name.
type UserOrder struct {
	ID                 string    `json:"id"`
	AppName            string    `json:"app_name"`
	AppID              string    `json:"app_id"`
	PricePaidCents     int       `json:"price_paid_cents"`
	OriginalPriceCents *int      `json:"original_price_cents,omitempty"`
	DiscountLabel      *string   `json:"discount_label,omitempty"`
	DiscountType       *string   `json:"discount_type,omitempty"`
	DiscountValue      *int      `json:"discount_value,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

// UserDownloadToken is a download token record safe to expose in data exports.
type UserDownloadToken struct {
	Token     string    `json:"token"`
	AppID     string    `json:"app_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

// GetUserSessions returns all sessions for the given email hash, newest first.
func GetUserSessions(ctx context.Context, db *sql.DB, emailHash string) ([]UserSession, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, created_at, expires_at
		FROM sessions WHERE email = $1
		ORDER BY created_at DESC`, emailHash)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []UserSession{}
	for rows.Next() {
		var s UserSession
		if err := rows.Scan(&s.ID, &s.CreatedAt, &s.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// GetUserOrders returns all orders with app name for the given email hash, newest first.
func GetUserOrders(ctx context.Context, db *sql.DB, emailHash, defaultLocale string) ([]UserOrder, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT o.id, COALESCE(tn.value, '') AS app_name, o.app_id, o.price_paid_cents,
		       o.original_price_cents, o.discount_label, o.discount_type, o.discount_value,
		       o.created_at
		FROM orders o
		LEFT JOIN apps a ON a.id = o.app_id
		LEFT JOIN entity_translations tn ON tn.entity_type = 'project' AND tn.entity_id = a.project_id AND tn.field = 'title' AND tn.locale = $1
		WHERE o.email = $2
		ORDER BY o.created_at DESC`, defaultLocale, emailHash)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []UserOrder{}
	for rows.Next() {
		var u UserOrder
		if err := rows.Scan(&u.ID, &u.AppName, &u.AppID, &u.PricePaidCents,
			&u.OriginalPriceCents, &u.DiscountLabel, &u.DiscountType, &u.DiscountValue,
			&u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// GetUserActivationsByEmail returns all activations across all licenses for the given email hash.
func GetUserActivationsByEmail(ctx context.Context, db *sql.DB, emailHash string) ([]models.Activation, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT a.id, a.license_id, a.machine_hash, a.device_label, a.key_id, a.activated_at, a.last_seen_at
		FROM activations a
		JOIN licenses l ON l.id = a.license_id
		JOIN orders o ON o.id = l.order_id
		WHERE o.email = $1
		ORDER BY a.activated_at DESC`, emailHash)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []models.Activation{}
	for rows.Next() {
		a, err := scanActivation(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// GetUserDownloadTokensByEmail returns all download tokens via the license→order chain.
func GetUserDownloadTokensByEmail(ctx context.Context, db *sql.DB, emailHash string) ([]UserDownloadToken, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT dt.token, dt.app_id, dt.expires_at, dt.used, dt.created_at
		FROM download_tokens dt
		JOIN licenses l ON l.id = dt.license_id
		JOIN orders o ON o.id = l.order_id
		WHERE o.email = $1
		ORDER BY dt.created_at DESC`, emailHash)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []UserDownloadToken{}
	for rows.Next() {
		var t UserDownloadToken
		if err := rows.Scan(&t.Token, &t.AppID, &t.ExpiresAt, &t.Used, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// EraseUserData irreversibly removes a user's personal data in one transaction:
// sessions, auth tokens, download tokens, activations, licenses, support chat
// (conversations + messages + ban record), and the user row itself. Orders are
// anonymized rather than deleted so revenue/tax records survive. userID is the
// user's id (== chat_conversations.user_id); emailHash keys the email-hashed rows.
func EraseUserData(ctx context.Context, db *sql.DB, emailHash, userID string) error {
	return WithTx(ctx, db, func(tx *sql.Tx) error {
		// 1. Active sessions
		if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE email = $1`, emailHash); err != nil {
			return err
		}
		// 2. Pending/used auth tokens
		if _, err := tx.ExecContext(ctx, `DELETE FROM auth_tokens WHERE email = $1`, emailHash); err != nil {
			return err
		}
		// 3. Download tokens (via license → order chain)
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM download_tokens
			WHERE license_id IN (
				SELECT l.id FROM licenses l JOIN orders o ON o.id = l.order_id
				WHERE o.email = $1
			)`, emailHash); err != nil {
			return err
		}
		// 4. Device activations (via license → order chain)
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM activations
			WHERE license_id IN (
				SELECT l.id FROM licenses l JOIN orders o ON o.id = l.order_id
				WHERE o.email = $1
			)`, emailHash); err != nil {
			return err
		}
		// 5. Licenses
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM licenses
			WHERE order_id IN (SELECT id FROM orders WHERE email = $1)`, emailHash); err != nil {
			return err
		}
		// 6. Anonymise orders — preserve for revenue/tax records
		if _, err := tx.ExecContext(ctx, `UPDATE orders SET email = 'DELETED' WHERE email = $1`, emailHash); err != nil {
			return err
		}
		// 7. Support chat — a conversation may hold a plaintext email (from the
		// "Share Email" action) plus the user's message history. Delete both
		// (messages cascade inside this helper).
		if err := DeleteConversationsByUserID(ctx, tx, userID); err != nil {
			return err
		}
		// 8. Chat ban record, if any.
		if _, err := tx.ExecContext(ctx, `DELETE FROM chat_bans WHERE user_id = $1`, userID); err != nil {
			return err
		}
		// 9. The user row itself — remove the residual pseudonymous email_hash so
		// no identifier survives erasure. A later login re-creates a fresh member.
		if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE email_hash = $1`, emailHash); err != nil {
			return err
		}
		return nil
	})
}

// DeleteSessionsByEmail deletes all sessions for the given email hash and returns the row count.
func DeleteSessionsByEmail(ctx context.Context, db *sql.DB, emailHash string) (int64, error) {
	result, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE email = $1`, emailHash)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// VoidOrder hard-deletes an order and all its dependent records (activations, download tokens,
// licenses). Unlike EraseUserData, this does not anonymise — it removes the order entirely.
func VoidOrder(ctx context.Context, db *sql.DB, orderID string) error {
	return WithTx(ctx, db, func(tx *sql.Tx) error {
		// 1. Activations (via licenses)
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM activations
			WHERE license_id IN (SELECT id FROM licenses WHERE order_id = $1)`, orderID); err != nil {
			return err
		}
		// 2. Download tokens (via licenses)
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM download_tokens
			WHERE license_id IN (SELECT id FROM licenses WHERE order_id = $1)`, orderID); err != nil {
			return err
		}
		// 3. Licenses
		if _, err := tx.ExecContext(ctx, `DELETE FROM licenses WHERE order_id = $1`, orderID); err != nil {
			return err
		}
		// 4. Order
		if _, err := tx.ExecContext(ctx, `DELETE FROM orders WHERE id = $1`, orderID); err != nil {
			return err
		}
		return nil
	})
}
