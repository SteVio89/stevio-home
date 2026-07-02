import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { sendMagicLink } from '../api/client';
import { useSiteConfig } from '../context/SiteConfigContext';
import { useDocumentHead } from '../hooks/useDocumentHead';

export default function MaintenanceWall() {
  const { site_name } = useSiteConfig();
  const { t } = useTranslation();

  useDocumentHead({ title: t('maintenance.title'), noindex: true });
  const [email, setEmail] = useState('');
  const [sent, setSent] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      await sendMagicLink(email);
      localStorage.setItem('user_email', email);
      setSent(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('maintenance.error_generic'));
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="maintenance-wall">
      <div className="maintenance-wall-inner">
        <h1 className="maintenance-wall-title">{site_name}</h1>
        <p className="maintenance-wall-subtitle">{t('maintenance.subtitle')}</p>

        {sent ? (
          <p className="maintenance-wall-sent">{t('maintenance.sent')}</p>
        ) : (
          <form onSubmit={handleSubmit} className="maintenance-wall-form">
            <input
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder={t('maintenance.admin_placeholder')}
              required
              autoFocus
            />
            {error && <p className="error">{error}</p>}
            <button className="btn btn-primary" type="submit" disabled={loading}>
              {loading ? t('maintenance.sending') : t('maintenance.admin_login')}
            </button>
          </form>
        )}

      </div>
    </div>
  );
}
