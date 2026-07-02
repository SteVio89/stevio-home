package queries

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
)

type AdminStats struct {
	TotalRevenueCents int `json:"total_revenue_cents"`
	Revenue30dCents   int `json:"revenue_30d_cents"`
	TotalOrders       int `json:"total_orders"`
	TotalLicenses     int `json:"total_licenses"`
	TotalActivations  int `json:"total_activations"`
}

func GetAdminStats(ctx context.Context, db *sql.DB) (*AdminStats, error) {
	var s AdminStats
	err := db.QueryRowContext(ctx, `
		SELECT
			COALESCE((SELECT SUM(price_paid_cents) FROM orders WHERE email != 'DELETED'), 0),
			COALESCE((SELECT SUM(price_paid_cents) FROM orders WHERE email != 'DELETED' AND created_at >= (NOW() - INTERVAL '30 days')), 0),
			COALESCE((SELECT COUNT(*) FROM orders WHERE email != 'DELETED'), 0),
			COALESCE((SELECT COUNT(*) FROM licenses), 0),
			COALESCE((SELECT COUNT(*) FROM activations), 0)
	`).Scan(&s.TotalRevenueCents, &s.Revenue30dCents, &s.TotalOrders, &s.TotalLicenses, &s.TotalActivations)
	return &s, err
}

// AppSalesRow holds per-app aggregates for the sales dashboard.
type AppSalesRow struct {
	AppID        string `json:"app_id"`
	AppName      string `json:"app_name"`
	OrderCount   int    `json:"order_count"`
	RevenueCents int    `json:"revenue_cents"`
}

// SalesReport holds the full sales dashboard response.
type SalesReport struct {
	Rows         []AppSalesRow `json:"rows"`
	TotalOrders  int           `json:"total_orders"`
	TotalRevenue int           `json:"total_revenue_cents"`
}

// GetSalesReport returns per-app order counts and revenue, optionally filtered by date range.
// startDate and endDate must be empty strings or "YYYY-MM-DD" format.
//
// The display name comes from the project entity_translation (post-009 the
// project owns titles, the app no longer has a 'name' translation). Each
// order has app_id; we join orders → apps → projects → entity_translations.
func GetSalesReport(ctx context.Context, db *sql.DB, startDate, endDate, defaultLocale string) (*SalesReport, error) {
	var (
		args        = []any{defaultLocale}
		whereClause = ""
	)

	addArg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	if startDate != "" {
		whereClause += " AND o.created_at >= " + addArg(startDate+"T00:00:00.000Z")
	}
	if endDate != "" {
		whereClause += " AND o.created_at <= " + addArg(endDate+"T23:59:59.999Z")
	}

	q := `
		SELECT o.app_id,
		       COALESCE(tn.value, '[deleted]') AS app_name,
		       COUNT(*) AS order_count,
		       COALESCE(SUM(o.price_paid_cents), 0) AS revenue_cents
		FROM orders o
		LEFT JOIN apps a ON a.id = o.app_id
		LEFT JOIN entity_translations tn
		       ON tn.entity_type = 'project'
		      AND tn.entity_id = a.project_id
		      AND tn.field = 'title'
		      AND tn.locale = $1
		WHERE o.email != 'DELETED'` + whereClause + `
		GROUP BY o.app_id, tn.value
		ORDER BY revenue_cents DESC, order_count DESC`

	// Sanity: defensive trim of any leading whitespace introduced by manual concat.
	q = strings.TrimSpace(q)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	report := &SalesReport{Rows: []AppSalesRow{}}
	for rows.Next() {
		var row AppSalesRow
		if err := rows.Scan(&row.AppID, &row.AppName, &row.OrderCount, &row.RevenueCents); err != nil {
			return nil, err
		}
		report.Rows = append(report.Rows, row)
		report.TotalOrders += row.OrderCount
		report.TotalRevenue += row.RevenueCents
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return report, nil
}
