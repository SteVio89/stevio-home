import { useMemo } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useSiteConfig } from '../context/SiteConfigContext';
import { formatPrice } from '../utils/format';

// MockCheckout is a dev-only stand-in for a real provider's checkout page.
// It reads the params our mock payment provider embedded in the URL, renders
// a prominent [MOCK] banner so no one confuses it with a real checkout, and
// offers Pay / Cancel buttons. The Pay button hits the backend's mock trigger
// endpoint which signs a webhook envelope and dispatches it through the real
// WebhookReceive pipeline — same code path real Paddle webhooks go through.
//
// Guarded at runtime by the backend (the trigger endpoint 404s unless
// payment_provider='mock'), so a production deploy that accidentally exposes
// this route cannot actually issue licenses.
export default function MockCheckout() {
  const [params] = useSearchParams();
  const { currency_symbol } = useSiteConfig();

  const sessionID = params.get('session_id') ?? '';
  const appID = params.get('app_id') ?? '';
  const appName = params.get('app_name') ?? '';
  const priceCents = parseInt(params.get('price_cents') ?? '0', 10);
  const currencyCode = params.get('currency_code') ?? '';
  const discountCode = params.get('discount_code') ?? '';
  const consentGivenAt = params.get('consent_given_at') ?? '';

  // The trigger URL carries all the same params plus action=pay. The backend
  // reads them, builds a signed envelope, and invokes processWebhook.
  const payUrl = useMemo(() => {
    const q = new URLSearchParams();
    q.set('action', 'pay');
    q.set('session_id', sessionID);
    q.set('app_id', appID);
    q.set('price_cents', String(priceCents));
    if (currencyCode) q.set('currency_code', currencyCode);
    if (discountCode) q.set('discount_code', discountCode);
    if (consentGivenAt) q.set('consent_given_at', consentGivenAt);
    return `/api/checkout/mock/trigger?${q.toString()}`;
  }, [sessionID, appID, priceCents, currencyCode, discountCode, consentGivenAt]);

  const symbol = currency_symbol || currencyCode || '';

  if (!sessionID || !appID) {
    return (
      <main className="page">
        <h1>Mock Checkout — invalid link</h1>
        <p>Missing session_id or app_id.</p>
      </main>
    );
  }

  return (
    <main className="page">
      <div
        style={{
          padding: '12px',
          marginBottom: '24px',
          background: '#fff4e5',
          border: '2px dashed #f39200',
          borderRadius: '8px',
          color: '#7a4600',
          fontWeight: 600,
        }}
        role="status"
      >
        [MOCK] Simulated checkout — no real payment is processed.
      </div>

      <h1>{appName || 'Mock purchase'}</h1>
      <p>
        Amount: <strong>{formatPrice(priceCents, symbol)}</strong>
        {discountCode ? <> (discount code <code>{discountCode}</code>)</> : null}
      </p>
      <p style={{ color: '#666' }}>Session: <code>{sessionID}</code></p>

      <div style={{ display: 'flex', gap: '12px', marginTop: '24px' }}>
        <a className="btn btn-primary" href={payUrl}>
          Pay (simulate)
        </a>
        <a className="btn" href="/">
          Cancel
        </a>
      </div>

      <p className="form-hint" style={{ marginTop: '24px' }}>
        Clicking Pay sends a signed mock webhook to{' '}
        <code>/api/payment/webhook</code>. The resulting order and license go
        through the same pipeline as a real Paddle payment.
      </p>
    </main>
  );
}
