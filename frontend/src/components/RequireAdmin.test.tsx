import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import RequireAdmin from './RequireAdmin';

const mockUseAuth = vi.fn();
vi.mock('../context/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}));

function renderWithRouter(ui: React.ReactNode) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

describe('RequireAdmin', () => {
  it('shows loading state', () => {
    mockUseAuth.mockReturnValue({ loading: true, loggedIn: false, isAdmin: false });
    renderWithRouter(<RequireAdmin><div>Admin Panel</div></RequireAdmin>);
    expect(screen.getByText('Loading...')).toBeInTheDocument();
    expect(screen.queryByText('Admin Panel')).not.toBeInTheDocument();
  });

  it('redirects to /login when not logged in', () => {
    mockUseAuth.mockReturnValue({ loading: false, loggedIn: false, isAdmin: false });
    renderWithRouter(<RequireAdmin><div>Admin Panel</div></RequireAdmin>);
    expect(screen.queryByText('Admin Panel')).not.toBeInTheDocument();
  });

  it('shows access denied for non-admin user', () => {
    mockUseAuth.mockReturnValue({ loading: false, loggedIn: true, isAdmin: false });
    renderWithRouter(<RequireAdmin><div>Admin Panel</div></RequireAdmin>);
    expect(screen.getByText('Access Denied')).toBeInTheDocument();
    expect(screen.getByText('Back to Store')).toBeInTheDocument();
    expect(screen.queryByText('Admin Panel')).not.toBeInTheDocument();
  });

  it('renders children for admin user', () => {
    mockUseAuth.mockReturnValue({ loading: false, loggedIn: true, isAdmin: true });
    renderWithRouter(<RequireAdmin><div>Admin Panel</div></RequireAdmin>);
    expect(screen.getByText('Admin Panel')).toBeInTheDocument();
  });
});
