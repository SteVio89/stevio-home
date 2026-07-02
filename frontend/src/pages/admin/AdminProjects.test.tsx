import { describe, it, expect, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import AdminProjects from './AdminProjects';

vi.mock('../../context/AuthContext', () => ({
  useAuth: () => ({ loggedIn: true, email: 'admin@test.com', isAdmin: true }),
}));

vi.mock('../../context/ToastContext', () => ({
  useToast: () => ({ addToast: vi.fn() }),
}));

describe('AdminProjects', () => {
  it('renders project list', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve([]),
      headers: new Headers({ 'Content-Type': 'application/json' }),
    }));

    render(
      <MemoryRouter>
        <AdminProjects />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('Projects')).toBeInTheDocument();
    });
  });

  it('shows empty state when no projects', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve([]),
      headers: new Headers({ 'Content-Type': 'application/json' }),
    }));

    render(
      <MemoryRouter>
        <AdminProjects />
      </MemoryRouter>
    );

    await waitFor(() => {
      expect(screen.getByText('No projects yet')).toBeInTheDocument();
    });
  });
});
