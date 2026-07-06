import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { getHero, getProjects, listPublicSocialLinks } from '../api/client';
import type { HeroContent, PublicProject, PublicSocialLink } from '../api/client';
import { useDocumentHead } from '../hooks/useDocumentHead';
import { useSiteConfig } from '../context/SiteConfigContext';
import { useLocale } from '../context/LocaleContext';
import { formatPrice } from '../utils/format';
import Skeleton from '../components/Skeleton';

// Platform → Font Awesome icon class mapping
const PLATFORM_ICONS: Record<string, string> = {
  github: 'fa-brands fa-github',
  gitlab: 'fa-brands fa-gitlab',
  codeberg: 'fa-solid fa-code-branch',
  twitter: 'fa-brands fa-x-twitter',
  mastodon: 'fa-brands fa-mastodon',
  bluesky: 'fa-brands fa-bluesky',
  linkedin: 'fa-brands fa-linkedin',
  xing: 'fa-brands fa-xing',
  twitch: 'fa-brands fa-twitch',
  youtube: 'fa-brands fa-youtube',
  steam: 'fa-brands fa-steam',
  playstation: 'fa-brands fa-playstation',
  discord: 'fa-brands fa-discord',
  reddit: 'fa-brands fa-reddit',
  instagram: 'fa-brands fa-instagram',
  facebook: 'fa-brands fa-facebook',
  tiktok: 'fa-brands fa-tiktok',
  email: 'fa-solid fa-envelope',
  website: 'fa-solid fa-globe',
};

function getPlatformIconClass(platform: string): string {
  return PLATFORM_ICONS[platform.toLowerCase()] ?? 'fa-solid fa-link';
}

interface ProjectCardContentProps {
  project: PublicProject;
  isExternal: boolean;
  t: (key: string) => string;
  currencySymbol: string;
}

function ProjectCardContent({ project, isExternal, t, currencySymbol }: ProjectCardContentProps) {
  const commerce = project.commerce;
  return (
    <>
      <div className="project-card-image">
        {project.image_url ? (
          <img src={project.image_url} alt={project.title} />
        ) : (
          <div className="project-card-image-fallback">?</div>
        )}
      </div>
      <div className="project-card-body">
        <h3 className="project-card-title">{project.title}</h3>
        <p className="project-card-tagline">{project.tagline}</p>
      </div>
      <div className="project-card-footer">
        {isExternal ? (
          <span className="project-card-external">↗ {t('landing.external_label')}</span>
        ) : commerce ? (
          commerce.purchase_mode === 'coming_soon' ? (
            <span className="project-card-coming-soon">{t('commerce.coming_soon')}</span>
          ) : commerce.price_cents === 0 ? (
            <span className="project-card-price">{t('commerce.get_free')}</span>
          ) : commerce.discounted_price_cents != null ? (
            <>
              <s className="project-card-price-original">
                {formatPrice(commerce.price_cents, currencySymbol)}
              </s>
              <span className="project-card-price-discount">
                {formatPrice(commerce.discounted_price_cents, currencySymbol)}
              </span>
            </>
          ) : (
            <span className="project-card-price">
              {formatPrice(commerce.price_cents, currencySymbol)}
            </span>
          )
        ) : null}
      </div>
    </>
  );
}

export default function Landing() {
  const [hero, setHero] = useState<HeroContent | null>(null);
  const [projects, setProjects] = useState<PublicProject[]>([]);
  const [socialLinks, setSocialLinks] = useState<PublicSocialLink[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const { currency_symbol: currencySymbol, base_url } = useSiteConfig();
  const { locale } = useLocale();
  const { t } = useTranslation();

  useDocumentHead({
    // Home page: no title → tab shows just the site name.
    canonical: base_url ? `${base_url}/${locale}/` : undefined,
  });

  useEffect(() => {
    Promise.all([getHero(), getProjects(), listPublicSocialLinks()])
      .then(([heroData, projectsData, socialLinksData]) => {
        setHero(heroData);
        setProjects(projectsData);
        setSocialLinks(socialLinksData);
        setError(false);
      })
      .catch(() => setError(true))
      .finally(() => setLoading(false));
  }, [locale]);

  return (
    <div className="page landing-page">
      {error && <p className="landing-error">{t('landing.load_error')}</p>}

      {/* Hero Section — always dark band */}
      <section className="landing-hero reveal-hidden">
        <div className="landing-hero-content">
          {hero && (
            <>
              <h1 className="landing-hero-headline">{hero.headline}</h1>
              <p className="landing-hero-tagline">{hero.tagline}</p>
              <p className="landing-hero-bio">{hero.bio}</p>
            </>
          )}
          {socialLinks.length > 0 && (
            <div className="landing-social-links">
              {socialLinks.map((link) => (
                <a
                  key={link.id}
                  href={link.url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="landing-social-link"
                  aria-label={link.platform}
                >
                  <i className={getPlatformIconClass(link.platform)} />
                </a>
              ))}
            </div>
          )}
        </div>
      </section>

      {/* Project Showcase Grid — no section heading per D-04 */}
      <section className="landing-projects reveal-hidden">
        <div className="landing-projects-grid">
          {loading ? (
            <>
              <Skeleton variant="card" />
              <Skeleton variant="card" />
              <Skeleton variant="card" />
            </>
          ) : projects.length === 0 ? (
            <div className="landing-projects-empty">
              <h2>{t('landing.projects_empty_heading')}</h2>
              <p>{t('landing.projects_empty_body')}</p>
            </div>
          ) : (
            projects.map((project) => {
              const isExternal = !!project.external_url;

              if (isExternal) {
                return (
                  <a
                    key={project.id}
                    href={project.external_url!}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="project-card"
                  >
                    <ProjectCardContent
                      project={project}
                      isExternal
                      t={t}
                      currencySymbol={currencySymbol}
                    />
                  </a>
                );
              }

              if (project.has_detail_page) {
                return (
                  <Link
                    key={project.id}
                    to={`/${locale}/project/${project.slug}`}
                    className="project-card"
                  >
                    <ProjectCardContent
                      project={project}
                      isExternal={false}
                      t={t}
                      currencySymbol={currencySymbol}
                    />
                  </Link>
                );
              }

              // Plain showcase without detail page — informational card, no link.
              return (
                <div key={project.id} className="project-card">
                  <ProjectCardContent
                    project={project}
                    isExternal={false}
                    t={t}
                    currencySymbol={currencySymbol}
                  />
                </div>
              );
            })
          )}
        </div>
      </section>

      {/* Contact CTA Band */}
      <section className="landing-cta reveal-hidden">
        <div className="landing-cta-content">
          <p>{t('landing.contact_intro')}</p>
          <Link to={`/${locale}/chat`} className="landing-cta-link">
            {t('landing.contact_cta')}
          </Link>
        </div>
      </section>
    </div>
  );
}
