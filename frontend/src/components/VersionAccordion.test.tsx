import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { VersionAccordion } from './VersionAccordion';
import type { AppVersion } from '../api/client';

const mockVersions: AppVersion[] = [
  {
    id: 'v1',
    app_id: 'app-1',
    version: '1.2.0',
    download_url: '',
    file_path: '',
    release_notes: '<ul><li>Bug fixes</li></ul>',
    published_at: '2026-03-15T00:00:00Z',
  },
  {
    id: 'v2',
    app_id: 'app-1',
    version: '1.1.0',
    download_url: '',
    file_path: '',
    release_notes: '<ul><li>Initial release</li></ul>',
    published_at: '2026-02-01T00:00:00Z',
  },
];

describe('VersionAccordion', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  function mockFetchOk(data: unknown) {
    return vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'Content-Type': 'application/json' }),
      json: () => Promise.resolve(data),
    });
  }

  it('shows loading skeleton while fetching', () => {
    vi.stubGlobal('fetch', () => new Promise(() => {})); // never resolves
    render(<VersionAccordion slug="my-project" />);
    const skeletons = document.querySelectorAll('.skeleton');
    expect(skeletons.length).toBeGreaterThan(0);
  });

  it('renders version entries with version number and formatted date', async () => {
    vi.stubGlobal('fetch', mockFetchOk(mockVersions));
    render(<VersionAccordion slug="my-project" />);
    await waitFor(() => {
      expect(screen.getByText(/1\.2\.0/)).toBeInTheDocument();
    });
    expect(screen.getByText(/1\.1\.0/)).toBeInTheDocument();
  });

  it('first item defaults to open, others closed', async () => {
    vi.stubGlobal('fetch', mockFetchOk(mockVersions));
    render(<VersionAccordion slug="my-project" />);
    await waitFor(() => {
      expect(document.querySelectorAll('details.version-accordion-item').length).toBe(2);
    });
    const items = document.querySelectorAll('details.version-accordion-item');
    expect(items[0]).toHaveAttribute('open');
    expect(items[1]).not.toHaveAttribute('open');
  });

  it('shows error message when fetch fails', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      headers: new Headers({ 'Content-Type': 'application/json' }),
      json: () => Promise.resolve({ error: 'server_error', message: 'Internal error' }),
    }));
    render(<VersionAccordion slug="my-project" />);
    await waitFor(() => {
      expect(screen.getByText(/versionsverlauf nicht verfügbar/i)).toBeInTheDocument();
    });
  });
});
