import { useEffect, useState } from 'react';
import { adminGetSales, type SalesReport, APIError } from '../../api/client';
import PageHeader from '../../components/PageHeader';
import { SkeletonStatCards, SkeletonTable } from '../../components/Skeleton';
import { useSiteConfig } from '../../context/SiteConfigContext';
import { formatPrice } from '../../utils/format';

export default function AdminSales() {
  const [start, setStart] = useState('');
  const [end, setEnd] = useState('');
  const [report, setReport] = useState<SalesReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  async function fetchReport(s?: string, e?: string) {
    setLoading(true);
    setError('');
    try {
      const data = await adminGetSales(s, e);
      setReport(data);
    } catch (err) {
      setError(err instanceof APIError ? err.message : 'Failed to load sales data');
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    fetchReport();
  }, []);

  function handleFilter(ev: React.FormEvent) {
    ev.preventDefault();
    fetchReport(start || undefined, end || undefined);
  }

  function handleReset() {
    setStart('');
    setEnd('');
    fetchReport();
  }

  const { currency_symbol } = useSiteConfig();
  const formatCents = (cents: number) => formatPrice(cents, currency_symbol);

  if (loading && !report) {
    return (
      <div className="page">
        <PageHeader title="Sales" />
        <SkeletonStatCards count={2} />
        <SkeletonTable rows={3} cols={3} />
      </div>
    );
  }

  return (
    <div className="page">
      <PageHeader title="Sales" />

      <form onSubmit={handleFilter} className="admin-sales-filter">
        <div className="admin-sales-filter-row">
          <div className="form-group">
            <label htmlFor="sales-start">From</label>
            <input
              id="sales-start"
              type="date"
              value={start}
              onChange={(e) => setStart(e.target.value)}
            />
          </div>
          <div className="form-group">
            <label htmlFor="sales-end">To</label>
            <input
              id="sales-end"
              type="date"
              value={end}
              onChange={(e) => setEnd(e.target.value)}
            />
          </div>
          <div className="admin-sales-filter-actions">
            <button type="submit" className="btn btn-primary btn-small" disabled={loading}>
              {loading ? 'Loading…' : 'Apply'}
            </button>
            {(start || end) && (
              <button type="button" className="btn btn-secondary btn-small" onClick={handleReset}>
                Reset
              </button>
            )}
          </div>
        </div>
      </form>

      {error && <p className="error">{error}</p>}

      {report && (
        <div className="admin-section">
          <div className="admin-sales-totals">
            <div className="admin-sales-stat">
              <span className="admin-sales-stat-value">{report.total_orders}</span>
              <span className="admin-sales-stat-label">Total orders</span>
            </div>
            <div className="admin-sales-stat">
              <span className="admin-sales-stat-value">{formatCents(report.total_revenue_cents)}</span>
              <span className="admin-sales-stat-label">Total revenue</span>
            </div>
          </div>

          {report.rows.length === 0 ? (
            <p className="admin-empty">No orders in this period.</p>
          ) : (
            <table className="admin-table">
              <thead>
                <tr>
                  <th>App</th>
                  <th>Orders</th>
                  <th>Revenue</th>
                </tr>
              </thead>
              <tbody>
                {report.rows.map((row) => (
                  <tr key={row.app_id}>
                    <td>{row.app_name}</td>
                    <td>{row.order_count}</td>
                    <td>{formatCents(row.revenue_cents)}</td>
                  </tr>
                ))}
              </tbody>
              <tfoot>
                <tr className="admin-sales-total-row">
                  <td><strong>Total</strong></td>
                  <td><strong>{report.total_orders}</strong></td>
                  <td><strong>{formatCents(report.total_revenue_cents)}</strong></td>
                </tr>
              </tfoot>
            </table>
          )}
        </div>
      )}
    </div>
  );
}
