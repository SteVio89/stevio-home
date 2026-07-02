import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Chat from './Chat';

// Mock useAuth
const mockUseAuth = vi.fn();
vi.mock('../context/AuthContext', () => ({
  useAuth: () => mockUseAuth(),
}));

// Mock useSiteConfig
vi.mock('../context/SiteConfigContext', () => ({
  useSiteConfig: () => ({ site_name: 'Test Store', locales: [{ code: 'de', is_default: true }] }),
}));

// Mock useToast
vi.mock('../context/ToastContext', () => ({
  useToast: () => ({ addToast: vi.fn() }),
}));

function renderChat() {
  return render(
    <MemoryRouter>
      <Chat />
    </MemoryRouter>
  );
}

describe('Chat', () => {
  beforeEach(() => {
    mockUseAuth.mockReturnValue({ loggedIn: true, email: 'test@test.com', isAdmin: false });
    // jsdom doesn't implement scrollIntoView
    Element.prototype.scrollIntoView = vi.fn();
  });

  it('shows "Start Chat" when no conversation exists', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 404,
      json: () => Promise.resolve({ error: 'not_found', message: 'Not found' }),
      headers: new Headers({ 'Content-Type': 'application/json' }),
    }));

    renderChat();

    await waitFor(() => {
      expect(screen.getByText('Hast du eine Frage? Starte einen Chat mit uns.')).toBeInTheDocument();
    });
    expect(screen.getByText('Chat starten')).toBeInTheDocument();
  });

  it('renders messages when conversation exists', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({
        conversation: {
          id: 'conv-1',
          display_name: 'Bold Eagle',
          email_shared: false,
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
        },
        messages: [
          { id: 'msg-1', conversation_id: 'conv-1', sender: 'user', body: 'Hello!', created_at: '2026-01-01T00:00:00Z' },
          { id: 'msg-2', conversation_id: 'conv-1', sender: 'admin', body: 'Hi, how can I help?', created_at: '2026-01-01T00:01:00Z' },
        ],
        has_unread: false,
      }),
      headers: new Headers({ 'Content-Type': 'application/json' }),
    }));

    renderChat();

    await waitFor(() => {
      expect(screen.getByText('Hello!')).toBeInTheDocument();
    });
    expect(screen.getByText('Hi, how can I help?')).toBeInTheDocument();
    expect(screen.getByText('Bold Eagle')).toBeInTheDocument();
  });

  it('shows banned message when user is forbidden', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 403,
      json: () => Promise.resolve({ error: 'forbidden', message: 'Forbidden' }),
      headers: new Headers({ 'Content-Type': 'application/json' }),
    }));

    renderChat();

    await waitFor(() => {
      expect(screen.getByText('Du wurdest vom Chat-Support ausgeschlossen.')).toBeInTheDocument();
    });
  });

  it('shows share email button when email is not shared', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({
        conversation: {
          id: 'conv-1',
          display_name: 'Bold Eagle',
          email_shared: false,
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
        },
        messages: [],
        has_unread: false,
      }),
      headers: new Headers({ 'Content-Type': 'application/json' }),
    }));

    renderChat();

    await waitFor(() => {
      expect(screen.getByText('E-Mail teilen')).toBeInTheDocument();
    });
  });

  it('shows "Email shared" badge when email is shared', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: () => Promise.resolve({
        conversation: {
          id: 'conv-1',
          display_name: 'test@test.com',
          email_shared: true,
          created_at: '2026-01-01T00:00:00Z',
          updated_at: '2026-01-01T00:00:00Z',
        },
        messages: [],
        has_unread: false,
      }),
      headers: new Headers({ 'Content-Type': 'application/json' }),
    }));

    renderChat();

    await waitFor(() => {
      expect(screen.getByText('E-Mail geteilt')).toBeInTheDocument();
    });
    // Share button should not be visible
    expect(screen.queryByText('E-Mail teilen')).not.toBeInTheDocument();
  });
});
