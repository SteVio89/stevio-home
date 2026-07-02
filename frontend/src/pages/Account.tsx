import { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { getLicenses, getOrders, exportUserData, deleteUserData, type License, type UserOrder } from '../api/client';
import { useAuth } from '../context/AuthContext';
import { useSiteConfig } from '../context/SiteConfigContext';
import { useLocale } from '../context/LocaleContext';
import { formatPrice } from '../utils/format';
import LicenseCard from '../components/LicenseCard';
import PageHeader from '../components/PageHeader';
import Skeleton from '../components/Skeleton';

export default function Account() {
  const { email, logout } = useAuth();
  const { currency_symbol } = useSiteConfig();
  const { locale } = useLocale();
  const { t } = useTranslation();
  const [licenses, setLicenses] = useState<License[]>([]);
  const [orders, setOrders] = useState<UserOrder[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Export state
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState('');

  // Delete state
  const [confirmEmail, setConfirmEmail] = useState('');
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState('');

  const fetchData = useCallback(async () => {
    try {
      const [licenseData, orderData] = await Promise.all([getLicenses(), getOrders()]);
      setLicenses(licenseData);
      setOrders(orderData);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('account.load_error'));
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Build a map from order_id → price_paid_cents for license cards
  const priceByOrderId = new Map(orders.map((o) => [o.id, o.price_paid_cents]));

  async function handleExport() {
    setExporting(true);
    setExportError('');
    try {
      await exportUserData();
    } catch (err) {
      setExportError(err instanceof Error ? err.message : t('account.export_error'));
    } finally {
      setExporting(false);
    }
  }

  async function handleDelete() {
    if (!confirmEmail) return;
    setDeleting(true);
    setDeleteError('');
    try {
      await deleteUserData(confirmEmail);
      await logout();
    } catch (err) {
      setDeleteError(err instanceof Error ? err.message : t('account.delete_error'));
      setDeleting(false);
    }
  }

  const deleteEnabled =
    confirmEmail.trim().toLowerCase() === (email ?? '').trim().toLowerCase() &&
    confirmEmail.trim() !== '';

  if (loading) {
    return (
      <div className="page">
        <PageHeader title={t('account.title')} />
        <div className="account-section-card" aria-hidden="true">
          <Skeleton variant="text" width="40%" />
          <Skeleton variant="table-row" count={3} />
        </div>
        <div className="account-section-card" aria-hidden="true">
          <Skeleton variant="text" width="30%" />
          <Skeleton variant="card" count={2} />
        </div>
      </div>
    );
  }

  return (
    <div className="page">
      <PageHeader title={t('account.title')} />

      {error && <p className="error">{error}</p>}

      {orders.length > 0 && (
        <div className="account-section-card">
          <h2>{t('account.orders_title')}</h2>
          <table className="orders-table account-orders-table">
            <thead>
              <tr>
                <th>{t('account.col_app')}</th>
                <th>{t('account.col_price')}</th>
                <th>{t('account.col_date')}</th>
              </tr>
            </thead>
            <tbody>
              {orders.map((order) => (
                <tr key={order.id}>
                  <td>{order.app_name}</td>
                  <td>{formatPrice(order.price_paid_cents, currency_symbol)}</td>
                  <td>{new Date(order.created_at).toLocaleDateString(locale)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <div className="account-section-card">
        <h2>{t('account.licenses_title')}</h2>

      {licenses.length === 0 ? (
        <p>{t('account.no_licenses')}</p>
      ) : (
        <div className="licenses-list">
          {licenses.map((license) => (
            <LicenseCard
              key={license.id}
              license={license}
              pricePaidCents={priceByOrderId.get(license.order_id)}
              onUpdate={fetchData}
            />
          ))}
        </div>
      )}
      </div>

      <div className="account-section-card">
        <h2>{t('account.data_title')}</h2>

        <div className="account-data-export">
          <p>{t('account.export_desc')}</p>
          {exportError && <p className="error">{exportError}</p>}
          <button
            className="btn btn-secondary"
            onClick={handleExport}
            disabled={exporting}
          >
            {exporting ? t('account.exporting') : t('account.export_button')}
          </button>
        </div>
      </div>

      <div className="danger-zone-card">
        <h3>{t('account.delete_title')}</h3>
        <p>{t('account.delete_desc')}</p>
        <div className="form-group">
          <label htmlFor="confirm-email">{t('account.delete_confirm_label')}</label>
          <input
            id="confirm-email"
            type="email"
            value={confirmEmail}
            onChange={(e) => setConfirmEmail(e.target.value)}
            placeholder={email ?? ''}
            autoComplete="off"
          />
        </div>
        {deleteError && <p className="error">{deleteError}</p>}
        <button
          className="btn btn-danger"
          onClick={handleDelete}
          disabled={!deleteEnabled || deleting}
        >
          {deleting ? t('account.deleting') : t('account.delete_button')}
        </button>
      </div>
    </div>
  );
}
