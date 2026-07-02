package queries

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/SteVio89/stevio-home/db/models"
	"github.com/SteVio89/stevio-home/dbutil"
)

// OrderListFilter controls pagination and optional filtering for admin order listing.
type OrderListFilter struct {
	Page    int
	PerPage int
	AppID   string
	From    string // ISO date string
	To      string // ISO date string
}

// AdminOrderItem represents a single order row in the admin list.
type AdminOrderItem struct {
	ID                 string    `json:"id"`
	PaymentSession     string    `json:"payment_session"`
	Email              string    `json:"email"`
	AppID              string    `json:"app_id"`
	AppName            string    `json:"app_name"`
	PricePaidCents     int       `json:"price_paid_cents"`
	OriginalPriceCents *int      `json:"original_price_cents,omitempty"`
	DiscountLabel      *string   `json:"discount_label,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

// OrderListResult is the paginated response for admin order listing.
type OrderListResult struct {
	Orders []AdminOrderItem `json:"orders"`
	Total  int              `json:"total"`
}

// ListAllOrders returns a paginated, optionally filtered list of all orders for admin use.
func ListAllOrders(ctx context.Context, db *sql.DB, defaultLocale string, filter OrderListFilter) (*OrderListResult, error) {
	filter.Page, filter.PerPage = clampPagination(filter.Page, filter.PerPage)

	var where []string
	var args []any

	if filter.AppID != "" {
		where = append(where, "o.app_id = ?")
		args = append(args, filter.AppID)
	}
	if filter.From != "" {
		where = append(where, "o.created_at >= ?")
		args = append(args, filter.From)
	}
	if filter.To != "" {
		where = append(where, "o.created_at <= ?")
		args = append(args, filter.To)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	// Count total matching rows. The WHERE template uses `?` placeholders;
	// NumberPlaceholders rewrites them to $1..$N matching args positionally.
	countQuery := dbutil.NumberPlaceholders(
		fmt.Sprintf(`SELECT COUNT(*) FROM orders o %s`, whereClause), 0)
	var total int
	if err := db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("ListAllOrders count: %w", err)
	}

	offset := (filter.Page - 1) * filter.PerPage

	// The data query prepends `defaultLocale` and appends `perPage, offset` to
	// `args`. All placeholders (including the LIMIT/OFFSET) use `?` so
	// NumberPlaceholders can number them in a single pass.
	dataQuery := dbutil.NumberPlaceholders(fmt.Sprintf(`
		SELECT o.id, o.payment_session, o.email, o.app_id,
		       COALESCE(tn.value, ''), o.price_paid_cents,
		       o.original_price_cents, o.discount_label, o.created_at
		FROM orders o
		LEFT JOIN apps a ON a.id = o.app_id
		LEFT JOIN entity_translations tn ON tn.entity_type = 'project' AND tn.entity_id = a.project_id AND tn.field = 'title' AND tn.locale = ?
		%s
		ORDER BY o.created_at DESC
		LIMIT ? OFFSET ?`, whereClause), 0)

	dataArgs := append([]any{defaultLocale}, args...)
	dataArgs = append(dataArgs, filter.PerPage, offset)
	rows, err := db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, fmt.Errorf("ListAllOrders query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var orders []AdminOrderItem
	for rows.Next() {
		var item AdminOrderItem
		if err := rows.Scan(
			&item.ID, &item.PaymentSession, &item.Email, &item.AppID,
			&item.AppName, &item.PricePaidCents,
			&item.OriginalPriceCents, &item.DiscountLabel, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("ListAllOrders scan: %w", err)
		}
		orders = append(orders, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ListAllOrders rows: %w", err)
	}
	if orders == nil {
		orders = []AdminOrderItem{}
	}

	return &OrderListResult{Orders: orders, Total: total}, nil
}

func OrderExistsByPaymentSession(ctx context.Context, db *sql.DB, sessionID string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM orders WHERE payment_session = $1)`, sessionID).Scan(&exists)
	return exists, err
}

// OrderDiscountSnapshot captures the discount state at the moment of purchase.
// All fields are optional — use the zero value when no discount was applied.
type OrderDiscountSnapshot struct {
	OriginalPriceCents *int
	DiscountLabel      *string
	DiscountType       *string
	DiscountValue      *int
}

func GetOrderByPaymentSession(ctx context.Context, db *sql.DB, sessionID string) (*models.Order, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, payment_session, email, app_id, price_paid_cents, refunded, created_at
		FROM orders WHERE payment_session = $1 LIMIT 1`, sessionID)
	var o models.Order
	// email and app_id may be NULL for poison-pill stub rows (inserted when refund
	// arrives before the order). Use sql.NullString to handle both cases.
	var email, appID sql.NullString
	err := row.Scan(&o.ID, &o.PaymentSession, &email, &appID, &o.PricePaidCents, &o.Refunded, &o.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	o.Email = email.String
	o.AppID = appID.String
	return &o, nil
}

// InsertRefundStub inserts a placeholder order row when a refund webhook
// arrives before the order webhook. Only payment_session and refunded are set.
// email and app_id are left NULL (not empty strings).
func InsertRefundStub(ctx context.Context, db *sql.DB, paymentSession string) error {
	id := dbutil.NewID()
	_, err := db.ExecContext(ctx,
		`INSERT INTO orders (id, payment_session, refunded, price_paid_cents)
		 VALUES ($1, $2, TRUE, 0)`,
		id, paymentSession)
	return err
}

// FulfillStubOrder updates a stub order row (inserted by InsertRefundStub)
// with the real order data. The refunded flag remains 1.
func FulfillStubOrder(ctx context.Context, tx *sql.Tx, paymentSession, email, appID string,
	pricePaidCents int, discountCodeID, autoDiscountID *string, snapshot OrderDiscountSnapshot, consentGivenAt string) (*models.Order, error) {
	var consentVal sql.NullString
	if consentGivenAt != "" {
		consentVal = sql.NullString{String: consentGivenAt, Valid: true}
	}
	row := tx.QueryRowContext(ctx, `
		UPDATE orders SET email = $1, app_id = $2, price_paid_cents = $3,
			discount_code_id = $4, auto_discount_id = $5,
			original_price_cents = $6, discount_label = $7, discount_type = $8, discount_value = $9,
			consent_given_at = $10
		WHERE payment_session = $11 AND refunded = TRUE
		RETURNING id, payment_session, email, app_id, price_paid_cents,
		          discount_code_id, auto_discount_id,
		          original_price_cents, discount_label, discount_type, discount_value,
		          consent_given_at, created_at`,
		email, appID, pricePaidCents, discountCodeID, autoDiscountID,
		snapshot.OriginalPriceCents, snapshot.DiscountLabel, snapshot.DiscountType, snapshot.DiscountValue,
		consentVal,
		paymentSession)
	var o models.Order
	var consentScanned sql.NullString
	err := row.Scan(&o.ID, &o.PaymentSession, &o.Email, &o.AppID, &o.PricePaidCents,
		&o.DiscountCodeID, &o.AutoDiscountID,
		&o.OriginalPriceCents, &o.DiscountLabel, &o.DiscountType, &o.DiscountValue,
		&consentScanned, &o.CreatedAt)
	if err != nil {
		return nil, err
	}
	if consentScanned.Valid {
		o.ConsentGivenAt = &consentScanned.String
	}
	o.Refunded = true
	return &o, nil
}

func InsertOrder(ctx context.Context, tx *sql.Tx, paymentSession, email, appID string, pricePaidCents int, discountCodeID, autoDiscountID *string, snapshot OrderDiscountSnapshot, consentGivenAt string) (*models.Order, error) {
	id := dbutil.NewID()
	var consentVal sql.NullString
	if consentGivenAt != "" {
		consentVal = sql.NullString{String: consentGivenAt, Valid: true}
	}
	row := tx.QueryRowContext(ctx, `
		INSERT INTO orders (id, payment_session, email, app_id, price_paid_cents, refunded, discount_code_id, auto_discount_id,
		                    original_price_cents, discount_label, discount_type, discount_value, consent_given_at)
		VALUES ($1, $2, $3, $4, $5, FALSE, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, payment_session, email, app_id, price_paid_cents, refunded, discount_code_id, auto_discount_id,
		          original_price_cents, discount_label, discount_type, discount_value, consent_given_at, created_at`,
		id, paymentSession, email, appID, pricePaidCents, discountCodeID, autoDiscountID,
		snapshot.OriginalPriceCents, snapshot.DiscountLabel, snapshot.DiscountType, snapshot.DiscountValue,
		consentVal)
	var o models.Order
	var refundedDummy bool // always false for new orders, scanned and discarded
	var consentScanned sql.NullString
	err := row.Scan(&o.ID, &o.PaymentSession, &o.Email, &o.AppID, &o.PricePaidCents,
		&refundedDummy,
		&o.DiscountCodeID, &o.AutoDiscountID,
		&o.OriginalPriceCents, &o.DiscountLabel, &o.DiscountType, &o.DiscountValue,
		&consentScanned, &o.CreatedAt)
	if err != nil {
		return nil, err
	}
	if consentScanned.Valid {
		o.ConsentGivenAt = &consentScanned.String
	}
	return &o, nil
}
