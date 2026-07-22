import { useEffect, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  getProjectDetail,
  getOwnership,
  validateDiscountCode,
  getAutoDiscount,
  createCheckoutSession,
  type ProjectDetail as ProjectDetailType,
  type DiscountValidation,
  type OwnershipStatus,
  APIError,
} from '../api/client';
import { useDocumentHead } from '../hooks/useDocumentHead';
import { useAuth } from '../context/AuthContext';
import { useSiteConfig } from '../context/SiteConfigContext';
import { useLocale } from '../context/LocaleContext';
import { isSafeCheckoutURL, isSafeExternalCheckoutURL } from '../utils/safeUrl';
import { getPaddle, setPaddleEventHandler } from '../utils/paddle';
import { CheckoutEventNames } from '@paddle/paddle-js';
import WithdrawalConsentModal from '../components/WithdrawalConsentModal';
import { ErrorBoundary } from '../components/ErrorBoundary';
import { PurchaseSidebar } from '../components/PurchaseSidebar';
import { VersionAccordion } from '../components/VersionAccordion';
import Skeleton from '../components/Skeleton';

export default function ProjectDetail() {
  const { slug } = useParams<{ slug: string }>();
  const [project, setProject] = useState<ProjectDetailType | null>(null);
  const [loading, setLoading] = useState(true);
  const [activeShot, setActiveShot] = useState(0);
  const [discountCode, setDiscountCode] = useState('');
  const [discount, setDiscount] = useState<DiscountValidation | null>(null);
  const [autoDiscount, setAutoDiscount] = useState<DiscountValidation | null>(null);
  const [discountError, setDiscountError] = useState('');
  const [discountLoading, setDiscountLoading] = useState(false);
  const [checkoutLoading, setCheckoutLoading] = useState(false);
  const [checkoutError, setCheckoutError] = useState('');
  const [showConsentModal, setShowConsentModal] = useState(false);
  const [ownership, setOwnership] = useState<OwnershipStatus | null>(null);
  const { loggedIn } = useAuth();
  const navigate = useNavigate();
  const { currency_code, base_url, payment_provider, paddle_client_token, paddle_environment } = useSiteConfig();
  const { locale } = useLocale();
  const { t } = useTranslation();

  const commerce = project?.commerce ?? null;

  useDocumentHead({
    title: project ? project.title : t('project.details'),
    description: project?.tagline || undefined,
    canonical: project && base_url ? `${base_url}/${locale}/project/${project.slug}` : undefined,
    ogImage: project?.image_url && base_url ? `${base_url}${project.image_url}` : undefined,
  });

  // JSON-LD structured data for search engines
  useEffect(() => {
    if (!project || !base_url) return;
    const script = document.createElement('script');
    script.type = 'application/ld+json';
    script.id = 'project-jsonld';
    const priceCents = commerce?.price_cents ?? 0;
    script.textContent = JSON.stringify({
      '@context': 'https://schema.org',
      '@type': 'SoftwareApplication',
      name: project.title,
      description: project.description || project.tagline,
      operatingSystem: 'macOS',
      applicationCategory: 'Utilities',
      ...(priceCents > 0
        ? {
            offers: {
              '@type': 'Offer',
              price: (priceCents / 100).toFixed(2),
              priceCurrency: currency_code,
            },
          }
        : {}),
      ...(project.image_url ? { image: `${base_url}${project.image_url}` } : {}),
      url: `${base_url}/${locale}/project/${project.slug}`,
    });
    document.head.appendChild(script);
    return () => {
      document.getElementById('project-jsonld')?.remove();
    };
  }, [project, commerce, currency_code, base_url, locale]);

  useEffect(() => {
    if (!slug) return;
    setActiveShot(0);
    setDiscount(null);
    setAutoDiscount(null);
    setDiscountCode('');
    setDiscountError('');
    getProjectDetail(slug)
      .then(async (p) => {
        setProject(p);
        const c = p.commerce;
        if (c && c.price_cents > 0) {
          try {
            const ad = await getAutoDiscount(c.id);
            setAutoDiscount(ad);
          } catch {
            // No active auto-discount for this app
          }
        }
        if (loggedIn && c && c.purchase_mode !== 'always_new_license') {
          getOwnership(c.id)
            .then(setOwnership)
            .catch(() => setOwnership(null));
        } else {
          setOwnership(null);
        }
      })
      .catch(() => {
        // project stays null -- renders "not found".
      })
      .finally(() => setLoading(false));
  }, [slug, loggedIn, locale]);

  async function handleApplyDiscount(ev: React.FormEvent) {
    ev.preventDefault();
    if (!commerce || !discountCode.trim()) return;
    setDiscountLoading(true);
    setDiscountError('');
    setDiscount(null);
    try {
      const result = await validateDiscountCode(discountCode.trim(), commerce.id);
      setDiscount(result);
    } catch (err) {
      if (err instanceof APIError && err.code === 'discount_invalid') {
        setDiscountError(t('commerce.discount_invalid'));
      } else {
        setDiscountError(t('commerce.discount_error'));
      }
    } finally {
      setDiscountLoading(false);
    }
  }

  function handleBuy() {
    if (!commerce) return;
    setShowConsentModal(true);
  }

  async function handleConsentConfirm() {
    if (!commerce) return;
    setShowConsentModal(false);
    setCheckoutLoading(true);
    setCheckoutError('');
    try {
      const codeToSend = discount ? discountCode.trim() : '';
      const consentTimestamp = new Date().toISOString();
      const session = await createCheckoutSession(commerce.id, codeToSend, consentTimestamp);

      if (payment_provider === 'paddle') {
        // Paddle Billing: open the in-page overlay for the created transaction.
        if (!paddle_client_token || !session.transaction_id) {
          setCheckoutError(t('commerce.checkout_error'));
          return;
        }
        const paddle = await getPaddle(paddle_client_token, paddle_environment);
        if (!paddle) {
          setCheckoutError(t('commerce.checkout_error'));
          return;
        }
        // Fulfillment happens via webhook; the success page verifies by session_id.
        setPaddleEventHandler((e) => {
          if (e.name === CheckoutEventNames.CHECKOUT_COMPLETED) {
            setPaddleEventHandler(null);
            paddle.Checkout.close();
            navigate(`/${locale}/success?session_id=${encodeURIComponent(session.session_id)}`);
          }
        });
        paddle.Checkout.open({ transactionId: session.transaction_id });
        return;
      }

      if (payment_provider === 'polar') {
        // Polar: redirect to its hosted checkout. Fulfillment happens via
        // webhook; the success page verifies by session_id.
        if (isSafeExternalCheckoutURL(session.url)) {
          window.location.href = session.url;
        } else {
          setCheckoutError(t('commerce.checkout_error'));
        }
        return;
      }

      // Mock provider: redirect to its simulated payment page.
      if (isSafeCheckoutURL(session.url)) {
        window.location.href = session.url;
      } else {
        setCheckoutError(t('commerce.checkout_invalid_url'));
      }
    } catch (err) {
      if (err instanceof APIError && err.code === 'login_required') {
        navigate(`/${locale}/login`);
      } else if (err instanceof APIError) {
        setCheckoutError(err.message);
      } else {
        setCheckoutError(t('commerce.checkout_error'));
      }
    } finally {
      setCheckoutLoading(false);
    }
  }

  if (loading) {
    return (
      <div className="app-detail" aria-hidden="true">
        <div className="app-detail-content">
          <Skeleton variant="card" />
          <Skeleton variant="text" />
          <Skeleton variant="text" />
          <Skeleton variant="text" />
        </div>
        <div className="app-detail-sidebar-wrap">
          <div className="purchase-sidebar">
            <Skeleton variant="text" />
            <Skeleton variant="text" />
          </div>
        </div>
      </div>
    );
  }

  if (!project) {
    return (
      <div className="page">
        <p className="error">{t('project.not_found')}</p>
        <Link to={`/${locale}/`} className="btn btn-secondary back-link">
          {t('not_found.back')}
        </Link>
      </div>
    );
  }

  const images = project.images ?? [];

  // Security: description, alt_text, system_requirements and release_notes are
  // backend-rendered HTML from markdown.ToHTML() which outputs a restricted safe
  // subset (headings, bold, lists, links, paragraphs only). No client-side
  // sanitization needed — same pattern as legal pages and version notes. Source:
  // UI-SPEC Markdown Rendering Contract.
  return (
    <div className="page">
      <Link to={`/${locale}/`} className="back-link">{t('project.back')}</Link>

      <div className="app-detail">
        <div className="app-detail-content">
          <div className="app-detail-header">
            {project.image_url && (
              <img src={project.image_url} alt={project.title} className="app-icon-large" />
            )}
            <div className="app-detail-meta">
              <h1>{project.title}</h1>
              <p className="app-tagline">{project.tagline}</p>
            </div>
          </div>

          <ErrorBoundary fallback={<p className="section-error">{t('error.section_crash')}</p>}>
            {images.length > 0 && (
              <section className="screenshot-gallery">
                <div className="screenshot-main">
                  <img
                    src={images[activeShot].url}
                    alt={images[activeShot].alt_text || project.title}
                  />
                </div>
                {images.length > 1 && (
                  <div className="screenshot-thumbs">
                    {images.map((s, i) => (
                      <button
                        key={s.id}
                        className={`screenshot-thumb${i === activeShot ? ' active' : ''}`}
                        onClick={() => setActiveShot(i)}
                        aria-label={t('project.screenshot', { n: i + 1 })}
                      >
                        <img src={s.url} alt={s.alt_text || ''} />
                      </button>
                    ))}
                  </div>
                )}
              </section>
            )}

            {project.description && (
              <section className="app-detail-section">
                <div
                  className="app-prose"
                  dangerouslySetInnerHTML={{ __html: project.description }}
                />
              </section>
            )}

            {project.system_requirements && (
              <section className="app-detail-section">
                <h2>{t('commerce.system_requirements')}</h2>
                <div
                  className="app-prose"
                  dangerouslySetInnerHTML={{ __html: project.system_requirements }}
                />
              </section>
            )}

            {commerce && (
              <section className="app-detail-section">
                <h2>{t('commerce.version_history')}</h2>
                <VersionAccordion key={project.slug} slug={project.slug} />
              </section>
            )}
          </ErrorBoundary>
        </div>

        {commerce && (
          <div className="app-detail-sidebar-wrap">
            <ErrorBoundary fallback={<p className="section-error">{t('error.sidebar_crash')}</p>}>
              <PurchaseSidebar
                project={project}
                commerce={commerce}
                locale={locale}
                ownership={ownership}
                checkout={{
                  loading: checkoutLoading,
                  error: checkoutError,
                  onBuy: handleBuy,
                }}
                discountState={{
                  code: discountCode,
                  applied: discount,
                  auto: autoDiscount,
                  error: discountError,
                  loading: discountLoading,
                  onChange: setDiscountCode,
                  onApply: handleApplyDiscount,
                }}
              />
            </ErrorBoundary>
          </div>
        )}
      </div>

      {showConsentModal && (
        <WithdrawalConsentModal
          onConfirm={handleConsentConfirm}
          onCancel={() => setShowConsentModal(false)}
        />
      )}
    </div>
  );
}
