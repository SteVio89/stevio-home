import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Account from './Account';

// Mock API client
vi.mock('../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/client')>();
  return {
    ...actual,
    getLicenses: vi.fn(),
    getOrders: vi.fn(),
    exportUserData: vi.fn(),
    deleteUserData: vi.fn(),
    createDownloadToken: vi.fn(),
  };
});

// Mock useAuth
vi.mock('../context/AuthContext', () => ({
  useAuth: () => ({ loggedIn: true, email: 'test@test.com', isAdmin: false, logout: vi.fn() }),
}));

// Mock useSiteConfig
vi.mock('../context/SiteConfigContext', () => ({
  useSiteConfig: () => ({
    currency_symbol: '€',
    currency_code: 'EUR',
    site_name: 'Test Store',
    maintenance_mode: false,
    payment_enabled: true,
    max_activations: 3,
    locales: [{ code: 'de', is_default: true }],
  }),
}));

// Mock useLocale
vi.mock('../context/LocaleContext', () => ({
  useLocale: () => ({ locale: 'de', setLocale: vi.fn() }),
}));

// Mock useToast
vi.mock('../context/ToastContext', () => ({
  useToast: () => ({ addToast: vi.fn() }),
}));

import { getLicenses, getOrders } from '../api/client';
import type { License, UserOrder } from '../api/client';

const mockGetLicenses = vi.mocked(getLicenses);
const mockGetOrders = vi.mocked(getOrders);

const mockLicense: License = {
  id: 'lic-1',
  key: 'ABCD-EFGH-IJKL-MNOP',
  order_id: 'order-1',
  app_id: 'com.test.app',
  app_bundle_id: 'com.test.app',
  app_name: 'Test App',
  created_at: '2025-01-15T00:00:00Z',
  activations: [],
};

const mockOrder: UserOrder = {
  id: 'order-1',
  app_name: 'Test App',
  app_id: 'com.test.app',
  price_paid_cents: 2999,
  created_at: '2025-01-15T00:00:00Z',
};

function renderAccount() {
  return render(
    <MemoryRouter>
      <Account />
    </MemoryRouter>
  );
}

describe('Account', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    Object.defineProperty(window, 'location', {
      writable: true,
      value: { href: '' },
    });
  });

  it('renders LicenseCard with download button when user has licenses', async () => {
    mockGetLicenses.mockResolvedValue([mockLicense]);
    mockGetOrders.mockResolvedValue([mockOrder]);

    renderAccount();

    // Wait for data to load — license key renders inside LicenseCard
    await waitFor(() => {
      expect(screen.getByText('ABCD-EFGH-IJKL-MNOP')).toBeInTheDocument();
    });

    // LicenseCard renders the app name (at least one instance — also in orders table)
    const appNameInstances = screen.getAllByText('Test App');
    expect(appNameInstances.length).toBeGreaterThan(0);

    // Download button (German: "Herunterladen") is present — verifies LicenseCard + download functionality
    expect(screen.getByText('Herunterladen')).toBeInTheDocument();
  });

  it('renders Skeleton loading state during data fetch', () => {
    // Never-resolving promise — page stays in loading state
    mockGetLicenses.mockReturnValue(new Promise(() => {}));
    mockGetOrders.mockReturnValue(new Promise(() => {}));

    renderAccount();

    // Skeleton elements are rendered (aria-hidden containers)
    const skeletonContainers = document.querySelectorAll('[aria-hidden="true"]');
    expect(skeletonContainers.length).toBeGreaterThan(0);
  });
});
