import { useEffect, useState } from 'react';
import { adminGetPageTranslation, adminUpsertPageTranslation, APIError } from '../../api/client';
import PageHeader from '../../components/PageHeader';
import { useToast } from '../../context/ToastContext';
import { useAdminLocales } from '../../hooks/useAdminLocales';

export default function AdminLegalPages() {
  const { addToast } = useToast();
  const { locales, loading: localesLoading } = useAdminLocales();
  const [activeLocale, setActiveLocale] = useState<string>('');
  const [impressum, setImpressum] = useState<Record<string, string>>({});
  const [privacy, setPrivacy] = useState<Record<string, string>>({});
  const [refund, setRefund] = useState<Record<string, string>>({});
  const [terms, setTerms] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [savingImpressum, setSavingImpressum] = useState(false);
  const [savingPrivacy, setSavingPrivacy] = useState(false);
  const [savingRefund, setSavingRefund] = useState(false);
  const [savingTerms, setSavingTerms] = useState(false);

  // Set initial locale once loaded
  useEffect(() => {
    if (locales.length > 0 && !activeLocale) {
      setActiveLocale(locales[0].code);
    }
  }, [locales, activeLocale]);

  useEffect(() => {
    if (locales.length === 0) return;
    const pageKeys = ['impressum', 'privacy_policy', 'refund_policy', 'terms_of_use'] as const;
    Promise.all(
      locales.flatMap((loc) =>
        pageKeys.map((key) =>
          adminGetPageTranslation(key, loc.code).then((fields) => ({ code: loc.code, key, content: fields.content || '' }))
        )
      )
    )
      .then((results) => {
        const imp: Record<string, string> = {};
        const priv: Record<string, string> = {};
        const ref: Record<string, string> = {};
        const trm: Record<string, string> = {};
        for (const { code, key, content } of results) {
          if (key === 'impressum') imp[code] = content;
          else if (key === 'privacy_policy') priv[code] = content;
          else if (key === 'refund_policy') ref[code] = content;
          else if (key === 'terms_of_use') trm[code] = content;
        }
        setImpressum(imp);
        setPrivacy(priv);
        setRefund(ref);
        setTerms(trm);
      })
      .catch((err) => addToast(err instanceof APIError ? err.message : 'Failed to load legal content', 'error'))
      .finally(() => setLoading(false));
  }, [addToast, locales]);

  async function handleSaveImpressum(ev: React.FormEvent) {
    ev.preventDefault();
    setSavingImpressum(true);
    try {
      await adminUpsertPageTranslation('impressum', activeLocale, { content: impressum[activeLocale] ?? '' });
      const label = locales.find(l => l.code === activeLocale)?.name ?? activeLocale;
      addToast(`Impressum (${label}) saved.`, 'success');
    } catch (err) {
      addToast(err instanceof APIError ? err.message : 'Failed to save Impressum', 'error');
    } finally {
      setSavingImpressum(false);
    }
  }

  async function handleSavePrivacy(ev: React.FormEvent) {
    ev.preventDefault();
    setSavingPrivacy(true);
    try {
      await adminUpsertPageTranslation('privacy_policy', activeLocale, { content: privacy[activeLocale] ?? '' });
      const label = locales.find(l => l.code === activeLocale)?.name ?? activeLocale;
      addToast(`Privacy policy (${label}) saved.`, 'success');
    } catch (err) {
      addToast(err instanceof APIError ? err.message : 'Failed to save privacy policy', 'error');
    } finally {
      setSavingPrivacy(false);
    }
  }

  async function handleSaveRefund(ev: React.FormEvent) {
    ev.preventDefault();
    setSavingRefund(true);
    try {
      await adminUpsertPageTranslation('refund_policy', activeLocale, { content: refund[activeLocale] ?? '' });
      const label = locales.find(l => l.code === activeLocale)?.name ?? activeLocale;
      addToast(`Widerrufsbelehrung (${label}) saved.`, 'success');
    } catch (err) {
      addToast(err instanceof APIError ? err.message : 'Failed to save Widerrufsbelehrung', 'error');
    } finally {
      setSavingRefund(false);
    }
  }

  async function handleSaveTerms(ev: React.FormEvent) {
    ev.preventDefault();
    setSavingTerms(true);
    try {
      await adminUpsertPageTranslation('terms_of_use', activeLocale, { content: terms[activeLocale] ?? '' });
      const label = locales.find(l => l.code === activeLocale)?.name ?? activeLocale;
      addToast(`Terms of Use (${label}) saved.`, 'success');
    } catch (err) {
      addToast(err instanceof APIError ? err.message : 'Failed to save Terms of Use', 'error');
    } finally {
      setSavingTerms(false);
    }
  }

  if (loading || localesLoading) return <div className="page"><div className="skeleton skeleton-text" style={{ width: '60%' }} /><div className="skeleton skeleton-text" style={{ width: '80%' }} /></div>;

  const localeLabel = locales.find(l => l.code === activeLocale)?.name ?? activeLocale;

  return (
    <div className="page">
      <PageHeader title="Legal Pages" />

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
          <h2>Impressum ({localeLabel})</h2>
        </div>
        <p className="admin-section-description">
          Write your Impressum in Markdown. Supports headings (##), bold (**text**), links ([text](url)), and bullet lists (- item).
        </p>
        <form onSubmit={handleSaveImpressum} className="admin-form">
          <div className="form-group">
            <label htmlFor="impressum-content">Content (Markdown)</label>
            <textarea
              id="impressum-content"
              className="legal-content-textarea"
              value={impressum[activeLocale] ?? ''}
              onChange={(e) => setImpressum((prev) => ({ ...prev, [activeLocale]: e.target.value }))}
              rows={20}
              spellCheck={false}
            />
          </div>
          <div className="form-actions">
            <button type="submit" className="btn btn-primary" disabled={savingImpressum}>
              {savingImpressum ? 'Saving…' : `Save Impressum (${localeLabel})`}
            </button>
          </div>
        </form>
      </div>

      <div className="admin-section">
        <div className="admin-section-header">
          <h2>Privacy Policy ({localeLabel})</h2>
        </div>
        <p className="admin-section-description">
          Write your Privacy Policy in Markdown.
        </p>
        <form onSubmit={handleSavePrivacy} className="admin-form">
          <div className="form-group">
            <label htmlFor="privacy-content">Content (Markdown)</label>
            <textarea
              id="privacy-content"
              className="legal-content-textarea"
              value={privacy[activeLocale] ?? ''}
              onChange={(e) => setPrivacy((prev) => ({ ...prev, [activeLocale]: e.target.value }))}
              rows={20}
              spellCheck={false}
            />
          </div>
          <div className="form-actions">
            <button type="submit" className="btn btn-primary" disabled={savingPrivacy}>
              {savingPrivacy ? 'Saving…' : `Save Privacy Policy (${localeLabel})`}
            </button>
          </div>
        </form>
      </div>

      <div className="admin-section">
        <div className="admin-section-header">
          <h2>Widerrufsbelehrung ({localeLabel})</h2>
        </div>
        <p className="admin-section-description">
          Write your Widerrufsbelehrung (Refund Policy) in Markdown. German law requires this for digital content sales.
        </p>
        <form onSubmit={handleSaveRefund} className="admin-form">
          <div className="form-group">
            <label htmlFor="refund-content">Content (Markdown)</label>
            <textarea
              id="refund-content"
              className="legal-content-textarea"
              value={refund[activeLocale] ?? ''}
              onChange={(e) => setRefund((prev) => ({ ...prev, [activeLocale]: e.target.value }))}
              rows={20}
              spellCheck={false}
            />
          </div>
          <div className="form-actions">
            <button type="submit" className="btn btn-primary" disabled={savingRefund}>
              {savingRefund ? 'Saving…' : `Save Widerrufsbelehrung (${localeLabel})`}
            </button>
          </div>
        </form>
      </div>

      <div className="admin-section">
        <div className="admin-section-header">
          <h2>Terms of Use ({localeLabel})</h2>
        </div>
        <p className="admin-section-description">
          Write your Terms of Use (AGB) in Markdown.
        </p>
        <form onSubmit={handleSaveTerms} className="admin-form">
          <div className="form-group">
            <label htmlFor="terms-content">Content (Markdown)</label>
            <textarea
              id="terms-content"
              className="legal-content-textarea"
              value={terms[activeLocale] ?? ''}
              onChange={(e) => setTerms((prev) => ({ ...prev, [activeLocale]: e.target.value }))}
              rows={20}
              spellCheck={false}
            />
          </div>
          <div className="form-actions">
            <button type="submit" className="btn btn-primary" disabled={savingTerms}>
              {savingTerms ? 'Saving…' : `Save Terms of Use (${localeLabel})`}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
