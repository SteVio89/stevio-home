import { useEffect, useState } from 'react';
import {
  adminListOrders,
  adminListApps,
  adminVoidOrder,
  type AdminOrder,
  type AdminAppListItem,
} from '../../api/client';
import PageHeader from '../../components/PageHeader';
import ConfirmModal from '../../components/ConfirmModal';
import { SkeletonTable } from '../../components/Skeleton';
import { useSiteConfig } from '../../context/SiteConfigContext';
import { formatPrice } from '../../utils/format';
import { useToast } from '../../context/ToastContext';

const PER_PAGE = 20;

export default function AdminOrders() {
  const [orders, setOrders] = useState<AdminOrder[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [apps, setApps] = useState<AdminAppListItem[]>([]);
  const [appFilter, setAppFilter] = useState('');
  const [fromDate, setFromDate] = useState('');
  const [toDate, setToDate] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [voidTarget, setVoidTarget] = useState<string | null>(null);
  const { currency_symbol } = useSiteConfig();
  const { addToast } = useToast();

  useEffect(() => {
    adminListApps().then(setApps).catch(() => {});
  }, []);

  useEffect(() => {
    const params: Record<string, string> = {
      page: String(page),
      per_page: String(PER_PAGE),
    };
    if (appFilter) params.app_id = appFilter;
    if (fromDate) params.from = fromDate;
    if (toDate) params.to = `${toDate}T23:59:59Z`;
    adminListOrders(params)
      .then((res) => {
        setOrders(res.orders);
        setTotal(res.total);
      })
      .catch((err) =>
        setError(err instanceof Error ? err.message : 'Failed to load orders')
      )
      .finally(() => setLoading(false));
  }, [page, appFilter, fromDate, toDate]);

  async function handleVoid(id: string) {
    try {
      await adminVoidOrder(id);
      setOrders(orders.filter((o) => o.id !== id));
      setTotal((t) => t - 1);
      addToast('Order voided', 'success');
    } catch {
      addToast('Failed to void order', 'error');
    }
    setVoidTarget(null);
  }

  const totalPages = Math.ceil(total / PER_PAGE);

  return (
    <div className="page">
      <PageHeader title="Orders" />
      {error && <p className="error">{error}</p>}

      {voidTarget && (
        <ConfirmModal
          title="Void Order"
          message="Void this order? This cannot be undone. All associated licenses and activations will be removed."
          confirmLabel="Void"
          danger
          onConfirm={() => handleVoid(voidTarget)}
          onCancel={() => setVoidTarget(null)}
        />
      )}

      <div className="admin-section">
        <div className="admin-filters">
          <select
            value={appFilter}
            onChange={(e) => {
              setAppFilter(e.target.value);
              setPage(1);
            }}
          >
            <option value="">All Apps</option>
            {apps.map((a) => (
              <option key={a.id} value={a.id}>
                {a.project_title || a.bundle_id}
              </option>
            ))}
          </select>
          <input
            type="date"
            value={fromDate}
            onChange={(e) => {
              setFromDate(e.target.value);
              setPage(1);
            }}
            placeholder="From"
          />
          <input
            type="date"
            value={toDate}
            onChange={(e) => {
              setToDate(e.target.value);
              setPage(1);
            }}
            placeholder="To"
          />
        </div>
        {loading ? (
          <SkeletonTable rows={5} cols={7} />
        ) : orders.length === 0 ? (
          <p className="admin-empty">No orders found.</p>
        ) : (
          <>
            <table className="admin-table">
              <thead>
                <tr>
                  <th>Date</th>
                  <th>Email</th>
                  <th>App</th>
                  <th>Price</th>
                  <th>Discount</th>
                  <th>Session</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {orders.map((o) => (
                  <tr key={o.id}>
                    <td>{new Date(o.created_at).toLocaleDateString()}</td>
                    <td>
                      <code>{o.email.slice(0, 8)}...</code>
                    </td>
                    <td>{o.app_name || '\u2014'}</td>
                    <td>
                      {formatPrice(o.price_paid_cents, currency_symbol)}
                    </td>
                    <td>{o.discount_label || '\u2014'}</td>
                    <td>
                      <code>{o.payment_session.slice(0, 12)}...</code>
                    </td>
                    <td>
                      <button
                        className="btn btn-danger btn-small"
                        onClick={() => setVoidTarget(o.id)}
                      >
                        Void
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            <div className="admin-pagination">
              <span className="admin-pagination-info">{total} order{total !== 1 ? 's' : ''}</span>
              <div className="admin-pagination-controls">
                <button
                  className="btn btn-secondary btn-small"
                  disabled={page <= 1}
                  onClick={() => setPage((p) => p - 1)}
                >
                  Previous
                </button>
                <span>
                  Page {page} of {totalPages}
                </span>
                <button
                  className="btn btn-secondary btn-small"
                  disabled={page >= totalPages}
                  onClick={() => setPage((p) => p + 1)}
                >
                  Next
                </button>
              </div>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
