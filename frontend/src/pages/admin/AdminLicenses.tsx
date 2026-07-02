import { useEffect, useState } from 'react';
import { adminListLicenses, adminListApps, adminIssueLicense, adminUnrevokeLicense, type AdminLicense, type AdminAppListItem } from '../../api/client';
import PageHeader from '../../components/PageHeader';
import { SkeletonTable } from '../../components/Skeleton';
import { useSiteConfig } from '../../context/SiteConfigContext';
import { useToast } from '../../context/ToastContext';

const PER_PAGE = 20;

export default function AdminLicenses() {
    const [licenses, setLicenses] = useState<AdminLicense[]>([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(1);
    const [apps, setApps] = useState<AdminAppListItem[]>([]);
    const [appFilter, setAppFilter] = useState('');
    const [keyPrefix, setKeyPrefix] = useState('');
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState('');
    const { currency_symbol } = useSiteConfig();
    const { addToast } = useToast();

    // Issue modal state
    const [showIssueModal, setShowIssueModal] = useState(false);
    const [issueEmail, setIssueEmail] = useState('');
    const [issueAppId, setIssueAppId] = useState('');
    const [issuePrice, setIssuePrice] = useState('');
    const [issuing, setIssuing] = useState(false);

    useEffect(() => {
        adminListApps().then(setApps).catch(() => {});
    }, []);

    useEffect(() => {
        setLoading(true);
        const params: Record<string, string> = {
            page: String(page),
            per_page: String(PER_PAGE),
        };
        if (appFilter) params.app_id = appFilter;
        if (keyPrefix) params.key_prefix = keyPrefix;
        adminListLicenses(params)
            .then((res) => { setLicenses(res.licenses); setTotal(res.total); })
            .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load'))
            .finally(() => setLoading(false));
    }, [page, appFilter, keyPrefix]);

    async function handleUnrevoke(id: string) {
        try {
            await adminUnrevokeLicense(id);
            setLicenses(licenses.map(l => l.id === id ? { ...l, revoked: false } : l));
            addToast('License unrevoked', 'success');
        } catch {
            addToast('Failed to unrevoke', 'error');
        }
    }

    async function handleIssue(ev: React.FormEvent) {
        ev.preventDefault();
        setIssuing(true);
        try {
            const result = await adminIssueLicense({
                email: issueEmail,
                app_id: issueAppId,
                price_cents: Math.round(parseFloat(issuePrice) * 100),
            });
            addToast(`License issued: ${result.license_key}`, 'success');
            setShowIssueModal(false);
            setIssueEmail('');
            setIssueAppId('');
            setIssuePrice('');
            // Refresh list
            setPage(1);
        } catch {
            addToast('Failed to issue license', 'error');
        } finally {
            setIssuing(false);
        }
    }

    const totalPages = Math.ceil(total / PER_PAGE);

    return (
        <div className="page">
            <PageHeader
                title="Licenses"
                actions={
                    <button className="btn btn-primary btn-small" onClick={() => setShowIssueModal(true)}>Issue License</button>
                }
            />
            {error && <p className="error">{error}</p>}

            {showIssueModal && (
                <div className="admin-modal-backdrop" onClick={() => setShowIssueModal(false)}>
                    <div className="admin-modal" onClick={(e) => e.stopPropagation()}>
                        <h3>Issue License</h3>
                        <form onSubmit={handleIssue}>
                            <div className="form-group">
                                <label htmlFor="issue-email">Email</label>
                                <input id="issue-email" type="email" required value={issueEmail} onChange={(e) => setIssueEmail(e.target.value)} />
                            </div>
                            <div className="form-group">
                                <label htmlFor="issue-app">App</label>
                                <select id="issue-app" required value={issueAppId} onChange={(e) => setIssueAppId(e.target.value)}>
                                    <option value="">Select app</option>
                                    {apps.filter(a => !a.deleted_at).map(a => <option key={a.id} value={a.id}>{a.project_title || a.bundle_id}</option>)}
                                </select>
                            </div>
                            <div className="form-group">
                                <label htmlFor="issue-price">Price ({currency_symbol})</label>
                                <input id="issue-price" type="number" step="0.01" min="0" required value={issuePrice} onChange={(e) => setIssuePrice(e.target.value)} />
                            </div>
                            <div className="form-actions">
                                <button type="button" className="btn btn-secondary" onClick={() => setShowIssueModal(false)}>Cancel</button>
                                <button type="submit" className="btn btn-primary" disabled={issuing}>{issuing ? 'Issuing...' : 'Issue'}</button>
                            </div>
                        </form>
                    </div>
                </div>
            )}

            <div className="admin-section">
                <div className="admin-filters">
                    <select value={appFilter} onChange={(e) => { setAppFilter(e.target.value); setPage(1); }}>
                        <option value="">All Apps</option>
                        {apps.map(a => <option key={a.id} value={a.id}>{a.project_title || a.bundle_id}</option>)}
                    </select>
                    <input type="text" placeholder="Key prefix..." value={keyPrefix} onChange={(e) => { setKeyPrefix(e.target.value); setPage(1); }} />
                </div>
                {loading ? <SkeletonTable rows={5} cols={6} /> : licenses.length === 0 ? (
                    <p className="admin-empty">No licenses found.</p>
                ) : (
                    <>
                        <table className="admin-table">
                            <thead>
                                <tr>
                                    <th>Key</th>
                                    <th>App</th>
                                    <th>Status</th>
                                    <th>Activations</th>
                                    <th>Created</th>
                                    <th>Actions</th>
                                </tr>
                            </thead>
                            <tbody>
                                {licenses.map(l => (
                                    <tr key={l.id} className={l.revoked ? 'admin-row-inactive' : ''}>
                                        <td><code>{l.key.slice(0, 12)}...</code></td>
                                        <td>{l.app_name || '\u2014'}</td>
                                        <td>{l.revoked ? <span className="badge badge-danger">Revoked</span> : <span className="badge badge-success">Active</span>}</td>
                                        <td>{l.activation_count}{l.max_activations ? ` / ${l.max_activations}` : ''}</td>
                                        <td>{new Date(l.created_at).toLocaleDateString()}</td>
                                        <td>
                                            {l.revoked && (
                                                <button className="btn btn-secondary btn-small" onClick={() => handleUnrevoke(l.id)}>Unrevoke</button>
                                            )}
                                        </td>
                                    </tr>
                                ))}
                            </tbody>
                        </table>
                        <div className="admin-pagination">
                            <span className="admin-pagination-info">{total} license{total !== 1 ? 's' : ''}</span>
                            <div className="admin-pagination-controls">
                                <button className="btn btn-secondary btn-small" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>Previous</button>
                                <span>Page {page} of {totalPages}</span>
                                <button className="btn btn-secondary btn-small" disabled={page >= totalPages} onClick={() => setPage(p => p + 1)}>Next</button>
                            </div>
                        </div>
                    </>
                )}
            </div>
        </div>
    );
}
