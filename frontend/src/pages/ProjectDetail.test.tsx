import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import ProjectDetail from './ProjectDetail';
import type { ProjectDetail as ProjectDetailType } from '../api/client';

// --- Mocks ---

vi.mock('../api/client', () => ({
  getProjectDetail: vi.fn(),
  getOwnership: vi.fn().mockResolvedValue({ has_license: false }),
  getAutoDiscount: vi.fn().mockRejectedValue(new Error('no discount')),
  validateDiscountCode: vi.fn(),
  createCheckoutSession: vi.fn(),
  getProjectVersions: vi.fn().mockResolvedValue([]),
  APIError: class APIError extends Error {
    code: string;
    status: number;
    constructor(message: string, code: string, status: number) {
      super(message);
      this.code = code;
      this.status = status;
    }
  },
}));

vi.mock('../context/AuthContext', () => ({
  useAuth: () => ({ loggedIn: true, email: 'test@test.com', isAdmin: false }),
}));

vi.mock('../context/SiteConfigContext', () => ({
  useSiteConfig: () => ({
    currency_symbol: '\u20ac',
    currency_code: 'EUR',
    site_name: 'Test Shop',
    maintenance_mode: false,
    payment_enabled: true,
    max_activations: 3,
    base_url: 'https://store.example.com',
    locales: [],
  }),
}));

vi.mock('../context/LocaleContext', () => ({
  useLocale: () => ({ locale: 'de', setLocale: vi.fn() }),
}));

vi.mock('../hooks/useDocumentHead', () => ({
  useDocumentHead: vi.fn(),
}));

vi.mock('react-router-dom', async (importOriginal) => {
  const actual = await importOriginal<typeof import('react-router-dom')>();
  return {
    ...actual,
    useParams: () => ({ slug: 'test-project' }),
    useNavigate: () => vi.fn(),
  };
});

// --- Test fixture ---

const testProject: ProjectDetailType = {
  id: 'p1',
  slug: 'test-project',
  position: 1,
  image_url: '/media/projects/p1.png',
  has_detail_page: true,
  title: 'Test Project',
  tagline: 'A test project',
  description: '<p>Test description</p>',
  system_requirements: '<p>macOS 14.0+, Apple Silicon</p>',
  images: [],
  versions: [],
  commerce: {
    id: 'a1',
    bundle_id: 'com.test.app',
    price_cents: 2999,
    purchase_mode: 'one_time_only',
    tax_category: 'digital-goods',
  },
};

function renderProjectDetail() {
  return render(
    <MemoryRouter>
      <ProjectDetail />
    </MemoryRouter>
  );
}

describe('ProjectDetail page', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows skeleton components during loading state', async () => {
    const { getProjectDetail } = await import('../api/client');
    vi.mocked(getProjectDetail).mockReturnValue(new Promise(() => {}));

    renderProjectDetail();

    const skeletons = document.querySelectorAll('.skeleton');
    expect(skeletons.length).toBeGreaterThan(0);
    expect(screen.queryByText('Laden\u2026')).not.toBeInTheDocument();
  });

  it('renders system requirements section when project has non-empty system_requirements', async () => {
    const { getProjectDetail } = await import('../api/client');
    vi.mocked(getProjectDetail).mockResolvedValue(testProject);

    renderProjectDetail();

    await waitFor(() => {
      expect(screen.getByText('Systemanforderungen')).toBeInTheDocument();
    });

    expect(screen.getByText('macOS 14.0+, Apple Silicon')).toBeInTheDocument();
  });

  it('hides system requirements when system_requirements is empty', async () => {
    const { getProjectDetail } = await import('../api/client');
    vi.mocked(getProjectDetail).mockResolvedValue({ ...testProject, system_requirements: '' });

    renderProjectDetail();

    await waitFor(() => {
      expect(screen.getByText('A test project')).toBeInTheDocument();
    });

    expect(screen.queryByText('Systemanforderungen')).not.toBeInTheDocument();
  });

  it('renders description in .app-prose container', async () => {
    const { getProjectDetail } = await import('../api/client');
    vi.mocked(getProjectDetail).mockResolvedValue(testProject);

    renderProjectDetail();

    await waitFor(() => {
      expect(screen.getByText('Test description')).toBeInTheDocument();
    });

    const proseContainers = document.querySelectorAll('.app-prose');
    expect(proseContainers.length).toBeGreaterThan(0);
  });

  it('renders version history heading via VersionAccordion section', async () => {
    const { getProjectDetail } = await import('../api/client');
    vi.mocked(getProjectDetail).mockResolvedValue(testProject);

    renderProjectDetail();

    await waitFor(() => {
      expect(screen.getByText('Versionsverlauf')).toBeInTheDocument();
    });
  });

  it('hides PurchaseSidebar when project has no commerce', async () => {
    const { getProjectDetail } = await import('../api/client');
    vi.mocked(getProjectDetail).mockResolvedValue({ ...testProject, commerce: undefined });

    renderProjectDetail();

    await waitFor(() => {
      expect(screen.getByText('A test project')).toBeInTheDocument();
    });

    // Sidebar wrapper not present when no commerce
    expect(document.querySelector('.app-detail-sidebar-wrap')).toBeNull();
  });
});
