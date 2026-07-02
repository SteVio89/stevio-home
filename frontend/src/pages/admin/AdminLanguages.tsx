import { useState } from 'react';
import PageHeader from '../../components/PageHeader';
import { useToast } from '../../context/ToastContext';
import { useAdminLocales } from '../../hooks/useAdminLocales';
import { adminCreateLocale, adminUpdateLocale } from '../../api/client';

export default function AdminLanguages() {
  const { locales, loading, reload } = useAdminLocales();
  const { addToast } = useToast();
  const [code, setCode] = useState('');
  const [name, setName] = useState('');
  const [sortOrder, setSortOrder] = useState(0);
  const [creating, setCreating] = useState(false);

  async function handleCreate(ev: React.FormEvent) {
    ev.preventDefault();
    setCreating(true);
    try {
      await adminCreateLocale({ code, name, sort_order: sortOrder });
      addToast(`Locale "${code}" created.`, 'success');
      setCode('');
      setName('');
      setSortOrder(0);
      reload();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Failed to create locale', 'error');
    } finally {
      setCreating(false);
    }
  }

  async function handleToggle(localeCode: string, enabled: boolean) {
    try {
      await adminUpdateLocale(localeCode, { enabled });
      reload();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Failed to update locale', 'error');
    }
  }

  async function handleSetDefault(localeCode: string) {
    try {
      await adminUpdateLocale(localeCode, { is_default: true });
      addToast(`"${localeCode}" is now the default locale.`, 'success');
      reload();
    } catch (err) {
      addToast(err instanceof Error ? err.message : 'Failed to set default', 'error');
    }
  }

  if (loading) return <div className="page"><div className="skeleton skeleton-text" style={{ width: '60%' }} /><div className="skeleton skeleton-text" style={{ width: '40%' }} /><div className="skeleton skeleton-text" style={{ width: '50%' }} /></div>;

  return (
    <div className="page">
      <PageHeader title="Languages" />

      <div className="admin-section">
        <div className="admin-section-header">
          <h2>Available Languages</h2>
        </div>
        <table className="admin-table">
          <thead>
            <tr>
              <th>Code</th>
              <th>Name</th>
              <th>Default</th>
              <th>Enabled</th>
              <th>Sort Order</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {locales.map((loc) => (
              <tr key={loc.code}>
                <td><code>{loc.code}</code></td>
                <td>{loc.name}</td>
                <td>{loc.is_default ? 'Yes' : ''}</td>
                <td>
                  <input
                    type="checkbox"
                    checked={loc.enabled}
                    onChange={(e) => handleToggle(loc.code, e.target.checked)}
                    disabled={loc.is_default && loc.enabled}
                  />
                </td>
                <td>{loc.sort_order}</td>
                <td>
                  {!loc.is_default && loc.enabled && (
                    <button className="btn btn-small btn-secondary" onClick={() => handleSetDefault(loc.code)}>
                      Set Default
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="admin-section">
        <div className="admin-section-header">
          <h2>Add Language</h2>
        </div>
        <form onSubmit={handleCreate} className="admin-form">
          <div className="form-group">
            <label htmlFor="locale-code">Code (e.g. "fr", "es")</label>
            <input
              id="locale-code"
              type="text"
              value={code}
              onChange={(e) => setCode(e.target.value.toLowerCase())}
              pattern="[a-z]{2,5}"
              maxLength={5}
              required
              placeholder="fr"
            />
          </div>
          <div className="form-group">
            <label htmlFor="locale-name">Display Name</label>
            <input
              id="locale-name"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              maxLength={100}
              required
              placeholder="Français"
            />
          </div>
          <div className="form-group">
            <label htmlFor="locale-sort">Sort Order</label>
            <input
              id="locale-sort"
              type="number"
              value={sortOrder}
              onChange={(e) => setSortOrder(parseInt(e.target.value, 10) || 0)}
              min={0}
              max={999}
            />
          </div>
          <div className="form-actions">
            <button type="submit" className="btn btn-primary" disabled={creating}>
              {creating ? 'Creating…' : 'Add Language'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
