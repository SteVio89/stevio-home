import { useState, useEffect } from 'react';
import PageHeader from '../../components/PageHeader';
import { useAdminLocales } from '../../hooks/useAdminLocales';
import { adminGetMailTemplate, adminUpsertMailTemplate } from '../../api/client';
import { useToast } from '../../context/ToastContext';

export default function AdminMailTemplates() {
  const { locales, loading: localesLoading } = useAdminLocales();
  const { addToast } = useToast();
  const [activeLocale, setActiveLocale] = useState('');
  const [subject, setSubject] = useState('');
  const [body, setBody] = useState('');
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  // Set initial locale when loaded
  useEffect(() => {
    if (locales.length > 0 && !activeLocale) {
      setActiveLocale(locales[0].code);
    }
  }, [locales, activeLocale]);

  // Load template for active locale
  useEffect(() => {
    if (!activeLocale) return;
    setLoading(true);
    adminGetMailTemplate(activeLocale)
      .then((tmpl) => {
        setSubject(tmpl.subject);
        setBody(tmpl.body);
      })
      .catch(() => {
        setSubject('');
        setBody('');
      })
      .finally(() => setLoading(false));
  }, [activeLocale]);

  async function handleSave(ev: React.FormEvent) {
    ev.preventDefault();
    setSubmitting(true);
    try {
      await adminUpsertMailTemplate(activeLocale, { subject, body });
      addToast('Template saved', 'success');
    } catch {
      addToast('Failed to save template', 'error');
    } finally {
      setSubmitting(false);
    }
  }

  if (localesLoading) {
    return (
      <div className="page">
        <div className="skeleton skeleton-text" style={{ width: '60%' }} />
        <div className="skeleton skeleton-text" style={{ width: '40%' }} />
      </div>
    );
  }

  return (
    <div className="page">
      <PageHeader title="Mail Templates" />

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
          <h2>Magic Link Email</h2>
        </div>
        <p className="admin-section-description">
          Customize the magic link email subject and body for this locale.
        </p>

        {loading ? (
          <div className="skeleton skeleton-text" style={{ width: '80%' }} />
        ) : (
          <form onSubmit={handleSave}>
            <div className="form-group">
              <label htmlFor="mail-subject">Subject</label>
              <input
                id="mail-subject"
                type="text"
                value={subject}
                onChange={(e) => setSubject(e.target.value)}
              />
            </div>
            <div className="form-group">
              <label htmlFor="mail-body">Body</label>
              <textarea
                id="mail-body"
                rows={12}
                value={body}
                onChange={(e) => setBody(e.target.value)}
              />
              <p className="form-hint">
                Use %s for the magic link URL and %d for the TTL in minutes.
              </p>
            </div>
            <div className="form-actions">
              <button type="submit" className="btn btn-primary" disabled={submitting}>
                {submitting ? 'Saving...' : 'Save'}
              </button>
            </div>
          </form>
        )}
      </div>
    </div>
  );
}
