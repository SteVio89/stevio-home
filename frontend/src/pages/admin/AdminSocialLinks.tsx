import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  adminListSocialLinks,
  adminDeleteSocialLink,
  adminReorderSocialLinks,
  type AdminSocialLink,
} from '../../api/client';
import ConfirmModal from '../../components/ConfirmModal';
import PageHeader from '../../components/PageHeader';
import { SkeletonTable } from '../../components/Skeleton';
import { useToast } from '../../context/ToastContext';

const PLATFORM_DISPLAY: Record<string, string> = {
  github: 'GitHub',
  linkedin: 'LinkedIn',
  steam: 'Steam',
  twitch: 'Twitch',
  xing: 'XING',
  playstation: 'PlayStation',
  youtube: 'YouTube',
  gitlab: 'GitLab',
  codeberg: 'Codeberg',
  website: 'Website',
  email: 'Email',
};

export default function AdminSocialLinks() {
  const [links, setLinks] = useState<AdminSocialLink[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<AdminSocialLink | null>(null);
  const { addToast } = useToast();

  useEffect(() => {
    adminListSocialLinks()
      .then(setLinks)
      .catch(() => setError('Failed to load social links'))
      .finally(() => setLoading(false));
  }, []);

  async function handleDelete(link: AdminSocialLink) {
    try {
      await adminDeleteSocialLink(link.id);
      setLinks(prev => prev.filter(l => l.id !== link.id));
      addToast('Link deleted', 'success');
    } catch {
      addToast('Failed to delete link', 'error');
    }
    setDeleteTarget(null);
  }

  async function moveItem(index: number, direction: -1 | 1) {
    const target = index + direction;
    if (target < 0 || target >= links.length) return;
    const newLinks = [...links];
    [newLinks[index], newLinks[target]] = [newLinks[target], newLinks[index]];

    const positions: Record<string, number> = {
      [links[index].id]: target,
      [links[target].id]: index,
    };

    const oldLinks = links;
    setLinks(newLinks);
    try {
      await adminReorderSocialLinks(positions);
    } catch {
      setLinks(oldLinks);
      addToast('Failed to reorder', 'error');
    }
  }

  if (loading) {
    return (
      <div className="page">
        <PageHeader
          title="Social Links"
          actions={<Link to="/admin/social-links/new" className="btn btn-primary btn-small">+ New Link</Link>}
        />
        <SkeletonTable rows={3} cols={4} />
      </div>
    );
  }

  if (error) {
    return (
      <div className="page">
        <PageHeader
          title="Social Links"
          actions={<Link to="/admin/social-links/new" className="btn btn-primary btn-small">+ New Link</Link>}
        />
        <p className="error-message">{error}</p>
      </div>
    );
  }

  return (
    <div className="page">
      <PageHeader
        title="Social Links"
        actions={<Link to="/admin/social-links/new" className="btn btn-primary btn-small">+ New Link</Link>}
      />

      {links.length === 0 ? (
        <div className="admin-empty-state">
          <p>No social links yet</p>
          <p>Add links to your profiles and contact info.</p>
          <Link to="/admin/social-links/new" className="btn btn-primary">+ New Link</Link>
        </div>
      ) : (
        <table className="admin-table">
          <thead>
            <tr>
              <th>Platform</th>
              <th>URL</th>
              <th>Position</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {links.map((link, i) => (
              <tr key={link.id}>
                <td>{PLATFORM_DISPLAY[link.platform] ?? (link.platform.charAt(0).toUpperCase() + link.platform.slice(1))}</td>
                <td>{link.url.length > 48 ? link.url.slice(0, 48) + '\u2026' : link.url}</td>
                <td>
                  <div className="order-actions">
                    <button
                      className="btn btn-secondary btn-small order-btn"
                      onClick={() => moveItem(i, -1)}
                      disabled={i === 0}
                      aria-label="Move up"
                      style={i === 0 ? { opacity: 0.4 } : undefined}
                    >
                      ↑
                    </button>
                    <button
                      className="btn btn-secondary btn-small order-btn"
                      onClick={() => moveItem(i, 1)}
                      disabled={i === links.length - 1}
                      aria-label="Move down"
                      style={i === links.length - 1 ? { opacity: 0.4 } : undefined}
                    >
                      ↓
                    </button>
                  </div>
                </td>
                <td>
                  <Link to={`/admin/social-links/${link.id}/edit`} className="btn btn-secondary btn-small">
                    Edit
                  </Link>{' '}
                  <button
                    className="btn btn-danger btn-small"
                    onClick={() => setDeleteTarget(link)}
                  >
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {deleteTarget && (
        <ConfirmModal
          title="Delete Link"
          message={`Are you sure you want to delete this ${PLATFORM_DISPLAY[deleteTarget.platform] ?? deleteTarget.platform} link? This cannot be undone.`}
          confirmLabel="Delete Link"
          danger
          onConfirm={() => handleDelete(deleteTarget)}
          onCancel={() => setDeleteTarget(null)}
        />
      )}
    </div>
  );
}
