import { useEffect, useState } from 'react';
import { adminGetPageTranslation, adminUpsertPageTranslation, APIError } from '../../api/client';
import LocaleTabs from '../../components/LocaleTabs';
import PageHeader from '../../components/PageHeader';
import { useToast } from '../../context/ToastContext';
import { useAdminLocales } from '../../hooks/useAdminLocales';

export default function AdminHero() {
  const { locales, loading: localesLoading } = useAdminLocales();
  const { addToast } = useToast();
  const [activeLocale, setActiveLocale] = useState<string>('');
  const [headline, setHeadline] = useState<Record<string, string>>({});
  const [tagline, setTagline] = useState<Record<string, string>>({});
  const [bio, setBio] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  // Set initial locale once loaded
  useEffect(() => {
    if (locales.length > 0 && !activeLocale) {
      setActiveLocale(locales[0].code);
    }
  }, [locales, activeLocale]);

  useEffect(() => {
    Promise.all(
      locales.map((loc) => adminGetPageTranslation('hero', loc.code).then((fields) => ({ code: loc.code, fields })))
    )
      .then((results) => {
        const h: Record<string, string> = {};
        const t: Record<string, string> = {};
        const b: Record<string, string> = {};
        for (const { code, fields } of results) {
          h[code] = fields.headline || '';
          t[code] = fields.tagline || '';
          b[code] = fields.bio || '';
        }
        setHeadline(h);
        setTagline(t);
        setBio(b);
      })
      .catch((err) => addToast(err instanceof APIError ? err.message : 'Failed to load hero content', 'error'))
      .finally(() => setLoading(false));
  }, [addToast, locales]);

  async function handleSave(ev: React.FormEvent) {
    ev.preventDefault();
    setSaving(true);
    try {
      await adminUpsertPageTranslation('hero', activeLocale, {
        headline: headline[activeLocale] ?? '',
        tagline: tagline[activeLocale] ?? '',
        bio: bio[activeLocale] ?? '',
      });
      addToast('Hero content updated', 'success');
    } catch (err) {
      addToast(err instanceof APIError ? err.message : 'Failed to save hero content', 'error');
    } finally {
      setSaving(false);
    }
  }

  if (loading || localesLoading) {
    return (
      <div className="page">
        <div className="skeleton skeleton-text" style={{ width: '60%' }} />
        <div className="skeleton skeleton-text" style={{ width: '80%' }} />
      </div>
    );
  }

  return (
    <div className="page">
      <PageHeader title="Hero Content" />
      <LocaleTabs locales={locales} activeLocale={activeLocale} onChange={setActiveLocale} />
      <form onSubmit={handleSave} className="admin-form">
        <div className="form-group">
          <label htmlFor="hero-headline">Headline</label>
          <input
            id="hero-headline"
            type="text"
            maxLength={2000}
            placeholder="Your headline"
            value={headline[activeLocale] ?? ''}
            onChange={(e) => setHeadline(prev => ({ ...prev, [activeLocale]: e.target.value }))}
          />
        </div>
        <div className="form-group">
          <label htmlFor="hero-tagline">Tagline</label>
          <input
            id="hero-tagline"
            type="text"
            maxLength={2000}
            placeholder="Short tagline"
            value={tagline[activeLocale] ?? ''}
            onChange={(e) => setTagline(prev => ({ ...prev, [activeLocale]: e.target.value }))}
          />
        </div>
        <div className="form-group">
          <label htmlFor="hero-bio">Bio</label>
          <textarea
            id="hero-bio"
            rows={6}
            maxLength={2000}
            placeholder="Your bio"
            value={bio[activeLocale] ?? ''}
            onChange={(e) => setBio(prev => ({ ...prev, [activeLocale]: e.target.value }))}
          />
        </div>
        <div className="form-actions">
          <button type="submit" className="btn btn-primary" disabled={saving}>
            {saving ? 'Saving\u2026' : 'Save Hero'}
          </button>
        </div>
      </form>
    </div>
  );
}
