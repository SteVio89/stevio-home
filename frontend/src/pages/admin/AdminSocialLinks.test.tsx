import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import AdminSocialLinks from './AdminSocialLinks';

vi.mock('../../context/AuthContext', () => ({
  useAuth: () => ({ loggedIn: true, email: 'admin@test.com', isAdmin: true }),
}));

vi.mock('../../context/ToastContext', () => ({
  useToast: () => ({ addToast: vi.fn() }),
}));

describe('AdminSocialLinks', () => {
  it('renders social links list', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve([]),
      headers: new Headers({ 'Content-Type': 'application/json' }),
    }));

    render(
      <MemoryRouter>
        <AdminSocialLinks />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Social Links')).toBeInTheDocument();
    });
  });

  it('shows empty state when no links', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve([]),
      headers: new Headers({ 'Content-Type': 'application/json' }),
    }));

    render(
      <MemoryRouter>
        <AdminSocialLinks />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('No social links yet')).toBeInTheDocument();
    });
  });
});
