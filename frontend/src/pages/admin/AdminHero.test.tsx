import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import AdminHero from './AdminHero';

vi.mock('../../context/AuthContext', () => ({
  useAuth: () => ({ loggedIn: true, email: 'admin@test.com', isAdmin: true }),
}));

vi.mock('../../context/ToastContext', () => ({
  useToast: () => ({ addToast: vi.fn() }),
}));

vi.mock('../../hooks/useAdminLocales', () => ({
  useAdminLocales: () => ({
    locales: [{ code: 'de', name: 'Deutsch', is_default: true, enabled: true, sort_order: 0 }],
    loading: false,
    error: '',
    reload: vi.fn(),
  }),
}));

describe('AdminHero', () => {
  it('renders and loads hero content from page translation API', async () => {
    const mockFetch = vi.fn().mockImplementation((url: string, opts?: RequestInit) => {
      if (url.includes('/admin/page-translations/hero/de') && (!opts || opts.method !== 'PUT')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ headline: 'Test Headline', tagline: 'Test Tagline', bio: 'Test Bio' }),
          headers: new Headers({ 'Content-Type': 'application/json' }),
        });
      }
      return Promise.resolve({
        ok: true,
        status: 204,
        json: () => Promise.resolve(undefined),
        headers: new Headers({}),
      });
    });
    vi.stubGlobal('fetch', mockFetch);

    render(
      <MemoryRouter>
        <AdminHero />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Hero Content')).toBeInTheDocument();
    });

    // Verify fetch was called with page translation URL (not settings)
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining('/admin/page-translations/hero/de'),
      expect.anything()
    );

    // Verify loaded values are displayed
    await waitFor(() => {
      expect(screen.getByDisplayValue('Test Headline')).toBeInTheDocument();
      expect(screen.getByDisplayValue('Test Tagline')).toBeInTheDocument();
      expect(screen.getByDisplayValue('Test Bio')).toBeInTheDocument();
    });
  });

  it('saves hero content via page translation PUT', async () => {
    const mockFetch = vi.fn().mockImplementation((url: string, opts?: RequestInit) => {
      if (url.includes('/admin/page-translations/hero/de') && (!opts || opts.method !== 'PUT')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ headline: 'Existing Headline', tagline: 'Existing Tagline', bio: 'Existing Bio' }),
          headers: new Headers({ 'Content-Type': 'application/json' }),
        });
      }
      // PUT call
      return Promise.resolve({
        ok: true,
        status: 204,
        json: () => Promise.resolve(undefined),
        headers: new Headers({}),
      });
    });
    vi.stubGlobal('fetch', mockFetch);

    render(
      <MemoryRouter>
        <AdminHero />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Save Hero')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText('Save Hero'));

    await waitFor(() => {
      const putCall = (mockFetch.mock.calls as [string, RequestInit][]).find(
        ([url, opts]) =>
          url.includes('/admin/page-translations/hero/de') && opts?.method === 'PUT'
      );
      expect(putCall).toBeDefined();
      const body = JSON.parse(putCall![1].body as string);
      expect(body).toHaveProperty('fields');
    });
  });
});
