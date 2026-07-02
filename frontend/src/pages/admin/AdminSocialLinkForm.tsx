import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import {
  adminCreateSocialLink,
  adminUpdateSocialLink,
  adminListSocialLinks,
  APIError,
} from '../../api/client';
import PageHeader from '../../components/PageHeader';
import Skeleton from '../../components/Skeleton';
import { useToast } from '../../context/ToastContext';

const PLATFORMS = [
  { value: 'website', label: 'Website' },
  { value: 'email', label: 'Email' },
  { value: 'github', label: 'GitHub' },
  { value: 'gitlab', label: 'GitLab' },
  { value: 'codeberg', label: 'Codeberg' },
  { value: 'linkedin', label: 'LinkedIn' },
  { value: 'xing', label: 'XING' },
  { value: 'youtube', label: 'YouTube' },
  { value: 'twitch', label: 'Twitch' },
  { value: 'steam', label: 'Steam' },
  { value: 'playstation', label: 'PlayStation' },
];

export default function AdminSocialLinkForm() {
  const { id } = useParams<{ id: string }>();
  const isEdit = Boolean(id);
  const navigate = useNavigate();
  const { addToast } = useToast();

  const [platform, setPlatform] = useState('website');
  const [url, setUrl] = useState('');
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!isEdit || !id) return;
    setLoading(true);
    adminListSocialLinks()
      .then((links) => {
        const found = links.find(l => l.id === id);
        if (!found) {
          addToast('Social link not found', 'error');
          navigate('/admin/social-links');
          return;
        }
        setPlatform(found.platform);
        setUrl(found.url);
      })
      .catch(() => {
        addToast('Failed to load social link', 'error');
        navigate('/admin/social-links');
      })
      .finally(() => setLoading(false));
  }, [id, isEdit, addToast, navigate]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (platform !== 'email' && !url.startsWith('https://')) {
      addToast('URL must start with https://', 'error');
      return;
    }
    setSaving(true);
    try {
      if (isEdit && id) {
        await adminUpdateSocialLink(id, { platform, url });
        addToast('Link updated', 'success');
      } else {
        await adminCreateSocialLink({ platform, url });
        addToast('Link created', 'success');
      }
      navigate('/admin/social-links');
    } catch (err) {
      addToast(err instanceof APIError ? err.message : 'Save failed', 'error');
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <div className="page">
        <PageHeader
          title={isEdit ? 'Edit Social Link' : 'New Social Link'}
          backTo="/admin/social-links"
          backLabel="Social Links"
        />
        <Skeleton variant="text" count={3} />
      </div>
    );
  }

  return (
    <div className="page">
      <PageHeader
        title={isEdit ? 'Edit Social Link' : 'New Social Link'}
        backTo="/admin/social-links"
        backLabel="Social Links"
      />
      <form onSubmit={handleSubmit} className="admin-form">
        <div className="form-group">
          <label htmlFor="platform">Platform</label>
          <select
            id="platform"
            value={platform}
            onChange={(e) => setPlatform(e.target.value)}
          >
            {PLATFORMS.map(p => (
              <option key={p.value} value={p.value}>{p.label}</option>
            ))}
          </select>
        </div>
        <div className="form-group">
          <label htmlFor="link-url">URL</label>
          <input
            id="link-url"
            type="url"
            placeholder={platform === 'email' ? 'mailto:you@example.com' : 'https://'}
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            required
          />
        </div>
        <div className="form-actions">
          <button type="submit" className="btn btn-primary" disabled={saving}>
            {saving ? 'Saving\u2026' : 'Save Link'}
          </button>
          <Link to="/admin/social-links" className="btn btn-secondary">Cancel</Link>
        </div>
      </form>
    </div>
  );
}
