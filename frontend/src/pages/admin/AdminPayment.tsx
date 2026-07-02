import { useEffect, useState } from 'react';
import { adminGetSettings, adminUpdateSetting, APIError } from '../../api/client';
import PageHeader from '../../components/PageHeader';
import Skeleton from '../../components/Skeleton';

// The admin selects one of four rows. Paddle appears twice — once for each
// environment — so the UX is flat, matching the "mock / sandbox-paddle /
// real-paddle" three-tier framing. The value 'paddle:sandbox' is a compound
// that we split into two settings (payment_provider + paddle_environment) on
// save. Disabled maps to an empty payment_provider, which hides the buy button.
const PROVIDERS = [
  { value: '', label: 'Disabled' },
  { value: 'mock', label: 'Mock (dev only)' },
  { value: 'paddle:sandbox', label: 'Paddle — Sandbox' },
  { value: 'paddle:production', label: 'Paddle — Production' },
];

// Mirrors backend secretSetSentinel — when a secret is already configured,
// the server returns this placeholder instead of the ciphertext. The admin UI
// echoes it back unchanged to mean "leave value unchanged on save".
const SECRET_SENTINEL = '********';

type Secret = { value: string; set: boolean };
const emptySecret: Secret = { value: '', set: false };

function secretFromServer(raw: string | undefined): Secret {
  if (!raw) return { value: '', set: false };
  if (raw === SECRET_SENTINEL) return { value: '', set: true };
  return { value: raw, set: true };
}

// Compose the compound dropdown value from the two backend settings.
function composeProvider(paymentProvider: string, paddleEnv: string): string {
  if (paymentProvider === 'paddle') {
    return 'paddle:' + (paddleEnv === 'sandbox' ? 'sandbox' : 'production');
  }
  return paymentProvider || '';
}

// Split the compound back into the two backend settings.
function decomposeProvider(compound: string): { provider: string; env: string } {
  if (compound.startsWith('paddle:')) {
    return { provider: 'paddle', env: compound.slice('paddle:'.length) };
  }
  return { provider: compound, env: '' };
}

export default function AdminPayment() {
  const [compoundProvider, setCompoundProvider] = useState('');
  const [paddleApiKey, setPaddleApiKey] = useState<Secret>(emptySecret);
  const [paddleWebhookSecret, setPaddleWebhookSecret] = useState<Secret>(emptySecret);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState('');

  useEffect(() => {
    adminGetSettings()
      .then((s) => {
        setCompoundProvider(composeProvider(s.payment_provider ?? '', s.paddle_environment ?? ''));
        setPaddleApiKey(secretFromServer(s.paddle_api_key));
        setPaddleWebhookSecret(secretFromServer(s.paddle_webhook_secret));
      })
      .catch((err) => setError(err instanceof APIError ? err.message : 'Failed to load settings'))
      .finally(() => setLoading(false));
  }, []);

  async function handleSave(ev: React.FormEvent) {
    ev.preventDefault();
    setSubmitting(true);
    setError('');
    setSuccess('');

    const { provider, env } = decomposeProvider(compoundProvider);
    const patches: Array<[string, string]> = [
      ['payment_provider', provider],
    ];
    if (provider === 'paddle') {
      patches.push(['paddle_environment', env || 'production']);
      if (paddleApiKey.value) patches.push(['paddle_api_key', paddleApiKey.value]);
      if (paddleWebhookSecret.value) patches.push(['paddle_webhook_secret', paddleWebhookSecret.value]);
    }

    try {
      const results = await Promise.allSettled(
        patches.map(([k, v]) => adminUpdateSetting(k, v))
      );
      const failed = results
        .map((r, i) => (r.status === 'rejected' ? patches[i][0] : null))
        .filter(Boolean);
      if (failed.length > 0) {
        setError(`Failed to save: ${failed.join(', ')}`);
      } else {
        setSuccess('Payment settings saved.');
        if (paddleApiKey.value) setPaddleApiKey({ value: '', set: true });
        if (paddleWebhookSecret.value) setPaddleWebhookSecret({ value: '', set: true });
      }
    } catch (err) {
      setError(err instanceof APIError ? err.message : 'Failed to save settings');
    } finally {
      setSubmitting(false);
    }
  }

  if (loading) return (
    <div className="page">
      <PageHeader title="Payment Settings" />
      <Skeleton variant="text" width="60%" count={3} />
    </div>
  );

  const secretPlaceholder = (s: Secret) =>
    s.set ? '(configured — leave blank to keep)' : '';

  const isPaddle = compoundProvider.startsWith('paddle:');

  return (
    <div className="page">
      <PageHeader title="Payment Settings" />

      <div className="admin-section">
        <form onSubmit={handleSave} className="admin-form">
          <div className="form-row">
            <div className="form-group">
              <label htmlFor="pay-provider">Payment provider</label>
              <select
                id="pay-provider"
                value={compoundProvider}
                onChange={(e) => {
                  setCompoundProvider(e.target.value);
                  setSuccess('');
                }}
              >
                {PROVIDERS.map((p) => (
                  <option key={p.value} value={p.value}>{p.label}</option>
                ))}
              </select>
              <small>
                &quot;Disabled&quot; hides the buy button. &quot;Mock&quot; uses signed local webhooks for end-to-end testing
                without a real provider. Paddle Sandbox uses Paddle&apos;s test environment; Production is live.
              </small>
            </div>
          </div>

          {isPaddle && (
            <>
              <h3>Paddle Billing Configuration</h3>
              <p className="admin-section-description">
                Credentials are encrypted at rest with SIGNING_KEY_SECRET. Fields left blank keep their existing value.
                Product names, prices, currencies, and tax categories are sent to Paddle per-checkout as non-catalog
                items — you do <strong>not</strong> need to mirror products in the Paddle dashboard.
              </p>
              <div className="form-row">
                <div className="form-group">
                  <label htmlFor="paddle-api-key">API Key</label>
                  <input
                    id="paddle-api-key"
                    type="password"
                    autoComplete="new-password"
                    value={paddleApiKey.value}
                    onChange={(e) => setPaddleApiKey({ value: e.target.value, set: paddleApiKey.set })}
                    placeholder={secretPlaceholder(paddleApiKey)}
                    maxLength={4096}
                  />
                  <small>From Paddle → Developer Tools → Authentication</small>
                </div>
                <div className="form-group">
                  <label htmlFor="paddle-webhook-secret">Webhook Secret</label>
                  <input
                    id="paddle-webhook-secret"
                    type="password"
                    autoComplete="new-password"
                    value={paddleWebhookSecret.value}
                    onChange={(e) => setPaddleWebhookSecret({ value: e.target.value, set: paddleWebhookSecret.set })}
                    placeholder={secretPlaceholder(paddleWebhookSecret)}
                    maxLength={4096}
                  />
                  <small>From Paddle → Developer Tools → Notifications</small>
                </div>
              </div>
            </>
          )}

          {compoundProvider === 'mock' && (
            <p className="admin-section-description">
              Mock simulates the full checkout + webhook flow locally (HMAC-signed envelopes, no external calls).
              Useful for end-to-end testing and development. Never use in production.
            </p>
          )}

          {error && <p className="error">{error}</p>}
          {success && <p className="success-message">{success}</p>}
          <div className="form-actions">
            <button type="submit" className="btn btn-primary" disabled={submitting}>
              {submitting ? 'Saving…' : 'Save Payment Settings'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
