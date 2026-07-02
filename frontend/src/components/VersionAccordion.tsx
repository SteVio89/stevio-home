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
  // Track open state per version id; first item defaults open
  const [openIds, setOpenIds] = useState<Set<string>>(new Set());

  useEffect(() => {
    setLoading(true);
    setError(false);
    setVersions(null);
    getProjectVersions(slug)
      .then((vs) => {
        setVersions(vs);
        // Default: first item open
        if (vs.length > 0) {
          setOpenIds(new Set([vs[0].id]));
        }
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

  function toggle(id: string) {
    setOpenIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
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
      {versions.map((v) => {
        const isOpen = openIds.has(v.id);
        return (
          <div key={v.id} className="version-accordion-item">
            <button
              className="version-accordion-header"
              aria-expanded={isOpen}
              aria-controls={`version-${v.id}-content`}
              onClick={() => toggle(v.id)}
            >
              <span>
                {v.version} — {formatDate(v.published_at)}
              </span>
              <span className={`version-accordion-chevron${isOpen ? ' open' : ''}`}>
                &#x25BC;
              </span>
            </button>
            <div
              id={`version-${v.id}-content`}
              role="region"
              className={`version-accordion-content${isOpen ? ' open' : ''}`}
            >
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
            </div>
          </div>
        );
      })}
    </div>
  );
}
