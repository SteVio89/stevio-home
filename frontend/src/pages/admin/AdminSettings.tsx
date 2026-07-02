import { useEffect, useState, useCallback } from 'react';
import { adminGetSettings, adminUpdateSetting, APIError } from '../../api/client';
import PageHeader from '../../components/PageHeader';
import Skeleton from '../../components/Skeleton';

const DEFAULTS: Record<string, string> = {
  currency_symbol: '€',
  currency_code: 'EUR',
  site_name: 'My Store',
  max_activations: '3',
  download_token_ttl_min: '15',
  magic_link_ttl_min: '15',
};

export default function AdminSettings() {
  const [form, setForm] = useState(DEFAULTS);
  const [maintenanceMode, setMaintenanceMode] = useState('0');
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  useEffect(() => {
    adminGetSettings()
      .then((s) => {
        const { maintenance_mode, ...rest } = s;
        setForm((f) => ({ ...f, ...rest }));
        if (maintenance_mode) setMaintenanceMode(maintenance_mode);
      })
      .catch((err) => setError(err instanceof APIError ? err.message : 'Failed to load settings'))
      .finally(() => setLoading(false));
  }, []);

  async function handleSave(ev: React.FormEvent) {
    ev.preventDefault();
    setSubmitting(true);
    setError('');
    setSuccess('');
    try {
      const results = await Promise.allSettled(
        Object.entries(form).map(([key, value]) => adminUpdateSetting(key, value))
      );
      const failed = results
        .map((r, i) => (r.status === 'rejected' ? Object.keys(form)[i] : null))
        .filter(Boolean);
      if (failed.length > 0) {
        setError(`Failed to save: ${failed.join(', ')}`);
      } else {
        setSuccess('Settings saved.');
      }
    } catch (err) {
      setError(err instanceof APIError ? err.message : 'Failed to save settings');
    } finally {
      setSubmitting(false);
    }
  }

  const handleMaintenanceToggle = useCallback(async (checked: boolean) => {
    const val = checked ? '1' : '0';
    setMaintenanceMode(val);
    setError('');
    setSuccess('');
    try {
      await adminUpdateSetting('maintenance_mode', val);
      setSuccess(checked ? 'Maintenance mode enabled.' : 'Maintenance mode disabled.');
    } catch {
      setError('Failed to update maintenance mode');
      setMaintenanceMode(checked ? '0' : '1');
    }
  }, []);

  if (loading) return (
    <div className="page">
      <PageHeader title="Site Settings" />
      <Skeleton variant="text" width="60%" count={5} />
    </div>
  );

  return (
    <div className="page">
      <PageHeader title="Site Settings" />

      <div className="admin-section">
        <div className="admin-section-header">
          <h2>
            Maintenance Mode
            {maintenanceMode === '1' && (
              <span className="maintenance-active-badge">ACTIVE</span>
            )}
          </h2>
        </div>
        <p className="admin-section-description">
          When enabled, the public store shows a "coming soon" page. Only admins can log in and access the site.
        </p>
        <label className="toggle-switch">
          <input
            type="checkbox"
            checked={maintenanceMode === '1'}
            onChange={(e) => handleMaintenanceToggle(e.target.checked)}
          />
          <span className="toggle-switch-track" />
          Enable maintenance mode
        </label>
      </div>

      <div className="admin-section">
        <h2>General</h2>
        <form onSubmit={handleSave} className="admin-form">
          <div className="form-row">
            <div className="form-group">
              <label htmlFor="cfg-site-name">Site name</label>
              <input
                id="cfg-site-name"
                type="text"
                value={form.site_name ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, site_name: e.target.value }))}
                maxLength={100}
                required
              />
            </div>
          </div>

          <h3>Currency</h3>
          <div className="form-row">
            <div className="form-group">
              <label htmlFor="cfg-currency-symbol">Currency symbol</label>
              <input
                id="cfg-currency-symbol"
                type="text"
                value={form.currency_symbol ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, currency_symbol: e.target.value }))}
                maxLength={10}
                required
              />
              <small>Shown before prices (e.g. €, $, £)</small>
            </div>
            <div className="form-group">
              <label htmlFor="cfg-currency-code">Currency code</label>
              <input
                id="cfg-currency-code"
                type="text"
                value={form.currency_code ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, currency_code: e.target.value.toUpperCase() }))}
                maxLength={10}
                required
              />
              <small>Used in structured data (e.g. EUR, USD, GBP)</small>
            </div>
          </div>

          <h3>License limits</h3>
          <div className="form-row">
            <div className="form-group">
              <label htmlFor="cfg-max-activations">Max activations per license</label>
              <input
                id="cfg-max-activations"
                type="number"
                min="1"
                value={form.max_activations ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, max_activations: e.target.value }))}
                required
              />
              <small>How many devices a single license can be activated on</small>
            </div>
          </div>

          <h3>Support</h3>
          <div className="form-row">
            <div className="form-group">
              <label htmlFor="cfg-support-email">Notification email</label>
              <input
                id="cfg-support-email"
                type="email"
                value={form.support_notification_email ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, support_notification_email: e.target.value }))}
                maxLength={255}
                placeholder="admin@example.com"
              />
              <small>Receive an email when a customer starts a support chat. Leave empty to disable.</small>
            </div>
          </div>

          <h3>Expiry</h3>
          <div className="form-row">
            <div className="form-group">
              <label htmlFor="cfg-dl-ttl">Download token TTL (minutes)</label>
              <input
                id="cfg-dl-ttl"
                type="number"
                min="1"
                value={form.download_token_ttl_min ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, download_token_ttl_min: e.target.value }))}
                required
              />
              <small>How long a download link remains valid after generation</small>
            </div>
            <div className="form-group">
              <label htmlFor="cfg-ml-ttl">Magic link TTL (minutes)</label>
              <input
                id="cfg-ml-ttl"
                type="number"
                min="1"
                value={form.magic_link_ttl_min ?? ''}
                onChange={(e) => setForm((f) => ({ ...f, magic_link_ttl_min: e.target.value }))}
                required
              />
              <small>How long a login email link remains valid</small>
            </div>
          </div>

          {error && <p className="error">{error}</p>}
          {success && <p className="success-message">{success}</p>}
          <div className="form-actions">
            <button type="submit" className="btn btn-primary" disabled={submitting}>
              {submitting ? 'Saving…' : 'Save Settings'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
