import { useCallback, useEffect, useRef, useState } from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import {
  adminCreateProject,
  adminUpdateProject,
  adminUploadProjectImage,
  adminGetProjectTranslations,
  adminUpsertProjectTranslation,
  adminListProjects,
  adminListApps,
  adminAttachCommerce,
  adminDetachCommerce,
  adminUpdateApp,
  adminGetAppTranslations,
  adminUpsertAppTranslation,
  APIError,
  type ProjectTranslation,
  type AdminProject,
} from '../../api/client';
import PageHeader from '../../components/PageHeader';
import LocaleTabs from '../../components/LocaleTabs';
import AdminProjectTabs from '../../components/AdminAppTabs';
import { useAdminLocales } from '../../hooks/useAdminLocales';
import { useToast } from '../../context/ToastContext';
import { DEFAULT_LOCALE } from '../../i18n';

type ProjectFlavor = 'external' | 'showcase' | 'commerce';

// Combined per-locale fields edited in this form: project text + (when commerce
// attached) the system_requirements overlay on the app entity.
interface LocaleFields extends ProjectTranslation {
  system_requirements: string;
}

const PURCHASE_MODES = [
  { value: 'always_new_license', label: 'Always New License (default)' },
  { value: 'one_time_only', label: 'One-Time Only — block repeat purchases' },
  { value: 'install_plus', label: 'Install+ — re-purchase adds activation slots' },
  { value: 'coming_soon', label: 'Coming Soon — preview only, no purchases' },
];

// Paddle tax categories (must match backend queries.IsValidTaxCategory).
// "standard" is Paddle's "Standard digital goods" — the pre-approved Default
// category for downloadable software (what we sell). The other categories,
// including the narrow "digital-goods" (non-software media files), must be
// explicitly requested and approved in the Paddle dashboard before use.
const TAX_CATEGORIES = [
  { value: 'standard', label: 'Standard Digital Goods (default)' },
  { value: 'ebooks', label: 'eBooks' },
  { value: 'saas', label: 'SaaS' },
  { value: 'software-programming-services', label: 'Software / Programming Services' },
  { value: 'digital-goods', label: 'Digital Goods (media files, not software)' },
  { value: 'professional-services', label: 'Professional Services' },
  { value: 'implementation-services', label: 'Implementation Services' },
  { value: 'training-services', label: 'Training Services' },
  { value: 'website-hosting', label: 'Website Hosting' },
];

function emptyTranslation(): LocaleFields {
  return { title: '', tagline: '', description: '', system_requirements: '' };
}

function slugify(value: string): string {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9\s-]/g, '')
    .replace(/\s+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '');
}

export default function AdminProjectForm() {
  const { id } = useParams<{ id: string }>();
  const isEdit = Boolean(id);
  const navigate = useNavigate();
  const { addToast } = useToast();
  const { locales, loading: localesLoading } = useAdminLocales();

  const [activeLocale, setActiveLocale] = useState('');
  const [flavor, setFlavor] = useState<ProjectFlavor>('showcase');
  const [externalUrl, setExternalUrl] = useState('');
  const [hasDetailPage, setHasDetailPage] = useState(false);
  const [slug, setSlug] = useState('');
  const [slugManuallyEdited, setSlugManuallyEdited] = useState(false);
  const [position, setPosition] = useState<number | undefined>(undefined);
  const [imageUrl, setImageUrl] = useState('');
  const [imageFile, setImageFile] = useState<File | null>(null);
  const [imagePreview, setImagePreview] = useState('');

  // Commerce fields
  const [bundleId, setBundleId] = useState('');
  const [priceCents, setPriceCents] = useState('');
  const [purchaseMode, setPurchaseMode] = useState('always_new_license');
  const [taxCategory, setTaxCategory] = useState('standard');
  const [appId, setAppId] = useState<string | null>(null); // populated on edit when commerce exists
  const [originalFlavor, setOriginalFlavor] = useState<ProjectFlavor>('showcase');

  const [translations, setTranslations] = useState<Record<string, LocaleFields>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [project, setProject] = useState<AdminProject | null>(null);
  const objectUrlRef = useRef('');

  const defaultLocale = locales.find((l) => l.is_default)?.code ?? locales[0]?.code ?? DEFAULT_LOCALE;

  useEffect(() => {
    if (locales.length > 0 && !activeLocale) {
      setActiveLocale(defaultLocale);
    }
  }, [locales, activeLocale, defaultLocale]);

  // Cleanup objectURL on unmount
  useEffect(() => {
    return () => {
      if (objectUrlRef.current) URL.revokeObjectURL(objectUrlRef.current);
    };
  }, []);

  const handleImageChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (objectUrlRef.current) URL.revokeObjectURL(objectUrlRef.current);
    const url = URL.createObjectURL(file);
    objectUrlRef.current = url;
    setImageFile(file);
    setImagePreview(url);
  }, []);

  function handleTitleChange(value: string) {
    updateTranslation('title', value);
    if (!isEdit && !slugManuallyEdited && activeLocale === defaultLocale) {
      setSlug(slugify(value));
    }
  }

  function updateTranslation(field: keyof LocaleFields, value: string) {
    setTranslations((prev) => ({
      ...prev,
      [activeLocale]: {
        ...(prev[activeLocale] ?? emptyTranslation()),
        [field]: value,
      },
    }));
  }

  // Load existing project + translations
  useEffect(() => {
    async function load() {
      try {
        if (isEdit && id) {
          const [allProjects, allApps] = await Promise.all([adminListProjects(), adminListApps()]);
          const proj = allProjects.find((p) => p.id === id);
          if (!proj) {
            addToast('Project not found', 'error');
            navigate('/admin/projects');
            return;
          }
          setProject(proj);
          setSlug(proj.slug ?? '');
          setSlugManuallyEdited(true);
          setPosition(proj.position);
          setImageUrl(proj.image_url ?? '');
          if (proj.image_url) setImagePreview(proj.image_url);
          setHasDetailPage(proj.has_detail_page);

          const commerceApp = allApps.find((a) => a.project_id === id && !a.deleted_at);
          let computedFlavor: ProjectFlavor;
          if (proj.external_url) {
            computedFlavor = 'external';
            setExternalUrl(proj.external_url);
          } else if (commerceApp) {
            computedFlavor = 'commerce';
            setAppId(commerceApp.id);
            setBundleId(commerceApp.bundle_id);
            setPriceCents(String(commerceApp.price_cents));
            setPurchaseMode(commerceApp.purchase_mode || 'always_new_license');
            setTaxCategory(commerceApp.tax_category || 'standard');
          } else {
            computedFlavor = 'showcase';
          }
          setFlavor(computedFlavor);
          setOriginalFlavor(computedFlavor);

          if (locales.length > 0) {
            const projTransByLocale: Record<string, ProjectTranslation> = {};
            await Promise.all(
              locales.map(async (loc) => {
                try {
                  projTransByLocale[loc.code] = await adminGetProjectTranslations(id, loc.code);
                } catch {
                  projTransByLocale[loc.code] = { title: '', tagline: '', description: '' };
                }
              }),
            );
            // Load app system_requirements translations if commerce present
            let appTransByLocale: Record<string, { system_requirements: string }> = {};
            if (commerceApp) {
              try {
                const raw = await adminGetAppTranslations(commerceApp.id);
                appTransByLocale = raw;
              } catch {
                // Keep empty.
              }
            }
            const merged: Record<string, LocaleFields> = {};
            for (const loc of locales) {
              merged[loc.code] = {
                ...projTransByLocale[loc.code],
                system_requirements: appTransByLocale[loc.code]?.system_requirements ?? '',
              };
            }
            setTranslations(merged);
          }
        }
      } catch (err) {
        addToast(err instanceof APIError ? err.message : 'Failed to load data', 'error');
      } finally {
        setLoading(false);
      }
    }
    if (locales.length > 0) load();
  }, [id, isEdit, locales, addToast, navigate]);

  // Invariants enforced in UI to keep state coherent with backend rules.
  useEffect(() => {
    if (flavor === 'external') {
      // External URL forces detail page off.
      setHasDetailPage(false);
    } else if (flavor === 'commerce') {
      // Commerce forces detail page on.
      setHasDetailPage(true);
    }
  }, [flavor]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();

    const defaultTrans = translations[defaultLocale] ?? emptyTranslation();
    if (!defaultTrans.title.trim()) {
      addToast('Title is required for the default locale', 'error');
      return;
    }

    if (flavor === 'external') {
      if (!externalUrl.startsWith('https://')) {
        addToast('External URL must start with https://', 'error');
        return;
      }
    }
    if (flavor === 'commerce' && !isEdit && !bundleId.trim()) {
      addToast('Bundle ID is required when creating a commerce product', 'error');
      return;
    }

    setSaving(true);
    try {
      let savedId = id;
      const externalForApi = flavor === 'external' ? externalUrl : '';
      const projectBody = {
        slug: slug || undefined,
        external_url: externalForApi || null,
        image_url: imageUrl || undefined,
        has_detail_page: hasDetailPage,
        title: defaultTrans.title,
        tagline: defaultTrans.tagline,
        description: defaultTrans.description,
      };

      if (isEdit && id) {
        await adminUpdateProject(id, projectBody);
      } else {
        const created = await adminCreateProject({
          ...projectBody,
          ...(flavor === 'commerce'
            ? {
                commerce: {
                  bundle_id: bundleId.trim(),
                  price_cents: parseInt(priceCents || '0', 10),
                  purchase_mode: purchaseMode,
                  tax_category: taxCategory,
                },
              }
            : {}),
        });
        savedId = created.id;
        if (created.commerce) {
          setAppId(created.commerce.id);
        }
      }

      // Upload image if file selected (after create so we have an ID)
      if (imageFile && savedId) {
        const result = await adminUploadProjectImage(savedId, imageFile);
        if (!isEdit) {
          await adminUpdateProject(savedId, {
            ...projectBody,
            image_url: result.image_url,
          });
        }
      }

      // Handle commerce attach/detach/update transitions on edit.
      if (isEdit && savedId) {
        if (flavor === 'commerce' && originalFlavor !== 'commerce') {
          const created = await adminAttachCommerce(savedId, {
            bundle_id: bundleId.trim(),
            price_cents: parseInt(priceCents || '0', 10),
            purchase_mode: purchaseMode,
            tax_category: taxCategory,
          });
          setAppId(created.id);
        } else if (flavor !== 'commerce' && originalFlavor === 'commerce') {
          await adminDetachCommerce(savedId);
        } else if (flavor === 'commerce' && appId) {
          await adminUpdateApp(appId, {
            price_cents: parseInt(priceCents || '0', 10),
            purchase_mode: purchaseMode,
            tax_category: taxCategory,
          });
        }
      }

      // Save project translations for non-default locales (default is upserted by
      // create/update via the body fields).
      if (savedId) {
        const localesToSave = isEdit ? locales : locales.filter((l) => l.code !== defaultLocale);
        if (localesToSave.length > 0) {
          await Promise.allSettled(
            localesToSave.map((loc) => {
              const trans = translations[loc.code] ?? emptyTranslation();
              return adminUpsertProjectTranslation(savedId!, loc.code, {
                title: trans.title,
                tagline: trans.tagline,
                description: trans.description,
              });
            }),
          );
        }

        // Save commerce system_requirements translations if commerce present.
        const commerceAppId = flavor === 'commerce' ? appId : null;
        if (commerceAppId) {
          await Promise.allSettled(
            locales.map((loc) => {
              const trans = translations[loc.code] ?? emptyTranslation();
              return adminUpsertAppTranslation(commerceAppId, loc.code, {
                system_requirements: trans.system_requirements,
              });
            }),
          );
        }
      }

      addToast(isEdit ? 'Project updated' : 'Project created', 'success');
      navigate('/admin/projects');
    } catch (err) {
      addToast(err instanceof APIError ? err.message : 'Save failed', 'error');
    } finally {
      setSaving(false);
    }
  }

  if (loading || localesLoading) {
    return (
      <div className="page">
        <PageHeader
          title={isEdit ? 'Edit Project' : 'New Project'}
          backTo="/admin/projects"
          backLabel="Projects"
        />
        <p>Loading...</p>
      </div>
    );
  }

  const current = translations[activeLocale] ?? emptyTranslation();
  const showCommerceSection = flavor === 'commerce';
  const showDetailToggle = flavor === 'showcase';

  return (
    <div className="page">
      <PageHeader
        title={isEdit ? 'Edit Project' : 'New Project'}
        backTo="/admin/projects"
        backLabel="Projects"
      />
      {isEdit && id && project && (
        <AdminProjectTabs
          projectId={id}
          projectTitle={project.title || '(untitled)'}
          hasCommerce={flavor === 'commerce'}
        />
      )}
      <form onSubmit={handleSubmit} className="admin-form">
        {/* Type toggle */}
        <div className="admin-type-toggle">
          <label>
            <input
              type="radio"
              name="projectFlavor"
              value="external"
              checked={flavor === 'external'}
              onChange={() => setFlavor('external')}
            />
            {' '}External link
          </label>
          <label>
            <input
              type="radio"
              name="projectFlavor"
              value="showcase"
              checked={flavor === 'showcase'}
              onChange={() => setFlavor('showcase')}
            />
            {' '}Plain showcase
          </label>
          <label>
            <input
              type="radio"
              name="projectFlavor"
              value="commerce"
              checked={flavor === 'commerce'}
              onChange={() => setFlavor('commerce')}
            />
            {' '}Commerce product
          </label>
        </div>

        {flavor === 'external' && (
          <div className="form-group">
            <label htmlFor="external-url">External URL</label>
            <input
              id="external-url"
              type="url"
              placeholder="https://"
              value={externalUrl}
              onChange={(e) => setExternalUrl(e.target.value)}
              required
            />
            <p className="form-hint">
              Visitors are sent here directly. No detail page is shown.
            </p>
          </div>
        )}

        {showDetailToggle && (
          <div className="form-group form-group-checkbox">
            <label htmlFor="has-detail-page">
              <input
                id="has-detail-page"
                type="checkbox"
                checked={hasDetailPage}
                onChange={(e) => setHasDetailPage(e.target.checked)}
              />
              {' '}Show detail page
            </label>
            <p className="admin-section-description">
              When checked, the project links to its own page (with description and gallery).
              When unchecked, it appears as an info-only card on Landing.
            </p>
          </div>
        )}

        {/* Slug, image, position */}
        <div className="form-group">
          <label htmlFor="project-slug">URL slug</label>
          <input
            id="project-slug"
            type="text"
            value={slug}
            onChange={(e) => {
              setSlug(e.target.value);
              setSlugManuallyEdited(true);
            }}
            placeholder="my-project"
          />
          <p className="form-hint">
            Used in the public URL: /:locale/project/<strong>{slug || 'my-project'}</strong>.
            Auto-generated from title if left blank on create.
          </p>
        </div>

        <div className="form-group">
          <label htmlFor="project-image">Project Image</label>
          <input
            id="project-image"
            type="file"
            accept="image/*"
            onChange={handleImageChange}
          />
          {imagePreview ? (
            <img src={imagePreview} alt="" className="admin-project-image-preview" />
          ) : (
            <div style={{
              width: 160,
              height: 120,
              border: '2px dashed var(--color-border)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              marginTop: '0.5rem',
              borderRadius: 'var(--radius)',
              color: 'var(--color-text-muted)',
              fontSize: '0.875rem',
            }}>
              No image
            </div>
          )}
        </div>

        {position !== undefined && (
          <div className="form-group">
            <label>Position</label>
            <p className="form-hint">{position} (use the up/down arrows on the project list to reorder)</p>
          </div>
        )}

        {/* Commerce section */}
        {showCommerceSection && (
          <fieldset className="admin-section">
            <legend>Commerce</legend>
            <div className="form-group">
              <label htmlFor="bundle-id">Bundle ID (e.g. com.yourname.app) *</label>
              {isEdit && originalFlavor === 'commerce' ? (
                <code className="code-block">{bundleId}</code>
              ) : (
                <input
                  id="bundle-id"
                  type="text"
                  value={bundleId}
                  onChange={(e) => setBundleId(e.target.value)}
                  placeholder="com.yourname.app"
                  required={!isEdit}
                />
              )}
              <p className="form-hint">
                Used by the SDK to identify this app. Cannot be changed after attach.
              </p>
            </div>
            <div className="form-group">
              <label htmlFor="price-cents">Price (in cents, e.g. 2900 = 29.00)</label>
              <input
                id="price-cents"
                type="number"
                min="0"
                value={priceCents}
                onChange={(e) => setPriceCents(e.target.value)}
                placeholder="2900"
              />
            </div>
            <div className="form-group">
              <label htmlFor="purchase-mode">Purchase Mode</label>
              <select
                id="purchase-mode"
                value={purchaseMode}
                onChange={(e) => setPurchaseMode(e.target.value)}
              >
                {PURCHASE_MODES.map((m) => (
                  <option key={m.value} value={m.value}>{m.label}</option>
                ))}
              </select>
              <p className="form-hint">
                One-Time Only and Install+ require the buyer to be logged in.
              </p>
            </div>
            <div className="form-group">
              <label htmlFor="tax-category">Tax Category</label>
              <select
                id="tax-category"
                value={taxCategory}
                onChange={(e) => setTaxCategory(e.target.value)}
              >
                {TAX_CATEGORIES.map((t) => (
                  <option key={t.value} value={t.value}>{t.label}</option>
                ))}
              </select>
              <p className="form-hint">
                Passed to Paddle per-checkout. Non-default categories must be enabled
                in your Paddle dashboard. Ignored by the Mock provider.
              </p>
            </div>
            {isEdit && appId && (
              <p className="form-hint">
                Manage gallery images and versions via the tabs above.
              </p>
            )}
          </fieldset>
        )}

        {/* Translatable fields */}
        <LocaleTabs locales={locales} activeLocale={activeLocale} onChange={setActiveLocale} />

        <div className="form-group">
          <label htmlFor="project-title">
            Title{' '}
            {activeLocale === defaultLocale && (
              <span style={{ color: 'var(--color-text-muted)' }}>(required)</span>
            )}
          </label>
          <input
            id="project-title"
            type="text"
            value={current.title}
            onChange={(e) => handleTitleChange(e.target.value)}
            required={activeLocale === defaultLocale}
          />
        </div>

        <div className="form-group">
          <label htmlFor="project-tagline">Tagline</label>
          <input
            id="project-tagline"
            type="text"
            value={current.tagline}
            onChange={(e) => updateTranslation('tagline', e.target.value)}
          />
        </div>

        <div className="form-group">
          <label htmlFor="project-description">Description</label>
          <textarea
            id="project-description"
            rows={5}
            value={current.description}
            onChange={(e) => updateTranslation('description', e.target.value)}
            placeholder="Full description shown on the detail page (Markdown supported)"
          />
        </div>

        {showCommerceSection && (
          <div className="form-group">
            <label htmlFor="system-requirements">System Requirements</label>
            <textarea
              id="system-requirements"
              rows={4}
              value={current.system_requirements}
              onChange={(e) => updateTranslation('system_requirements', e.target.value)}
              placeholder={'- macOS 14.0+\n- Apple Silicon or Intel\n- 100 MB disk space'}
            />
            <small className="form-help">Shown on the detail page when commerce is attached. Markdown supported.</small>
          </div>
        )}

        <div className="form-actions">
          <button type="submit" className="btn btn-primary" disabled={saving}>
            {saving ? 'Saving…' : 'Save Project'}
          </button>
          <Link to="/admin/projects" className="btn btn-secondary">Cancel</Link>
        </div>
      </form>
    </div>
  );
}
