import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  APIError,
  sendMagicLink,
  logout,
  getMe,
  getProjects,
  getProjectDetail,
  getProjectVersions,
  getLicenses,
  renameDevice,
  createDownloadToken,
  adminAttachCommerce,
} from './client';

const mockFetch = vi.fn();
vi.stubGlobal('fetch', mockFetch);

function jsonResponse(data: unknown, status = 200) {
  return Promise.resolve({
    ok: status >= 200 && status < 300,
    status,
    statusText: 'OK',
    headers: new Headers({ 'Content-Type': 'application/json' }),
    json: () => Promise.resolve(data),
  });
}

function errorResponse(error: string, message: string, status: number) {
  return Promise.resolve({
    ok: false,
    status,
    statusText: message,
    json: () => Promise.resolve({ error, message }),
  });
}

beforeEach(() => {
  mockFetch.mockReset();
});

describe('APIError', () => {
  it('stores code, message, and status', () => {
    const err = new APIError('not_found', 'Resource not found', 404);
    expect(err.code).toBe('not_found');
    expect(err.message).toBe('Resource not found');
    expect(err.status).toBe(404);
    expect(err).toBeInstanceOf(Error);
  });
});

describe('sendMagicLink', () => {
  it('sends POST with email', async () => {
    mockFetch.mockReturnValue(jsonResponse({ status: 'sent' }));
    const result = await sendMagicLink('user@test.com');
    expect(result.status).toBe('sent');
    expect(mockFetch).toHaveBeenCalledWith('/auth/login', expect.objectContaining({
      method: 'POST',
      credentials: 'include',
    }));
    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body.email).toBe('user@test.com');
  });
});

describe('getMe', () => {
  it('returns admin status', async () => {
    mockFetch.mockReturnValue(jsonResponse({ is_admin: true }));
    const result = await getMe();
    expect(result.is_admin).toBe(true);
  });

  it('throws APIError on 401', async () => {
    mockFetch.mockReturnValue(errorResponse('unauthorized', 'Authentication required', 401));
    await expect(getMe()).rejects.toThrow(APIError);
    try {
      await getMe();
    } catch (e) {
      expect(e).toBeInstanceOf(APIError);
      expect((e as APIError).status).toBe(401);
    }
  });
});

describe('logout', () => {
  it('sends POST', async () => {
    mockFetch.mockReturnValue(jsonResponse({ message: 'logged out' }));
    await logout();
    expect(mockFetch).toHaveBeenCalledWith('/auth/logout', expect.objectContaining({
      method: 'POST',
    }));
  });
});

describe('getProjects', () => {
  it('returns project list and strips null commerce', async () => {
    const projects = [
      { id: '1', slug: 'p1', title: 'P1', tagline: '', position: 1, image_url: '', has_detail_page: true, commerce: null },
      { id: '2', slug: 'p2', title: 'P2', tagline: '', position: 2, image_url: '', has_detail_page: true, commerce: { id: 'a1', bundle_id: 'com.test', price_cents: 999, purchase_mode: 'always_new_license', tax_category: 'digital-goods' } },
    ];
    mockFetch.mockReturnValue(jsonResponse(projects));
    const result = await getProjects();
    expect(result).toHaveLength(2);
    expect(result[0].commerce).toBeUndefined();
    expect(result[1].commerce?.id).toBe('a1');
    expect(mockFetch).toHaveBeenCalledWith('/api/projects', expect.anything());
  });
});

describe('getProjectDetail', () => {
  it('fetches by slug and lifts system_requirements from commerce', async () => {
    const detail = {
      id: '1',
      slug: 'my-project',
      title: 'P',
      tagline: '',
      position: 1,
      image_url: '',
      has_detail_page: true,
      description: '<p>Desc</p>',
      images: [],
      versions: [],
      commerce: { id: 'a1', bundle_id: 'com.t', price_cents: 0, purchase_mode: 'always_new_license', tax_category: 'digital-goods', system_requirements: '<p>macOS 14</p>' },
    };
    mockFetch.mockReturnValue(jsonResponse(detail));
    const result = await getProjectDetail('my-project');
    expect(result.system_requirements).toBe('<p>macOS 14</p>');
    expect(mockFetch).toHaveBeenCalledWith('/api/projects/my-project', expect.anything());
  });
});

describe('getProjectVersions', () => {
  it('fetches versions by slug', async () => {
    mockFetch.mockReturnValue(jsonResponse([]));
    const result = await getProjectVersions('my-project');
    expect(result).toEqual([]);
    expect(mockFetch).toHaveBeenCalledWith('/api/projects/my-project/versions', expect.anything());
  });
});

describe('getLicenses', () => {
  it('returns licenses', async () => {
    mockFetch.mockReturnValue(jsonResponse([]));
    const result = await getLicenses();
    expect(result).toEqual([]);
  });
});

describe('renameDevice', () => {
  it('sends PATCH with device label', async () => {
    mockFetch.mockReturnValue(jsonResponse({ message: 'updated' }));
    await renameDevice('act-1', 'New Name');
    expect(mockFetch).toHaveBeenCalledWith('/api/account/activations/act-1', expect.objectContaining({
      method: 'PATCH',
    }));
    const body = JSON.parse(mockFetch.mock.calls[0][1].body);
    expect(body.device_label).toBe('New Name');
  });
});

describe('createDownloadToken', () => {
  it('returns url and expires_at', async () => {
    mockFetch.mockReturnValue(jsonResponse({
      url: '/api/downloads/file?token=abc',
      expires_at: '2026-01-01T00:00:00Z',
    }));
    const result = await createDownloadToken('lic-1');
    expect(result.url).toBe('/api/downloads/file?token=abc');
  });
});

describe('adminAttachCommerce', () => {
  it('sends POST to project commerce endpoint', async () => {
    const created = { id: 'a1', project_id: 'p1', bundle_id: 'com.t', price_cents: 999, purchase_mode: 'always_new_license', created_at: '2026-01-01T00:00:00Z' };
    mockFetch.mockReturnValue(jsonResponse(created));
    const result = await adminAttachCommerce('p1', { bundle_id: 'com.t', price_cents: 999, purchase_mode: 'always_new_license', tax_category: 'digital-goods' });
    expect(result.id).toBe('a1');
    expect(mockFetch).toHaveBeenCalledWith('/api/admin/projects/p1/commerce', expect.objectContaining({
      method: 'POST',
    }));
  });
});

describe('error handling', () => {
  it('falls back to statusText when JSON parsing fails', async () => {
    mockFetch.mockReturnValue(Promise.resolve({
      ok: false,
      status: 500,
      statusText: 'Internal Server Error',
      json: () => Promise.reject(new Error('not json')),
    }));
    await expect(getProjects()).rejects.toThrow(APIError);
    try {
      await getProjects();
    } catch (e) {
      expect((e as APIError).code).toBe('unknown');
      expect((e as APIError).message).toBe('Internal Server Error');
    }
  });
});
