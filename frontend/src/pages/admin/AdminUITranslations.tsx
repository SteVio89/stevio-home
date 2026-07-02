import { useEffect, useState } from 'react';
import PageHeader from '../../components/PageHeader';
import { useToast } from '../../context/ToastContext';
import { useAdminLocales } from '../../hooks/useAdminLocales';
import defaultKeys from '../../i18n/locales/de.json';

const groupedKeys: { group: string; keys: { key: string; defaultValue: string }[] }[] = (() => {
  const groups = new Map<string, { key: string; defaultValue: string }[]>();
  for (const [key, value] of Object.entries(defaultKeys)) {
    const group = key.split('.')[0];
    if (!groups.has(group)) groups.set(group, []);
    groups.get(group)!.push({ key, defaultValue: value });
  }
  return Array.from(groups.entries()).map(([group, keys]) => ({ group, keys }));
})();

export default function AdminUITranslations() {
  const { locales, loading: localesLoading } = useAdminLocales();
  const { addToast } = useToast();
  const [activeLocale, setActiveLocale] = useState('');
  const [translations, setTranslations] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(false);
  const [filter, setFilter] = useState('');
  const [editKey, setEditKey] = useState('');
  const [editValue, setEditValue] = useState('');
  const [saving, setSaving] = useState(false);

  // Set initial locale when loaded
  useEffect(() => {
    if (locales.length > 0 && !activeLocale) {
      setActiveLocale(locales[0].code);
    }
  }, [locales, activeLocale]);

  // Load translations for active locale
  useEffect(() => {
    if (!activeLocale) return;
    setLoading(true);
    fetch(`/api/admin/i18n/${activeLocale}`, { credentials: 'include' })
      .then((res) => res.json())
      .then(setTranslations)
      .catch(() => addToast('Failed to load translations', 'error'))
      .finally(() => setLoading(false));
  }, [activeLocale, addToast]);

  async function handleSave(ev: React.FormEvent) {
    ev.preventDefault();
    if (!editKey) return;
    setSaving(true);
    try {
      const res = await fetch(`/api/admin/i18n/${activeLocale}`, {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: editKey, value: editValue }),
      });
      if (!res.ok) throw new Error('Failed to save');
      setTranslations((prev) => ({ ...prev, [editKey]: editValue }));
      addToast(`Key "${editKey}" saved.`, 'success');
      setEditKey('');
      setEditValue('');
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Failed to save', 'error');
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete(key: string) {
    try {
      const res = await fetch(`/api/admin/i18n/${activeLocale}/${encodeURIComponent(key)}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      if (!res.ok) throw new Error('Failed to delete');
      setTranslations((prev) => {
        const next = { ...prev };
        delete next[key];
        return next;
      });
      addToast(`Override for "${key}" removed.`, 'success');
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Failed to delete', 'error');
    }
  }

  if (localesLoading) return <div className="page"><div className="skeleton skeleton-text" style={{ width: '50%' }} /><div className="skeleton skeleton-text" style={{ width: '70%' }} /></div>;

  const entries = Object.entries(translations)
    .filter(([k]) => !filter || k.toLowerCase().includes(filter.toLowerCase()))
    .sort(([a], [b]) => a.localeCompare(b));

  return (
    <div className="page">
      <PageHeader title="UI Translations" />

      <div className="admin-locale-tabs">
        {locales.map((loc) => (
          <button
            key={loc.code}
            className={`btn btn-small${activeLocale === loc.code ? ' btn-primary' : ' btn-secondary'}`}
            onClick={() => setActiveLocale(loc.code)}
          >
            {loc.name}
          </button>
        ))}
      </div>

      <div className="admin-section">
        <div className="admin-section-header">
          <h2>Add / Edit Override</h2>
        </div>
        <p className="admin-section-description">
          Override bundled UI strings for this locale. Only overridden keys are stored — delete to revert to the bundled default.
        </p>
        <form onSubmit={handleSave} className="admin-form admin-form-row">
          <div className="form-group">
            <label htmlFor="tr-key">Key</label>
            <select
              id="tr-key"
              value={editKey}
              onChange={(e) => {
                const k = e.target.value;
                setEditKey(k);
                if (k && translations[k] !== undefined) {
                  setEditValue(translations[k]);
                } else {
                  setEditValue('');
                }
              }}
              required
            >
              <option value="">— Select a key —</option>
              {groupedKeys.map(({ group, keys }) => (
                <optgroup key={group} label={group}>
                  {keys.map(({ key, defaultValue }) => (
                    <option key={key} value={key}>
                      {key}{translations[key] !== undefined ? ' \u2726' : ''} — {defaultValue.length > 60 ? defaultValue.slice(0, 60) + '\u2026' : defaultValue}
                    </option>
                  ))}
                </optgroup>
              ))}
            </select>
          </div>
          <div className="form-group" style={{ flex: 2 }}>
            <label htmlFor="tr-value">Value</label>
            <input
              id="tr-value"
              type="text"
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              maxLength={5000}
              required
            />
          </div>
          <div className="form-actions">
            <button type="submit" className="btn btn-primary" disabled={saving}>
              {saving ? 'Saving…' : 'Save'}
            </button>
          </div>
        </form>
      </div>

      <div className="admin-section">
        <div className="admin-section-header">
          <h2>Current Overrides ({entries.length})</h2>
        </div>
        <div className="form-group">
          <input
            type="text"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Filter keys…"
          />
        </div>
        {loading ? (
          <div className="skeleton skeleton-text" style={{ width: '80%' }} />
        ) : entries.length === 0 ? (
          <p>No overrides for this locale.</p>
        ) : (
          <table className="admin-table">
            <thead>
              <tr>
                <th>Key</th>
                <th>Value</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {entries.map(([key, value]) => (
                <tr key={key}>
                  <td><code>{key}</code></td>
                  <td>{value.length > 80 ? value.slice(0, 80) + '…' : value}</td>
                  <td>
                    <button
                      className="btn btn-small btn-secondary"
                      onClick={() => { setEditKey(key); setEditValue(value); }}
                    >
                      Edit
                    </button>
                    <button
                      className="btn btn-small btn-danger"
                      onClick={() => handleDelete(key)}
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
