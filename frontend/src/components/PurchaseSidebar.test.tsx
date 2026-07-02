import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { PurchaseSidebar } from './PurchaseSidebar';
import type { ProjectDetail, Commerce, OwnershipStatus, DiscountValidation } from '../api/client';

// Mutable holder so individual tests can swap the active payment provider
// without tearing down / re-registering the vi.mock. Defaults to Paddle.
const siteConfigState = { payment_provider: 'paddle' };

vi.mock('../context/SiteConfigContext', () => ({
  useSiteConfig: () => ({
    currency_symbol: '€',
    currency_code: 'EUR',
    site_name: 'Test Shop',
    maintenance_mode: false,
    payment_enabled: true,
    payment_provider: siteConfigState.payment_provider,
    max_activations: 5,
    base_url: '',
    locales: [],
  }),
}));

const baseProject: ProjectDetail = {
  id: 'p1',
  slug: 'test-app',
  position: 1,
  image_url: '',
  has_detail_page: true,
  title: 'Test App',
  tagline: 'A test app',
  description: 'Test description',
};

const baseCommerce: Commerce = {
  id: 'app-1',
  bundle_id: 'com.test.app',
  price_cents: 2999,
  purchase_mode: 'always_new_license',
  tax_category: 'digital-goods',
};

const defaultCheckout = {
  loading: false,
  error: '',
  onBuy: vi.fn(),
};

const defaultDiscountState = {
  code: '',
  applied: null as DiscountValidation | null,
  auto: null as DiscountValidation | null,
  error: '',
  loading: false,
  onChange: vi.fn(),
  onApply: vi.fn(),
};

function renderSidebar(
  commerceOverrides: Partial<Commerce> = {},
  ownership: OwnershipStatus | null = null
) {
  return render(
    <MemoryRouter initialEntries={['/de/project/test-app']}>
      <PurchaseSidebar
        project={baseProject}
        commerce={{ ...baseCommerce, ...commerceOverrides }}
        locale="de"
        ownership={ownership}
        checkout={defaultCheckout}
        discountState={defaultDiscountState}
      />
    </MemoryRouter>
  );
}

describe('PurchaseSidebar', () => {
  beforeEach(() => {
    siteConfigState.payment_provider = 'paddle';
  });

  it('paid app renders buy button, trust section with Paddle text, and refund link', () => {
    renderSidebar();
    expect(screen.getByRole('button', { name: /jetzt kaufen/i })).toBeInTheDocument();
    expect(screen.getByText(/paddle/i)).toBeInTheDocument();
    expect(screen.getByText(/widerrufsrecht/i)).toBeInTheDocument();
  });

  it('coming_soon app renders "Demnächst verfügbar" — no buy button, no trust section', () => {
    renderSidebar({ purchase_mode: 'coming_soon' });
    expect(screen.getByText('Demnächst verfügbar')).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
    expect(screen.queryByText(/paddle/i)).not.toBeInTheDocument();
  });

  it('free app (price_cents=0) renders "Kostenlos laden" — no trust section, no discount input', () => {
    renderSidebar({ price_cents: 0 });
    expect(screen.getByText('Kostenlos laden')).toBeInTheDocument();
    expect(screen.queryByText(/paddle/i)).not.toBeInTheDocument();
    expect(screen.queryByPlaceholderText(/rabattcode/i)).not.toBeInTheDocument();
  });

  it('trust section contains refund policy link to widerruf page', () => {
    renderSidebar();
    const refundLink = screen.getByRole('link', { name: /widerrufsrecht/i });
    expect(refundLink).toBeInTheDocument();
    expect(refundLink.getAttribute('href')).toContain('widerruf');
  });

  it('does not render provider trust text when payment_provider is mock', () => {
    siteConfigState.payment_provider = 'mock';
    renderSidebar();
    expect(screen.queryByText(/paddle/i)).not.toBeInTheDocument();
  });
});
