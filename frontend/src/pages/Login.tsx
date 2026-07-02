import { useEffect, useState } from 'react';
import { useSearchParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { sendMagicLink } from '../api/client';
import PageHeader from '../components/PageHeader';
import { useDocumentHead } from '../hooks/useDocumentHead';
import { useToast } from '../context/ToastContext';
import { useLocale } from '../context/LocaleContext';

export default function Login() {
  const { locale } = useLocale();
  const { t } = useTranslation();
  useDocumentHead({ title: t('login.title'), noindex: true });
  const [email, setEmail] = useState('');
  const [sent, setSent] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { addToast } = useToast();

  const errorMessages: Record<string, string> = {
    token_invalid: t('login.error_token_invalid'),
    internal_error: t('login.error_internal'),
  };

  // Run-once-on-mount: consume any ?error= search param the verify redirect
  // attached, surface it as a toast, then strip it from the URL. Re-running on
  // locale/toast changes would re-show stale errors.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    const errCode = searchParams.get('error');
    if (errCode) {
      addToast(errorMessages[errCode] || t('login.error_internal'));
      navigate(`/${locale}/login`, { replace: true });
    }
  }, []);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      await sendMagicLink(email);
      localStorage.setItem('user_email', email);
      setSent(true);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('login.error_generic'));
    } finally {
      setLoading(false);
    }
  }

  if (sent) {
    return (
      <div className="page login-page">
        <div className="login-card">
          <PageHeader title={t('login.sent_title')} />
          <p>{t('login.sent_message', { email })}</p>
          <p>{t('login.sent_expiry')}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="page login-page">
      <div className="login-card">
        <PageHeader title={t('login.title')} />
        <p>{t('login.subtitle')}</p>

        <form onSubmit={handleSubmit} className="login-form">
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder={t('login.placeholder')}
            required
            autoFocus
          />
          {error && <p className="error">{error}</p>}
          <button className="btn btn-primary" type="submit" disabled={loading}>
            {loading ? t('login.sending') : t('login.send')}
          </button>
        </form>
      </div>
    </div>
  );
}
