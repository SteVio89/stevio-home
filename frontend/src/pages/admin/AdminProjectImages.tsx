import { useEffect, useRef, useState } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import {
  adminListProjects,
  adminListApps,
  getProjectDetail,
  adminUploadProjectGalleryImage,
  adminReorderProjectImages,
  adminDeleteProjectImage,
  adminGetProjectImageTranslations,
  adminUpsertProjectImageTranslation,
  type ProjectImage,
  APIError,
} from '../../api/client';
import PageHeader from '../../components/PageHeader';
import ConfirmModal from '../../components/ConfirmModal';
import Skeleton from '../../components/Skeleton';
import AdminProjectTabs from '../../components/AdminAppTabs';
import LocaleTabs from '../../components/LocaleTabs';
import { useAdminLocales } from '../../hooks/useAdminLocales';
import { useToast } from '../../context/ToastContext';

// AdminProjectImages manages a project's gallery images. Replaces AdminScreenshots
// (which was keyed by app id). Image rows are now attached to projects directly.
export default function AdminProjectImages() {
  const { id: projectId } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { addToast } = useToast();
  const { locales, loading: localesLoading } = useAdminLocales();
  const [projectTitle, setProjectTitle] = useState('');
  const [hasCommerce, setHasCommerce] = useState(false);
  const [images, setImages] = useState<ProjectImage[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Translation state: imageId -> locale -> { alt_text }
  const [translations, setTranslations] = useState<Record<string, Record<string, { alt_text: string }>>>({});
  const [activeLocales, setActiveLocales] = useState<Record<string, string>>({});
  const [savingAlt, setSavingAlt] = useState<string | null>(null);

  // Upload form
  const [altText, setAltText] = useState('');
  const [uploading, setUploading] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<ProjectImage | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  const defaultLocale = locales.find((l) => l.is_default)?.code ?? '';

  useEffect(() => {
    if (!projectId) return;
    Promise.all([adminListProjects(), adminListApps()])
      .then(async ([projects, apps]) => {
        const project = projects.find((p) => p.id === projectId);
        if (!project) {
          addToast('Project not found', 'error');
          navigate('/admin/projects');
          return;
        }
        setProjectTitle(project.title || '(untitled)');
        setHasCommerce(apps.some((a) => a.project_id === projectId && !a.deleted_at));

        // Public project detail returns images. We don't reuse the public endpoint
        // here because it 404s when has_detail_page=false, so use slug-based fetch
        // only as a fallback; otherwise list directly via translations endpoint.
        try {
          if (project.slug && project.has_detail_page) {
            const detail = await getProjectDetail(project.slug);
            setImages(detail.images ?? []);
          }
        } catch {
          // No detail page -- start with empty gallery; admin can still upload.
          setImages([]);
        }

        const tm = await adminGetProjectImageTranslations(projectId);
        setTranslations(tm);
      })
      .catch((err) => setError(err instanceof Error ? err.message : 'Failed to load'))
      .finally(() => setLoading(false));
  }, [projectId, addToast, navigate]);

  function getActiveLocale(imgId: string): string {
    return activeLocales[imgId] || defaultLocale;
  }

  function setActiveLocaleForImage(imgId: string, locale: string) {
    setActiveLocales((prev) => ({ ...prev, [imgId]: locale }));
  }

  function getAltText(imgId: string, locale: string): string {
    return translations[imgId]?.[locale]?.alt_text ?? '';
  }

  function updateAltText(imgId: string, locale: string, value: string) {
    setTranslations((prev) => ({
      ...prev,
      [imgId]: {
        ...(prev[imgId] ?? {}),
        [locale]: { alt_text: value },
      },
    }));
  }

  async function handleSaveAltText(imgId: string) {
    if (!projectId) return;
    const locale = getActiveLocale(imgId);
    const altTextValue = getAltText(imgId, locale);
    setSavingAlt(imgId);
    try {
      await adminUpsertProjectImageTranslation(projectId, imgId, locale, { alt_text: altTextValue });
      const label = locales.find((l) => l.code === locale)?.name ?? locale;
      addToast(`Alt text (${label}) saved.`, 'success');
    } catch (err) {
      addToast(err instanceof APIError ? err.message : 'Save failed', 'error');
    } finally {
      setSavingAlt(null);
    }
  }

  async function handleUpload(e: React.FormEvent) {
    e.preventDefault();
    const file = fileRef.current?.files?.[0];
    if (!file || !projectId) return;
    setUploading(true);
    setError('');
    try {
      const img = await adminUploadProjectGalleryImage(projectId, file, altText);
      setImages((prev) => [...prev, img]);
      // Seed translation state for the new image
      const newTrans: Record<string, { alt_text: string }> = {};
      locales.forEach((l) => {
        newTrans[l.code] = { alt_text: l.code === defaultLocale ? altText : '' };
      });
      setTranslations((prev) => ({ ...prev, [img.id]: newTrans }));
      setAltText('');
      if (fileRef.current) fileRef.current.value = '';
    } catch (err) {
      setError(err instanceof APIError ? err.message : 'Upload failed');
    } finally {
      setUploading(false);
    }
  }

  async function move(index: number, direction: -1 | 1) {
    if (!projectId) return;
    const next = [...images];
    const target = index + direction;
    if (target < 0 || target >= next.length) return;
    [next[index], next[target]] = [next[target], next[index]];

    const positions: Record<string, number> = {};
    next.forEach((s, i) => {
      positions[s.id] = i;
    });

    const previous = images;
    setImages(next);
    try {
      await adminReorderProjectImages(projectId, positions);
    } catch {
      setImages(previous);
    }
  }

  async function handleDelete(img: ProjectImage) {
    if (!projectId) return;
    try {
      await adminDeleteProjectImage(projectId, img.id);
      setImages((prev) => prev.filter((s) => s.id !== img.id));
      setTranslations((prev) => {
        const next = { ...prev };
        delete next[img.id];
        return next;
      });
    } catch (err) {
      setError(err instanceof APIError ? err.message : 'Delete failed');
    }
    setDeleteTarget(null);
  }

  if (loading || localesLoading)
    return (
      <div className="page">
        <PageHeader title="Images" />
        <Skeleton variant="text" width="40%" />
        <Skeleton variant="card" height="200px" count={3} />
      </div>
    );

  if (!projectId) {
    return (
      <div className="page">
        <PageHeader title="Images" />
        <p className="error">{error || 'Missing project ID.'}</p>
      </div>
    );
  }

  return (
    <div className="page">
      <PageHeader title="Images" />
      <AdminProjectTabs projectId={projectId} projectTitle={projectTitle} hasCommerce={hasCommerce} />

      {error && <p className="error">{error}</p>}

      {deleteTarget && (
        <ConfirmModal
          title="Delete Image"
          message={`Delete image "${getAltText(deleteTarget.id, defaultLocale) || 'this image'}"? This cannot be undone.`}
          confirmLabel="Delete"
          danger
          onConfirm={() => handleDelete(deleteTarget)}
          onCancel={() => setDeleteTarget(null)}
        />
      )}

      {images.length > 0 && (
        <div className="admin-section">
          <h2>Current Images</h2>
          <div className="screenshot-admin-grid">
            {images.map((img, i) => {
              const imgLocale = getActiveLocale(img.id);
              return (
                <div key={img.id} className="screenshot-admin-item">
                  <img src={img.url} alt={getAltText(img.id, defaultLocale)} />
                  <div className="screenshot-admin-controls">
                    <div className="screenshot-admin-move">
                      <button
                        className="btn btn-secondary btn-small"
                        onClick={() => move(i, -1)}
                        disabled={i === 0}
                        title="Move left"
                      >
                        ←
                      </button>
                      <button
                        className="btn btn-secondary btn-small"
                        onClick={() => move(i, 1)}
                        disabled={i === images.length - 1}
                        title="Move right"
                      >
                        →
                      </button>
                    </div>
                    <button
                      className="btn btn-danger btn-small"
                      onClick={() => setDeleteTarget(img)}
                      title="Delete"
                    >
                      ✕
                    </button>
                  </div>
                  {locales.length > 1 && (
                    <LocaleTabs
                      locales={locales}
                      activeLocale={imgLocale}
                      onChange={(loc) => setActiveLocaleForImage(img.id, loc)}
                    />
                  )}
                  <div className="form-group">
                    <label htmlFor={`alt-${img.id}`}>
                      Alt text{locales.length > 1 ? ` (${imgLocale})` : ''}
                    </label>
                    <input
                      id={`alt-${img.id}`}
                      type="text"
                      value={getAltText(img.id, imgLocale)}
                      onChange={(e) => updateAltText(img.id, imgLocale, e.target.value)}
                      placeholder="Brief description of the image"
                    />
                  </div>
                  <button
                    className="btn btn-secondary btn-small"
                    onClick={() => handleSaveAltText(img.id)}
                    disabled={savingAlt === img.id}
                  >
                    {savingAlt === img.id ? 'Saving...' : 'Save alt text'}
                  </button>
                </div>
              );
            })}
          </div>
        </div>
      )}

      <div className="admin-section">
        <h2>Upload Image</h2>
        <form onSubmit={handleUpload}>
          <div className="form-group">
            <label htmlFor="image-file">Image file (PNG, JPG, WebP) *</label>
            <input
              id="image-file"
              ref={fileRef}
              type="file"
              accept=".png,.jpg,.jpeg,.webp"
              required
            />
          </div>
          <div className="form-group">
            <label htmlFor="alt-text">Alt text (default language)</label>
            <input
              id="alt-text"
              type="text"
              value={altText}
              onChange={(e) => setAltText(e.target.value)}
              placeholder="Brief description of the image"
            />
          </div>
          <div className="form-actions">
            <button type="submit" className="btn btn-primary" disabled={uploading}>
              {uploading ? 'Uploading...' : 'Upload Image'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
