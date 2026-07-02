import { useState } from 'react';
import {
  adminLookupUser,
  adminRenameActivation,
  adminRevokeActivation,
  adminDeleteUserSessions,
  adminVoidOrder,
  type AdminUserData,
  type Activation,
  APIError,
} from '../../api/client';
import PageHeader from '../../components/PageHeader';
import ConfirmModal from '../../components/ConfirmModal';

export default function AdminUsers() {
  const [email, setEmail] = useState('');
  const [searching, setSearching] = useState(false);
  const [searchError, setSearchError] = useState('');
  const [userData, setUserData] = useState<AdminUserData | null>(null);

  // Rename activation inline state: activationId → new label
  const [renameValues, setRenameValues] = useState<Record<string, string>>({});
  const [renaming, setRenaming] = useState<Record<string, boolean>>({});
  const [actionError, setActionError] = useState('');
  const [sessionSuccess, setSessionSuccess] = useState('');
  const [confirmAction, setConfirmAction] = useState<{ title: string; message: string; onConfirm: () => void } | null>(null);

  async function handleSearch(e: React.FormEvent) {
    e.preventDefault();
    if (!email.trim()) return;
    setSearching(true);
    setSearchError('');
    setUserData(null);
    setActionError('');
    setSessionSuccess('');
    try {
      const data = await adminLookupUser(email.trim());
      setUserData(data);
    } catch (err) {
      setSearchError(err instanceof APIError ? err.message : 'Lookup failed');
    } finally {
      setSearching(false);
    }
  }

  async function handleRename(activationId: string) {
    const label = renameValues[activationId];
    if (!label?.trim()) return;
    setRenaming((prev) => ({ ...prev, [activationId]: true }));
    setActionError('');
    try {
      await adminRenameActivation(activationId, label.trim());
      if (userData) {
        setUserData({
          ...userData,
          activations: userData.activations.map((a) =>
            a.id === activationId ? { ...a, device_label: label.trim() } : a
          ),
        });
      }
      setRenameValues((prev) => ({ ...prev, [activationId]: '' }));
    } catch (err) {
      setActionError(err instanceof APIError ? err.message : 'Rename failed');
    } finally {
      setRenaming((prev) => ({ ...prev, [activationId]: false }));
    }
  }

  async function handleRevokeActivation(activation: Activation) {
    setActionError('');
    try {
      await adminRevokeActivation(activation.id);
      if (userData) {
        setUserData({
          ...userData,
          activations: userData.activations.filter((a) => a.id !== activation.id),
        });
      }
    } catch (err) {
      setActionError(err instanceof APIError ? err.message : 'Revoke failed');
    }
    setConfirmAction(null);
  }

  async function handleDeleteSessions() {
    if (!userData) return;
    setActionError('');
    try {
      const res = await adminDeleteUserSessions(userData.hash);
      setUserData({ ...userData, sessions: [] });
      setSessionSuccess(`${res.deleted} session(s) invalidated.`);
    } catch (err) {
      setActionError(err instanceof APIError ? err.message : 'Failed to delete sessions');
    }
    setConfirmAction(null);
  }

  async function handleVoidOrder(orderId: string) {
    setActionError('');
    try {
      await adminVoidOrder(orderId);
      if (userData) {
        const licenseIds = userData.licenses
          .filter((l) => l.order_id === orderId)
          .map((l) => l.id);
        setUserData({
          ...userData,
          orders: userData.orders.filter((o) => o.id !== orderId),
          licenses: userData.licenses.filter((l) => l.order_id !== orderId),
          activations: userData.activations.filter((a) => !licenseIds.includes(a.license_id)),
          download_tokens: userData.download_tokens,
        });
      }
    } catch (err) {
      setActionError(err instanceof APIError ? err.message : 'Void failed');
    }
    setConfirmAction(null);
  }

  return (
    <div className="page">
      <PageHeader title="User Lookup" />

      {confirmAction && (
        <ConfirmModal
          title={confirmAction.title}
          message={confirmAction.message}
          confirmLabel="Confirm"
          danger
          onConfirm={confirmAction.onConfirm}
          onCancel={() => setConfirmAction(null)}
        />
      )}

      <form onSubmit={handleSearch} className="admin-user-search">
        <div className="form-group">
          <label htmlFor="user-email">Email address</label>
          <input
            id="user-email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="user@example.com"
          />
        </div>
        <div className="form-actions">
          <button type="submit" className="btn btn-primary" disabled={searching}>
            {searching ? 'Searching…' : 'Look up user'}
          </button>
        </div>
        {searchError && <p className="error">{searchError}</p>}
      </form>

      {userData && (
        <div className="admin-user-data">
          {actionError && <p className="error">{actionError}</p>}

          <div className="admin-section">
            <div className="admin-user-section-header">
              <h2>Sessions ({userData.sessions.length})</h2>
              {userData.sessions.length > 0 && (
                <button className="btn btn-danger btn-small" onClick={() => setConfirmAction({
                  title: 'Invalidate Sessions',
                  message: 'Invalidate all sessions for this user? They will need to log in again.',
                  onConfirm: handleDeleteSessions,
                })}>
                  Invalidate all sessions
                </button>
              )}
            </div>
            {sessionSuccess && <p className="success-message">{sessionSuccess}</p>}
            {userData.sessions.length === 0 ? (
              <p className="admin-empty">No active sessions.</p>
            ) : (
              <table className="admin-table">
                <thead>
                  <tr>
                    <th>Session ID</th>
                    <th>Created</th>
                    <th>Expires</th>
                  </tr>
                </thead>
                <tbody>
                  {userData.sessions.map((s) => (
                    <tr key={s.id}>
                      <td><code className="app-id">{s.id.slice(0, 16)}…</code></td>
                      <td>{new Date(s.created_at).toLocaleString()}</td>
                      <td>{new Date(s.expires_at).toLocaleString()}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>

          <div className="admin-section">
            <h2>Orders ({userData.orders.length})</h2>
            {userData.orders.length === 0 ? (
              <p className="admin-empty">No orders.</p>
            ) : (
              <table className="admin-table">
                <thead>
                  <tr>
                    <th>App</th>
                    <th>Order ID</th>
                    <th>Purchased</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {userData.orders.map((o) => (
                    <tr key={o.id}>
                      <td>{o.app_name}</td>
                      <td><code className="app-id">{o.id.slice(0, 16)}…</code></td>
                      <td>{new Date(o.created_at).toLocaleString()}</td>
                      <td>
                        <button
                          className="btn btn-danger btn-small"
                          onClick={() => setConfirmAction({
                            title: 'Void Order',
                            message: 'Permanently void this order and all its licenses/activations?',
                            onConfirm: () => handleVoidOrder(o.id),
                          })}
                        >
                          Void order
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>

          <div className="admin-section">
            <h2>Licenses ({userData.licenses.length})</h2>
            {userData.licenses.length === 0 ? (
              <p className="admin-empty">No licenses.</p>
            ) : (
              <table className="admin-table">
                <thead>
                  <tr>
                    <th>License key</th>
                    <th>App ID</th>
                    <th>Issued</th>
                  </tr>
                </thead>
                <tbody>
                  {userData.licenses.map((l) => (
                    <tr key={l.id}>
                      <td><code className="app-id">{l.key}</code></td>
                      <td><code className="app-id">{l.app_id.slice(0, 12)}…</code></td>
                      <td>{new Date(l.created_at).toLocaleString()}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>

          <div className="admin-section">
            <h2>Activations ({userData.activations.length})</h2>
            {userData.activations.length === 0 ? (
              <p className="admin-empty">No activations.</p>
            ) : (
              <table className="admin-table">
                <thead>
                  <tr>
                    <th>Device</th>
                    <th>Activated</th>
                    <th>Last seen</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {userData.activations.map((a) => (
                    <tr key={a.id}>
                      <td>
                        <div className="admin-activation-cell">
                          <span>{a.device_label ?? <em>unnamed</em>}</span>
                          <input
                            type="text"
                            className="admin-rename-input"
                            placeholder="New label"
                            value={renameValues[a.id] ?? ''}
                            onChange={(e) =>
                              setRenameValues((prev) => ({ ...prev, [a.id]: e.target.value }))
                            }
                          />
                        </div>
                      </td>
                      <td>{new Date(a.activated_at).toLocaleString()}</td>
                      <td>{a.last_seen_at ? new Date(a.last_seen_at).toLocaleString() : '—'}</td>
                      <td>
                        <div className="actions">
                          <button
                            className="btn btn-secondary btn-small"
                            disabled={!renameValues[a.id]?.trim() || renaming[a.id]}
                            onClick={() => handleRename(a.id)}
                          >
                            {renaming[a.id] ? 'Saving…' : 'Rename'}
                          </button>
                          <button
                            className="btn btn-danger btn-small"
                            onClick={() => setConfirmAction({
                              title: 'Revoke Activation',
                              message: `Revoke activation "${a.device_label ?? a.id}"?`,
                              onConfirm: () => handleRevokeActivation(a),
                            })}
                          >
                            Revoke
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
