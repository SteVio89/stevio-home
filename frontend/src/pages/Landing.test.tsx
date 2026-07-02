import { describe, it, expect, vi, beforeEach, beforeAll } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Landing from './Landing';
import type { PublicProject } from '../api/client';

// Mock IntersectionObserver (not available in jsdom) — must be a class-like constructor
beforeAll(() => {
  const MockIntersectionObserver = vi.fn(function (this: IntersectionObserver) {
    this.observe = vi.fn();
    this.unobserve = vi.fn();
    this.disconnect = vi.fn();
  });
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (window as any).IntersectionObserver = MockIntersectionObserver;
});

// Mock all API functions
vi.mock('../api/client', () => ({
  getHero: vi.fn(),
  getProjects: vi.fn(),
  listPublicSocialLinks: vi.fn(),
}));

// Mock useSiteConfig
vi.mock('../context/SiteConfigContext', () => ({
  useSiteConfig: () => ({
    currency_symbol: '€',
    site_name: 'Stevio',
    base_url: 'https://stevio.de',
  }),
}));

// Mock useLocale
vi.mock('../context/LocaleContext', () => ({
  useLocale: () => ({ locale: 'de' }),
}));

// Mock useDocumentHead
vi.mock('../hooks/useDocumentHead', () => ({
  useDocumentHead: vi.fn(),
}));

import { getHero, getProjects, listPublicSocialLinks } from '../api/client';
const mockGetHero = vi.mocked(getHero);
const mockGetProjects = vi.mocked(getProjects);
const mockListPublicSocialLinks = vi.mocked(listPublicSocialLinks);

// Test data fixtures
const heroFixture = { headline: 'Hallo', tagline: 'Dev', bio: 'Bio text' };

const commerceProject: PublicProject = {
  id: '1',
  slug: 'my-app',
  position: 1,
  image_url: '/media/img.png',
  has_detail_page: true,
  title: 'App One',
  tagline: 'A cool app',
  commerce: {
    id: 'a1',
    bundle_id: 'com.test.app1',
    price_cents: 999,
    purchase_mode: 'one_time_only',
    tax_category: 'digital-goods',
  },
};

const externalProject: PublicProject = {
  id: '2',
  slug: 'oss',
  position: 2,
  image_url: '/media/img2.png',
  external_url: 'https://github.com/test/proj',
  has_detail_page: false,
  title: 'Open Source Thing',
  tagline: 'A cool OSS project',
};

const socialLinksFixture = [
  { id: 's1', platform: 'github', url: 'https://github.com/test', position: 1 },
];

function renderLanding() {
  return render(
    <MemoryRouter initialEntries={['/de/']}>
      <Landing />
    </MemoryRouter>
  );
}

describe('Landing', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetHero.mockResolvedValue(heroFixture);
    mockGetProjects.mockResolvedValue([commerceProject]);
    mockListPublicSocialLinks.mockResolvedValue(socialLinksFixture);
  });

  it('renders hero section with headline, tagline, and bio from API', async () => {
    renderLanding();

    await waitFor(() => {
      expect(screen.getByText('Hallo')).toBeInTheDocument();
    });
    expect(screen.getByText('Dev')).toBeInTheDocument();
    expect(screen.getByText('Bio text')).toBeInTheDocument();
  });

  it('renders social link icons with correct href and aria-label', async () => {
    renderLanding();

    await waitFor(() => {
      const link = screen.getByRole('link', { name: 'github' });
      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute('href', 'https://github.com/test');
      expect(link).toHaveAttribute('target', '_blank');
    });
  });

  it('commerce project links to /{locale}/project/{slug}', async () => {
    renderLanding();

    await waitFor(() => {
      expect(screen.getByText('App One')).toBeInTheDocument();
    });

    const card = document.querySelector('a.project-card');
    expect(card).toBeTruthy();
    expect(card?.getAttribute('href')).toBe('/de/project/my-app');
  });

  it('renders external project card with target="_blank"', async () => {
    mockGetProjects.mockResolvedValue([externalProject]);
    renderLanding();

    await waitFor(() => {
      expect(screen.getByText('Open Source Thing')).toBeInTheDocument();
    });

    const card = document.querySelector('a.project-card');
    expect(card).toBeTruthy();
    expect(card?.getAttribute('target')).toBe('_blank');
    expect(card?.getAttribute('href')).toBe('https://github.com/test/proj');
  });

  it('plain showcase without detail page renders as non-link card', async () => {
    mockGetProjects.mockResolvedValue([
      { ...commerceProject, commerce: undefined, has_detail_page: false, title: 'Showcase Only' },
    ]);
    renderLanding();

    await waitFor(() => {
      expect(screen.getByText('Showcase Only')).toBeInTheDocument();
    });

    const link = document.querySelector('a.project-card');
    expect(link).toBeNull();
    const card = document.querySelector('div.project-card');
    expect(card).toBeTruthy();
  });

  it('shows empty state when no projects', async () => {
    mockGetProjects.mockResolvedValue([]);
    renderLanding();

    await waitFor(() => {
      expect(screen.getByText('Projekte folgen bald')).toBeInTheDocument();
    });
  });

  it('shows discount pricing with strikethrough original price', async () => {
    const discounted: PublicProject = {
      ...commerceProject,
      commerce: {
        ...commerceProject.commerce!,
        discounted_price_cents: 499,
      },
    };
    mockGetProjects.mockResolvedValue([discounted]);
    renderLanding();

    await waitFor(() => {
      expect(screen.getByText('App One')).toBeInTheDocument();
    });

    const strikethrough = document.querySelector('.project-card-price-original');
    expect(strikethrough).toBeTruthy();
    const discountedEl = document.querySelector('.project-card-price-discount');
    expect(discountedEl).toBeTruthy();
  });
});
