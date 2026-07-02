import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  adminGetStats,
  adminListOrders,
  type AdminStats,
  type AdminOrder,
} from '../../api/client';
import PageHeader from '../../components/PageHeader';
import { useSiteConfig } from '../../context/SiteConfigContext';
import { formatPrice } from '../../utils/format';
import Skeleton, { SkeletonStatCards, SkeletonTable } from '../../components/Skeleton';

export default function AdminDashboard() {
  const [stats, setStats] = useState<AdminStats | null>(null);
  const [recentOrders, setRecentOrders] = useState<AdminOrder[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const { currency_symbol } = useSiteConfig();

  useEffect(() => {
    Promise.all([
      adminGetStats(),
      adminListOrders({ page: '1', per_page: '5' }),
    ])
      .then(([statsData, ordersData]) => {
        setStats(statsData);
        setRecentOrders(ordersData.orders);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'Failed to load data');
      })
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="page">
        <PageHeader title="Dashboard" />
        <SkeletonStatCards count={5} />
        <div className="admin-section">
          <Skeleton variant="text" width="30%" />
          <SkeletonTable rows={5} cols={4} />
        </div>
      </div>
    );
  }

  return (
    <div className="page">
      <PageHeader
        title="Dashboard"
        actions={
          <div style={{ display: 'flex', gap: '0.5rem' }}>
            <Link to="/admin/projects/new" className="btn btn-primary btn-small">New Project</Link>
            <Link to="/admin/discount-codes" className="btn btn-secondary btn-small">New Discount</Link>
          </div>
        }
      />

      {error && <p className="error">{error}</p>}

      {stats && (
        <div className="admin-stat-cards">
          <div className="admin-stat-card">
            <span className="admin-stat-value">{formatPrice(stats.total_revenue_cents, currency_symbol)}</span>
            <span className="admin-stat-label">Total Revenue</span>
          </div>
          <div className="admin-stat-card">
            <span className="admin-stat-value">{formatPrice(stats.revenue_30d_cents, currency_symbol)}</span>
            <span className="admin-stat-label">Last 30 Days</span>
          </div>
          <div className="admin-stat-card">
            <span className="admin-stat-value">{stats.total_orders}</span>
            <span className="admin-stat-label">Orders</span>
          </div>
          <div className="admin-stat-card">
            <span className="admin-stat-value">{stats.total_licenses}</span>
            <span className="admin-stat-label">Licenses</span>
          </div>
          <div className="admin-stat-card">
            <span className="admin-stat-value">{stats.total_activations}</span>
            <span className="admin-stat-label">Activations</span>
          </div>
        </div>
      )}

      {recentOrders.length > 0 && (
        <div className="admin-section">
          <div className="admin-section-header">
            <h2>Recent Orders</h2>
            <Link to="/admin/orders" className="btn btn-secondary btn-small">View all</Link>
          </div>
          <table className="admin-table">
            <thead>
              <tr>
                <th>Date</th>
                <th>App</th>
                <th>Price</th>
                <th>Discount</th>
              </tr>
            </thead>
            <tbody>
              {recentOrders.map((o) => (
                <tr key={o.id}>
                  <td>{new Date(o.created_at).toLocaleDateString()}</td>
                  <td>{o.app_name || '\u2014'}</td>
                  <td>{formatPrice(o.price_paid_cents, currency_symbol)}</td>
                  <td>{o.discount_label || '\u2014'}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
