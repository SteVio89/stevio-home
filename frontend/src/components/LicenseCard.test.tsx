import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import LicenseCard from './LicenseCard';
import type { License } from '../api/client';
import { SiteConfigContext, type SiteConfig } from '../context/SiteConfigContext';

// Mock the API client
vi.mock('../api/client', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api/client')>();
  return {
    ...actual,
    createDownloadToken: vi.fn(),
  };
});

import { createDownloadToken } from '../api/client';
const mockCreateDownloadToken = vi.mocked(createDownloadToken);

const mockLicense: License = {
  id: 'lic-1',
  key: 'abc-def-ghi',
  order_id: 'order-1',
  app_id: 'com.test.app',
  app_bundle_id: 'com.test.app',
  app_name: 'Test App',
  created_at: '2025-01-01T00:00:00Z',
  activations: [],
};

const mockConfig: SiteConfig = {
  currency_symbol: '€',
  currency_code: 'EUR',
  site_name: 'Test Store',
  maintenance_mode: false,
  payment_enabled: true,
  payment_provider: 'paddle',
  paddle_client_token: 'test_token',
  paddle_environment: 'sandbox',
  max_activations: 3,
  base_url: 'http://localhost:3000',
  locales: [
    { code: 'de', name: 'Deutsch', is_default: true },
    { code: 'en', name: 'English', is_default: false },
  ],
};

function renderCard(license = mockLicense) {
  const onUpdate = vi.fn();
  return {
    onUpdate,
    ...render(
      <SiteConfigContext.Provider value={mockConfig}>
        <MemoryRouter>
          <LicenseCard license={license} onUpdate={onUpdate} />
        </MemoryRouter>
      </SiteConfigContext.Provider>
    ),
  };
}

beforeEach(() => {
  mockCreateDownloadToken.mockReset();
  // Reset location.href mock
  Object.defineProperty(window, 'location', {
    writable: true,
    value: { href: '' },
  });
});

describe('LicenseCard', () => {
  it('renders license info', () => {
    renderCard();
    expect(screen.getByText('Test App')).toBeInTheDocument();
    expect(screen.getByText('abc-def-ghi')).toBeInTheDocument();
    // Default locale is 'de', so button shows German text
    expect(screen.getByText('Herunterladen')).toBeInTheDocument();
  });

  it('validates download URL starts with /api/downloads/', async () => {
    const user = userEvent.setup();

    // Return a malicious URL
    mockCreateDownloadToken.mockResolvedValue({
      url: 'https://evil.com/malware',
      expires_at: '2026-01-01T00:00:00Z',
    });

    renderCard();
    await user.click(screen.getByText('Herunterladen'));

    await waitFor(() => {
      expect(screen.getByText('Download-Link konnte nicht erstellt werden. Bitte erneut versuchen.')).toBeInTheDocument();
    });

    // window.location.href should NOT have been set to the malicious URL
    expect(window.location.href).not.toBe('https://evil.com/malware');
  });

  it('sets location.href for valid download URL', async () => {
    const user = userEvent.setup();

    mockCreateDownloadToken.mockResolvedValue({
      url: '/api/downloads/file?token=abc123',
      expires_at: '2026-01-01T00:00:00Z',
    });

    renderCard();
    await user.click(screen.getByText('Herunterladen'));

    await waitFor(() => {
      expect(window.location.href).toBe('/api/downloads/file?token=abc123');
    });
  });

  it('shows error when download fails', async () => {
    const user = userEvent.setup();
    mockCreateDownloadToken.mockRejectedValue(new Error('Network error'));

    renderCard();
    await user.click(screen.getByText('Herunterladen'));

    await waitFor(() => {
      expect(screen.getByText('Download-Link konnte nicht erstellt werden. Bitte erneut versuchen.')).toBeInTheDocument();
    });
  });

  it('blocks javascript: URL', async () => {
    const user = userEvent.setup();
    mockCreateDownloadToken.mockResolvedValue({
      url: 'javascript:alert(1)',
      expires_at: '2099-01-01T00:00:00Z',
    });
    renderCard();
    await user.click(screen.getByText('Herunterladen'));
    await waitFor(() => {
      expect(screen.getByText('Download-Link konnte nicht erstellt werden. Bitte erneut versuchen.')).toBeInTheDocument();
    });
    expect(window.location.href).not.toBe('javascript:alert(1)');
  });

  it('blocks data: URL', async () => {
    const user = userEvent.setup();
    mockCreateDownloadToken.mockResolvedValue({
      url: 'data:text/html,<h1>evil</h1>',
      expires_at: '2099-01-01T00:00:00Z',
    });
    renderCard();
    await user.click(screen.getByText('Herunterladen'));
    await waitFor(() => {
      expect(screen.getByText('Download-Link konnte nicht erstellt werden. Bitte erneut versuchen.')).toBeInTheDocument();
    });
  });

  it('blocks protocol-relative URL', async () => {
    const user = userEvent.setup();
    mockCreateDownloadToken.mockResolvedValue({
      url: '//evil.com/malware',
      expires_at: '2099-01-01T00:00:00Z',
    });
    renderCard();
    await user.click(screen.getByText('Herunterladen'));
    await waitFor(() => {
      expect(screen.getByText('Download-Link konnte nicht erstellt werden. Bitte erneut versuchen.')).toBeInTheDocument();
    });
  });
});
