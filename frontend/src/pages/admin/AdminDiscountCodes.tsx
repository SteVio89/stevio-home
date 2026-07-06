import { useEffect, useState } from 'react';
import {
  adminListApps,
  adminListDiscountCodes,
  adminCreateDiscountCode,
  adminUpdateDiscountCode,
  adminDeleteDiscountCode,
  adminRestoreDiscountCode,
  adminPermanentDeleteDiscountCode,
  adminListAutoDiscounts,
  adminCreateAutoDiscount,
  adminUpdateAutoDiscount,
  adminDeleteAutoDiscount,
  adminRestoreAutoDiscount,
  type AdminAppListItem,
  type DiscountCode,
  type AutoDiscount,
  APIError,
} from '../../api/client';
import PageHeader from '../../components/PageHeader';
import ConfirmModal from '../../components/ConfirmModal';
import { SkeletonTable } from '../../components/Skeleton';
import { useSiteConfig } from '../../context/SiteConfigContext';
import { formatPrice } from '../../utils/format';

const EMPTY_CODE_FORM = {
  code: '',
  label: '',
  discount_type: 'percent' as 'percent' | 'fixed',
  discount_value: '',
  app_id: '',
  max_uses: '',
  expires_at: '',
  stackable: false,
};

const EMPTY_AUTO_FORM = {
  label: '',
  discount_type: 'percent' as 'percent' | 'fixed',
  discount_value: '',
  app_id: '',
  valid_from: '',
  expires_at: '',
};

// Convert a date-only string (YYYY-MM-DD) to an ISO-8601 end-of-day UTC timestamp.
function dateToEndOfDay(date: string): string {
  return `${date}T23:59:59Z`;
}

// Convert a date-only string (YYYY-MM-DD) to an ISO-8601 start-of-day UTC timestamp.
function dateToStartOfDay(date: string): string {
  return `${date}T00:00:00Z`;
}


export default function AdminDiscountCodes() {
  const [codes, setCodes] = useState<DiscountCode[]>([]);
  const [autoDiscounts, setAutoDiscounts] = useState<AutoDiscount[]>([]);
  const [apps, setApps] = useState<AdminAppListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [showCodeForm, setShowCodeForm] = useState(false);
  const [codeForm, setCodeForm] = useState(EMPTY_CODE_FORM);
  const [codeSubmitting, setCodeSubmitting] = useState(false);
  const [codeFormError, setCodeFormError] = useState('');

  const [showAutoForm, setShowAutoForm] = useState(false);
  const [autoForm, setAutoForm] = useState(EMPTY_AUTO_FORM);
  const [autoSubmitting, setAutoSubmitting] = useState(false);
  const [autoFormError, setAutoFormError] = useState('');

  const [activeTab, setActiveTab] = useState<'codes' | 'auto'>('codes');
  const [archiveTarget, setArchiveTarget] = useState<{ id: string; type: 'code' | 'auto'; label: string } | null>(null);
  const [permanentTarget, setPermanentTarget] = useState<{ id: string; label: string } | null>(null);
  const { currency_symbol } = useSiteConfig();

  useEffect(() => {
    Promise.all([adminListDiscountCodes(), adminListAutoDiscounts(), adminListApps()])
      .then(([c, a, ap]) => {
        setCodes(c);
        setAutoDiscounts(a);
        setApps(ap);
      })
      .catch((err) => setError(err instanceof APIError ? err.message : 'Failed to load'))
      .finally(() => setLoading(false));
  }, []);

  // --- Discount code handlers ---

  async function handleCreateCode(ev: React.FormEvent) {
    ev.preventDefault();
    setCodeFormError('');
    const value = parseInt(codeForm.discount_value, 10);
    if (!codeForm.code || isNaN(value) || value <= 0) {
      setCodeFormError('Code and a positive discount value are required.');
      return;
    }
    if (codeForm.discount_type === 'percent' && value > 100) {
      setCodeFormError('Percent discount cannot exceed 100.');
      return;
    }
    setCodeSubmitting(true);
    try {
      const created = await adminCreateDiscountCode({
        code: codeForm.code.trim().toUpperCase(),
        label: codeForm.label.trim(),
        discount_type: codeForm.discount_type,
        discount_value: value,
        app_id: codeForm.app_id || null,
        max_uses: codeForm.max_uses ? parseInt(codeForm.max_uses, 10) : null,
        expires_at: codeForm.expires_at ? dateToEndOfDay(codeForm.expires_at) : null,
        stackable: codeForm.stackable,
      });
      setCodes((prev) => [created, ...prev]);
      setCodeForm(EMPTY_CODE_FORM);
      setShowCodeForm(false);
    } catch (err) {
      setCodeFormError(err instanceof APIError ? err.message : 'Failed to create code');
    } finally {
      setCodeSubmitting(false);
    }
  }

  async function handleToggleCode(code: DiscountCode) {
    try {
      const updated = await adminUpdateDiscountCode(code.id, {
        label: code.label,
        discount_type: code.discount_type,
        discount_value: code.discount_value,
        app_id: code.app_id,
        max_uses: code.max_uses,
        expires_at: code.expires_at,
        active: !code.active,
        stackable: code.stackable,
      });
      // Preserve stats — RETURNING does not include joined order counts.
      setCodes((prev) =>
        prev.map((c) =>
          c.id === code.id
            ? { ...updated, order_count: c.order_count, revenue_cents: c.revenue_cents }
            : c
        )
      );
    } catch (err) {
      setError(err instanceof APIError ? err.message : 'Failed to update code');
    }
  }

  async function handleArchiveCode(id: string) {
    try {
      await adminDeleteDiscountCode(id);
      setCodes((prev) =>
        prev.map((c) =>
          c.id === id ? { ...c, deleted_at: new Date().toISOString() } : c
        )
      );
    } catch (err) {
      setError(err instanceof APIError ? err.message : 'Failed to archive code');
    }
    setArchiveTarget(null);
  }

  async function handleRestoreCode(id: string) {
    try {
      await adminRestoreDiscountCode(id);
      setCodes((prev) =>
        prev.map((c) => (c.id === id ? { ...c, deleted_at: null } : c))
      );
    } catch (err) {
      setError(err instanceof APIError ? err.message : 'Failed to restore code');
    }
  }

  async function handlePermanentDeleteCode(id: string) {
    try {
      await adminPermanentDeleteDiscountCode(id);
      setCodes((prev) => prev.filter((c) => c.id !== id));
    } catch (err) {
      setError(err instanceof APIError ? err.message : 'Failed to delete code');
    }
    setPermanentTarget(null);
  }

  // --- Auto-discount handlers ---

  async function handleCreateAuto(ev: React.FormEvent) {
    ev.preventDefault();
    setAutoFormError('');
    const value = parseInt(autoForm.discount_value, 10);
    if (isNaN(value) || value <= 0) {
      setAutoFormError('A positive discount value is required.');
      return;
    }
    if (autoForm.discount_type === 'percent' && value > 100) {
      setAutoFormError('Percent discount cannot exceed 100.');
      return;
    }
    setAutoSubmitting(true);
    try {
      const created = await adminCreateAutoDiscount({
        label: autoForm.label.trim(),
        discount_type: autoForm.discount_type,
        discount_value: value,
        app_id: autoForm.app_id || null,
        valid_from: autoForm.valid_from ? dateToStartOfDay(autoForm.valid_from) : null,
        expires_at: autoForm.expires_at ? dateToEndOfDay(autoForm.expires_at) : null,
      });
      setAutoDiscounts((prev) => [{ ...created, order_count: 0, revenue_cents: 0 }, ...prev]);
      setAutoForm(EMPTY_AUTO_FORM);
      setShowAutoForm(false);
    } catch (err) {
      setAutoFormError(err instanceof APIError ? err.message : 'Failed to create discount');
    } finally {
      setAutoSubmitting(false);
    }
  }

  async function handleToggleAuto(d: AutoDiscount) {
    try {
      const updated = await adminUpdateAutoDiscount(d.id, {
        label: d.label,
        discount_type: d.discount_type,
        discount_value: d.discount_value,
        app_id: d.app_id,
        valid_from: d.valid_from,
        expires_at: d.expires_at,
        active: !d.active,
      });
      // Preserve stats — RETURNING does not include joined order counts.
      setAutoDiscounts((prev) =>
        prev.map((a) =>
          a.id === d.id
            ? { ...updated, order_count: a.order_count, revenue_cents: a.revenue_cents }
            : a
        )
      );
    } catch (err) {
      setError(err instanceof APIError ? err.message : 'Failed to update discount');
    }
  }

  async function handleArchiveAuto(id: string) {
    try {
      await adminDeleteAutoDiscount(id);
      setAutoDiscounts((prev) =>
        prev.map((a) =>
          a.id === id ? { ...a, deleted_at: new Date().toISOString() } : a
        )
      );
    } catch (err) {
      setError(err instanceof APIError ? err.message : 'Failed to archive discount');
    }
    setArchiveTarget(null);
  }

  async function handleRestoreAuto(id: string) {
    try {
      await adminRestoreAutoDiscount(id);
      setAutoDiscounts((prev) =>
        prev.map((a) => (a.id === id ? { ...a, deleted_at: null } : a))
      );
    } catch (err) {
      setError(err instanceof APIError ? err.message : 'Failed to restore discount');
    }
  }

  // --- Formatting helpers ---

  function formatDiscount(type: string, value: number) {
    if (type === 'percent') return `${value}% off`;
    return `${formatPrice(value, currency_symbol)} off`;
  }

  function formatScope(appId: string | null) {
    if (!appId) return 'All apps';
    const app = apps.find((a) => a.id === appId);
    return app ? (app.project_title || app.bundle_id) : appId;
  }

  function formatDate(iso: string | null) {
    if (!iso) return '—';
    return new Date(iso).toLocaleDateString();
  }

  if (loading) return (
    <div className="page">
      <PageHeader title="Discounts" />
      <SkeletonTable rows={4} cols={6} />
    </div>
  );

  return (
    <div className="page">
      <PageHeader title="Discounts" />

      {error && <p className="error">{error}</p>}

      {archiveTarget && (
        <ConfirmModal
          title="Archive Discount"
          message={`Archive "${archiveTarget.label || 'this discount'}"? You can restore it later.`}
          confirmLabel="Archive"
          danger
          onConfirm={() => archiveTarget.type === 'code' ? handleArchiveCode(archiveTarget.id) : handleArchiveAuto(archiveTarget.id)}
          onCancel={() => setArchiveTarget(null)}
        />
      )}

      {permanentTarget && (
        <ConfirmModal
          title="Delete Permanently"
          message={`Permanently delete "${permanentTarget.label || 'this discount'}"? This cannot be undone.`}
          confirmLabel="Delete permanently"
          danger
          onConfirm={() => handlePermanentDeleteCode(permanentTarget.id)}
          onCancel={() => setPermanentTarget(null)}
        />
      )}

      <div className="tab-bar">
        <button className={`tab-bar-item${activeTab === 'codes' ? ' tab-bar-item-active' : ''}`} onClick={() => setActiveTab('codes')}>
          Discount Codes ({codes.length})
        </button>
        <button className={`tab-bar-item${activeTab === 'auto' ? ' tab-bar-item-active' : ''}`} onClick={() => setActiveTab('auto')}>
          Automatic Discounts ({autoDiscounts.length})
        </button>
      </div>

      {/* ── Discount Codes ── */}
      {activeTab === 'codes' && <div className="admin-section">
        <div className="admin-section-header">
          <h2>Discount Codes</h2>
          <button
            className="btn btn-primary btn-small"
            onClick={() => { setShowCodeForm((v) => !v); setCodeFormError(''); }}
          >
            {showCodeForm ? 'Cancel' : '+ New Code'}
          </button>
        </div>

        {showCodeForm && (
          <form onSubmit={handleCreateCode} className="admin-form">
            <div className="form-row">
              <div className="form-group">
                <label htmlFor="dc-code">Code *</label>
                <input
                  id="dc-code"
                  type="text"
                  placeholder="BF24"
                  value={codeForm.code}
                  onChange={(e) => setCodeForm((f) => ({ ...f, code: e.target.value }))}
                  required
                />
              </div>
              <div className="form-group">
                <label htmlFor="dc-label">Label</label>
                <input
                  id="dc-label"
                  type="text"
                  placeholder="Black Friday 2024"
                  value={codeForm.label}
                  onChange={(e) => setCodeForm((f) => ({ ...f, label: e.target.value }))}
                />
              </div>
            </div>

            <div className="form-row">
              <div className="form-group">
                <label htmlFor="dc-type">Discount type *</label>
                <select
                  id="dc-type"
                  value={codeForm.discount_type}
                  onChange={(e) => setCodeForm((f) => ({ ...f, discount_type: e.target.value as 'percent' | 'fixed' }))}
                >
                  <option value="percent">Percent off (%)</option>
                  <option value="fixed">Fixed amount off ({currency_symbol})</option>
                </select>
              </div>
              <div className="form-group">
                <label htmlFor="dc-value">
                  {codeForm.discount_type === 'percent' ? 'Percent (0–100) *' : 'Amount in cents *'}
                </label>
                <input
                  id="dc-value"
                  type="number"
                  min="1"
                  max={codeForm.discount_type === 'percent' ? 100 : undefined}
                  placeholder={codeForm.discount_type === 'percent' ? '20' : '500'}
                  value={codeForm.discount_value}
                  onChange={(e) => setCodeForm((f) => ({ ...f, discount_value: e.target.value }))}
                  required
                />
              </div>
            </div>

            <div className="form-row">
              <div className="form-group">
                <label htmlFor="dc-scope">Scope</label>
                <select
                  id="dc-scope"
                  value={codeForm.app_id}
                  onChange={(e) => setCodeForm((f) => ({ ...f, app_id: e.target.value }))}
                >
                  <option value="">Store-wide (all apps)</option>
                  {apps.map((a) => (
                    <option key={a.id} value={a.id}>{a.project_title || a.bundle_id}</option>
                  ))}
                </select>
              </div>
              <div className="form-group">
                <label htmlFor="dc-max-uses">Max uses (blank = unlimited)</label>
                <input
                  id="dc-max-uses"
                  type="number"
                  min="1"
                  placeholder="100"
                  value={codeForm.max_uses}
                  onChange={(e) => setCodeForm((f) => ({ ...f, max_uses: e.target.value }))}
                />
              </div>
            </div>

            <div className="form-row">
              <div className="form-group">
                <label htmlFor="dc-expires">Expires on (blank = no expiry)</label>
                <input
                  id="dc-expires"
                  type="date"
                  value={codeForm.expires_at}
                  onChange={(e) => setCodeForm((f) => ({ ...f, expires_at: e.target.value }))}
                />
              </div>
              <div className="form-group form-group-checkbox">
                <label htmlFor="dc-stackable">
                  <input
                    id="dc-stackable"
                    type="checkbox"
                    checked={codeForm.stackable}
                    onChange={(e) => setCodeForm((f) => ({ ...f, stackable: e.target.checked }))}
                  />
                  {' '}Stackable with automatic discounts
                </label>
                <p className="admin-section-description">
                  When checked, this code is applied on top of any active sale price.
                </p>
              </div>
            </div>

            {codeFormError && <p className="error">{codeFormError}</p>}
            <div className="form-actions">
              <button type="submit" className="btn btn-primary" disabled={codeSubmitting}>
                {codeSubmitting ? 'Creating…' : 'Create Code'}
              </button>
            </div>
          </form>
        )}

        {codes.length === 0 ? (
          <p className="admin-empty">No discount codes yet.</p>
        ) : (
          <table className="admin-table">
            <thead>
              <tr>
                <th>Code</th>
                <th>Label</th>
                <th>Discount</th>
                <th>Scope</th>
                <th>Stackable</th>
                <th>Uses</th>
                <th>Orders</th>
                <th>Revenue</th>
                <th>Expires</th>
                <th>Status</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {codes.map((code) => (
                <tr key={code.id} className={code.deleted_at || !code.active ? 'admin-row-inactive' : ''}>
                  <td><code className="app-id">{code.code}</code></td>
                  <td>{code.label || <span className="admin-empty-cell">—</span>}</td>
                  <td>{formatDiscount(code.discount_type, code.discount_value)}</td>
                  <td>{formatScope(code.app_id)}</td>
                  <td>
                    {code.stackable ? (
                      <span className="discount-status discount-status-active">Yes</span>
                    ) : (
                      <span className="admin-empty-cell">No</span>
                    )}
                  </td>
                  <td>
                    {code.uses}
                    {code.max_uses != null ? ` / ${code.max_uses}` : ''}
                  </td>
                  <td>{code.order_count}</td>
                  <td>{formatPrice(code.revenue_cents, currency_symbol)}</td>
                  <td>{formatDate(code.expires_at)}</td>
                  <td>
                    {code.deleted_at ? (
                      <span className="badge badge-danger">Archived</span>
                    ) : (
                      <span className={`discount-status discount-status-${code.active ? 'active' : 'inactive'}`}>
                        {code.active ? 'Active' : 'Inactive'}
                      </span>
                    )}
                  </td>
                  <td>
                    <div className="actions">
                      {!code.deleted_at && (
                        <button className="btn btn-secondary btn-small" onClick={() => handleToggleCode(code)}>
                          {code.active ? 'Deactivate' : 'Activate'}
                        </button>
                      )}
                      {code.deleted_at ? (
                        <>
                          <button className="btn btn-secondary btn-small" onClick={() => handleRestoreCode(code.id)}>
                            Restore
                          </button>
                          <button className="btn btn-danger btn-small" onClick={() => setPermanentTarget({ id: code.id, label: code.code })}>
                            Delete permanently
                          </button>
                        </>
                      ) : (
                        <button className="btn btn-danger btn-small" onClick={() => setArchiveTarget({ id: code.id, type: 'code', label: code.code })}>
                          Archive
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
            {codes.length > 0 && (
              <tfoot>
                <tr className="admin-sales-total-row">
                  <td colSpan={6}><strong>Total</strong></td>
                  <td><strong>{codes.reduce((s, c) => s + c.order_count, 0)}</strong></td>
                  <td><strong>{formatPrice(codes.reduce((s, c) => s + c.revenue_cents, 0), currency_symbol)}</strong></td>
                  <td colSpan={3} />
                </tr>
              </tfoot>
            )}
          </table>
        )}
      </div>}

      {/* ── Automatic Discounts ── */}
      {activeTab === 'auto' && <div className="admin-section">
        <div className="admin-section-header">
          <h2>Automatic Discounts</h2>
          <button
            className="btn btn-primary btn-small"
            onClick={() => { setShowAutoForm((v) => !v); setAutoFormError(''); }}
          >
            {showAutoForm ? 'Cancel' : '+ New Sale'}
          </button>
        </div>
        <p className="admin-section-description">
          Applied automatically to all visitors — no code required. Useful for limited-time sales.
        </p>

        {showAutoForm && (
          <form onSubmit={handleCreateAuto} className="admin-form">
            <div className="form-row">
              <div className="form-group">
                <label htmlFor="ad-label">Label</label>
                <input
                  id="ad-label"
                  type="text"
                  placeholder="Summer Sale"
                  value={autoForm.label}
                  onChange={(e) => setAutoForm((f) => ({ ...f, label: e.target.value }))}
                />
              </div>
              <div className="form-group">
                <label htmlFor="ad-scope">Scope</label>
                <select
                  id="ad-scope"
                  value={autoForm.app_id}
                  onChange={(e) => setAutoForm((f) => ({ ...f, app_id: e.target.value }))}
                >
                  <option value="">Store-wide (all apps)</option>
                  {apps.map((a) => (
                    <option key={a.id} value={a.id}>{a.project_title || a.bundle_id}</option>
                  ))}
                </select>
              </div>
            </div>

            <div className="form-row">
              <div className="form-group">
                <label htmlFor="ad-type">Discount type *</label>
                <select
                  id="ad-type"
                  value={autoForm.discount_type}
                  onChange={(e) => setAutoForm((f) => ({ ...f, discount_type: e.target.value as 'percent' | 'fixed' }))}
                >
                  <option value="percent">Percent off (%)</option>
                  <option value="fixed">Fixed amount off ({currency_symbol})</option>
                </select>
              </div>
              <div className="form-group">
                <label htmlFor="ad-value">
                  {autoForm.discount_type === 'percent' ? 'Percent (0–100) *' : 'Amount in cents *'}
                </label>
                <input
                  id="ad-value"
                  type="number"
                  min="1"
                  max={autoForm.discount_type === 'percent' ? 100 : undefined}
                  placeholder={autoForm.discount_type === 'percent' ? '20' : '500'}
                  value={autoForm.discount_value}
                  onChange={(e) => setAutoForm((f) => ({ ...f, discount_value: e.target.value }))}
                  required
                />
              </div>
            </div>

            <div className="form-row">
              <div className="form-group">
                <label htmlFor="ad-valid-from">Valid from (blank = immediately)</label>
                <input
                  id="ad-valid-from"
                  type="date"
                  value={autoForm.valid_from}
                  onChange={(e) => setAutoForm((f) => ({ ...f, valid_from: e.target.value }))}
                />
              </div>
              <div className="form-group">
                <label htmlFor="ad-expires">Expires on (blank = no expiry)</label>
                <input
                  id="ad-expires"
                  type="date"
                  value={autoForm.expires_at}
                  onChange={(e) => setAutoForm((f) => ({ ...f, expires_at: e.target.value }))}
                />
              </div>
            </div>

            {autoFormError && <p className="error">{autoFormError}</p>}
            <div className="form-actions">
              <button type="submit" className="btn btn-primary" disabled={autoSubmitting}>
                {autoSubmitting ? 'Creating…' : 'Create Sale'}
              </button>
            </div>
          </form>
        )}

        {autoDiscounts.length === 0 ? (
          <p className="admin-empty">No automatic discounts yet.</p>
        ) : (
          <table className="admin-table">
            <thead>
              <tr>
                <th>Label</th>
                <th>Discount</th>
                <th>Scope</th>
                <th>Orders</th>
                <th>Revenue</th>
                <th>Valid From</th>
                <th>Expires</th>
                <th>Status</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {autoDiscounts.map((d) => (
                <tr key={d.id} className={d.deleted_at || !d.active ? 'admin-row-inactive' : ''}>
                  <td>{d.label || <span className="admin-empty-cell">—</span>}</td>
                  <td>{formatDiscount(d.discount_type, d.discount_value)}</td>
                  <td>{formatScope(d.app_id)}</td>
                  <td>{d.order_count}</td>
                  <td>{formatPrice(d.revenue_cents, currency_symbol)}</td>
                  <td>{formatDate(d.valid_from)}</td>
                  <td>{formatDate(d.expires_at)}</td>
                  <td>
                    {d.deleted_at ? (
                      <span className="badge badge-danger">Archived</span>
                    ) : (
                      <span className={`discount-status discount-status-${d.active ? 'active' : 'inactive'}`}>
                        {d.active ? 'Active' : 'Inactive'}
                      </span>
                    )}
                  </td>
                  <td>
                    <div className="actions">
                      {!d.deleted_at && (
                        <button className="btn btn-secondary btn-small" onClick={() => handleToggleAuto(d)}>
                          {d.active ? 'Deactivate' : 'Activate'}
                        </button>
                      )}
                      {d.deleted_at ? (
                        <button className="btn btn-secondary btn-small" onClick={() => handleRestoreAuto(d.id)}>
                          Restore
                        </button>
                      ) : (
                        <button className="btn btn-danger btn-small" onClick={() => setArchiveTarget({ id: d.id, type: 'auto', label: d.label })}>
                          Archive
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
            {autoDiscounts.length > 0 && (
              <tfoot>
                <tr className="admin-sales-total-row">
                  <td colSpan={3}><strong>Total</strong></td>
                  <td><strong>{autoDiscounts.reduce((s, d) => s + d.order_count, 0)}</strong></td>
                  <td><strong>{formatPrice(autoDiscounts.reduce((s, d) => s + d.revenue_cents, 0), currency_symbol)}</strong></td>
                  <td colSpan={4} />
                </tr>
              </tfoot>
            )}
          </table>
        )}
      </div>}
    </div>
  );
}
