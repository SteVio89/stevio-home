import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { useSiteConfig } from '../context/SiteConfigContext';
import { formatPrice } from '../utils/format';
import type { ProjectDetail, Commerce, OwnershipStatus, DiscountValidation } from '../api/client';

export interface PurchaseSidebarProps {
  project: ProjectDetail;
  commerce: Commerce;
  locale: string;
  ownership: OwnershipStatus | null;
  checkout: {
    loading: boolean;
    error: string;
    onBuy: () => void;
  };
  discountState: {
    code: string;
    applied: DiscountValidation | null;
    auto: DiscountValidation | null;
    error: string;
    loading: boolean;
    onChange: (code: string) => void;
    onApply: (ev: React.FormEvent) => void;
  };
}

export function PurchaseSidebar({
  project,
  commerce,
  locale,
  ownership,
  checkout,
  discountState,
}: PurchaseSidebarProps) {
  const { t } = useTranslation();
  const { currency_symbol, payment_provider } = useSiteConfig();
  // Only Paddle carries a trust line. Mock and Disabled render none.
  const trustKey = payment_provider === 'paddle' ? 'commerce.trust_paddle' : '';

  // coming_soon: simplified view — no buy button, no trust section
  if (commerce.purchase_mode === 'coming_soon') {
    return (
      <aside className="purchase-sidebar" aria-label="Purchase">
        <p>{t('commerce.coming_soon')}</p>
      </aside>
    );
  }

  // Free app: get button only — no trust section, no discount input
  if (commerce.price_cents === 0) {
    return (
      <aside className="purchase-sidebar" aria-label="Purchase">
        <button
          className="btn btn-primary"
          onClick={checkout.onBuy}
          disabled={checkout.loading}
          aria-label={`${t('commerce.get_free')} — ${project.title}`}
        >
          {checkout.loading ? t('commerce.redirecting') : t('commerce.get_free')}
        </button>
        {checkout.error && <p className="checkout-error">{checkout.error}</p>}
      </aside>
    );
  }

  // Paid app: full sidebar with price, discount, buy button, trust section
  const activeDiscount = discountState.applied ?? discountState.auto;
  const hasLicense = ownership?.has_license ?? false;
  const isOneTimeOwned = commerce.purchase_mode === 'one_time_only' && hasLicense;

  const purchaseModeLabel =
    commerce.purchase_mode === 'one_time_only'
      ? t('commerce.purchase_one_time')
      : t('commerce.purchase_per_device');

  return (
    <aside className="purchase-sidebar" aria-label="Purchase">
      <div className="purchase-sidebar-header">
        <h2>{project.title}</h2>
        {activeDiscount ? (
          <p className="app-price">
            <span className="app-price-original">
              {formatPrice(activeDiscount.original_price_cents, currency_symbol)}
            </span>{' '}
            <span className="app-price-discounted">
              {formatPrice(activeDiscount.final_price_cents, currency_symbol)}
            </span>
          </p>
        ) : (
          <p className="app-price">{formatPrice(commerce.price_cents, currency_symbol)}</p>
        )}
      </div>

      {!isOneTimeOwned && (
        <form onSubmit={discountState.onApply} className="discount-form">
          <input
            type="text"
            className="discount-input"
            placeholder={t('commerce.discount_placeholder')}
            value={discountState.code}
            onChange={(e) => discountState.onChange(e.target.value)}
            maxLength={50}
          />
          <button
            type="submit"
            className="btn btn-secondary btn-small"
            disabled={discountState.loading || !discountState.code.trim()}
          >
            {discountState.loading ? '…' : t('commerce.apply')}
          </button>
        </form>
      )}

      {discountState.error && <p className="discount-error">{discountState.error}</p>}
      {discountState.applied?.stacked_with_auto && (
        <p className="discount-stacked-note">{t('commerce.stacked_note')}</p>
      )}

      {isOneTimeOwned ? (
        <button className="btn btn-secondary" disabled>
          {t('commerce.already_own')}
        </button>
      ) : (
        <button
          className="btn btn-primary"
          onClick={checkout.onBuy}
          disabled={checkout.loading}
          aria-label={`${t('commerce.buy_now')} — ${project.title} — ${formatPrice(commerce.price_cents, currency_symbol)}`}
        >
          {checkout.loading ? t('commerce.redirecting') : t('commerce.buy_now')}
        </button>
      )}

      {checkout.error && <p className="checkout-error">{checkout.error}</p>}

      <div className="purchase-trust">
        <p className="purchase-mode-label">{purchaseModeLabel}</p>
        {trustKey && <p>{t(trustKey)}</p>}
        <p>
          <Link to={`/${locale}/widerruf`}>{t('commerce.trust_refund')}</Link>
        </p>
      </div>
    </aside>
  );
}
