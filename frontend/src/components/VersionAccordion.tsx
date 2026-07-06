import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { getProjectVersions, type AppVersion } from '../api/client';
import Skeleton from './Skeleton';

interface VersionAccordionProps {
  slug: string;
}

export function VersionAccordion({ slug }: VersionAccordionProps) {
  const { t, i18n } = useTranslation();
  const [versions, setVersions] = useState<AppVersion[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  // Reset happens via remount: the parent keys this component on `slug`, so a
  // slug change mounts a fresh instance with the initial loading/empty state.
  useEffect(() => {
    getProjectVersions(slug)
      .then((vs) => {
        setVersions(vs);
      })
      .catch(() => {
        setError(true);
      })
      .finally(() => {
        setLoading(false);
      });
  }, [slug]);

  if (loading) {
    return (
      <div className="version-accordion-loading">
        <Skeleton variant="text" />
        <Skeleton variant="text" />
        <Skeleton variant="text" />
      </div>
    );
  }

  if (error) {
    return <p className="section-error">{t('commerce.versions_unavailable')}</p>;
  }

  // Empty state: hide section entirely
  if (!versions || versions.length === 0) {
    return null;
  }

  function formatDate(iso: string): string {
    return new Date(iso).toLocaleDateString(i18n.language, {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    });
  }

  return (
    <div className="version-accordion">
      {versions.map((v, i) => (
        // First item defaults open via the native `open` attribute.
        <details key={v.id} className="version-accordion-item" open={i === 0}>
          <summary className="version-accordion-header">
            <span>
              {v.version} — {formatDate(v.published_at)}
            </span>
            <span className="version-accordion-chevron">&#x25BC;</span>
          </summary>
          <div className="version-accordion-content-inner">
            {v.release_notes ? (
              // Security: release_notes is backend-rendered HTML from markdown.ToHTML()
              // which outputs a restricted safe subset (headings, bold, lists, links,
              // paragraphs only). No client-side sanitization needed — same pattern
              // as legal pages. Source: UI-SPEC Markdown Rendering Contract.
              <div
                className="app-prose"
                dangerouslySetInnerHTML={{ __html: v.release_notes }}
              />
            ) : null}
          </div>
        </details>
      ))}
    </div>
  );
}
