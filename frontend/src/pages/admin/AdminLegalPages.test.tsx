import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import AdminLegalPages from './AdminLegalPages';

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

describe('AdminLegalPages', () => {
  it('renders and loads legal content from page translation API', async () => {
    const mockFetch = vi.fn().mockImplementation((url: string, opts?: RequestInit) => {
      if (typeof url === 'string' && url.includes('/admin/page-translations/') && (!opts || opts.method !== 'PUT')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ content: 'Test content' }),
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
        <AdminLegalPages />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Legal Pages')).toBeInTheDocument();
    });

    // Verify fetch was called with page translation URLs (not settings)
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining('/admin/page-translations/impressum/de'),
      expect.anything()
    );
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining('/admin/page-translations/privacy_policy/de'),
      expect.anything()
    );
    expect(mockFetch).toHaveBeenCalledWith(
      expect.stringContaining('/admin/page-translations/refund_policy/de'),
      expect.anything()
    );

    // Verify no settings API calls were made
    const settingsCalls = (mockFetch.mock.calls as [string][]).filter(([url]) =>
      typeof url === 'string' && url.includes('/admin/settings')
    );
    expect(settingsCalls).toHaveLength(0);
  });

  it('saves legal section via page translation PUT', async () => {
    const mockFetch = vi.fn().mockImplementation((url: string, opts?: RequestInit) => {
      if (typeof url === 'string' && url.includes('/admin/page-translations/') && (!opts || opts.method !== 'PUT')) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ content: 'Test content' }),
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
        <AdminLegalPages />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText(/Save Impressum/)).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText(/Save Impressum/));

    await waitFor(() => {
      const putCall = (mockFetch.mock.calls as [string, RequestInit][]).find(
        ([url, opts]) =>
          url.includes('/admin/page-translations/impressum/de') && opts?.method === 'PUT'
      );
      expect(putCall).toBeDefined();
      const body = JSON.parse(putCall![1].body as string);
      expect(body).toHaveProperty('fields');
    });
  });
});
