import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import RequireAuth from './RequireAuth';

// Mock useAuth
const mockUseAuth = vi.fn();
vi.mock('../context/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}));

function renderWithRouter(ui: React.ReactNode) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

describe('RequireAuth', () => {
  it('shows loading state', () => {
    mockUseAuth.mockReturnValue({ loading: true, loggedIn: false });
    renderWithRouter(<RequireAuth><div>Protected</div></RequireAuth>);
    expect(screen.getByText('Laden…')).toBeInTheDocument();
    expect(screen.queryByText('Protected')).not.toBeInTheDocument();
  });

  it('redirects to /login when not logged in', () => {
    mockUseAuth.mockReturnValue({ loading: false, loggedIn: false });
    renderWithRouter(<RequireAuth><div>Protected</div></RequireAuth>);
    expect(screen.queryByText('Protected')).not.toBeInTheDocument();
  });

  it('renders children when logged in', () => {
    mockUseAuth.mockReturnValue({ loading: false, loggedIn: true });
    renderWithRouter(<RequireAuth><div>Protected</div></RequireAuth>);
    expect(screen.getByText('Protected')).toBeInTheDocument();
  });
});
