import { useEffect, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  adminListProjects,
  adminListApps,
  adminListVersions,
  adminCreateVersion,
  adminUploadBinary,
  adminGetVersionTranslations,
  adminUpsertVersionTranslation,
  type AppVersion,
  type AdminAppListItem,
  APIError,
} from "../../api/client";
import PageHeader from "../../components/PageHeader";
import { SkeletonTable } from "../../components/Skeleton";
import AdminProjectTabs from "../../components/AdminAppTabs";
import LocaleTabs from "../../components/LocaleTabs";
import { useAdminLocales } from "../../hooks/useAdminLocales";
import { useToast } from "../../context/ToastContext";

export default function AdminVersions() {
  const { id: projectId } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { addToast } = useToast();
  const { locales, loading: localesLoading } = useAdminLocales();
  const [projectTitle, setProjectTitle] = useState("");
  const [commerceApp, setCommerceApp] = useState<AdminAppListItem | null>(null);
  const [versions, setVersions] = useState<AppVersion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  // Translation state: versionId -> locale -> { release_notes }
  const [translations, setTranslations] = useState<Record<string, Record<string, { release_notes: string }>>>(
    {},
  );

  // New version form
  const [newVersion, setNewVersion] = useState("");
  const [newReleaseNotes, setNewReleaseNotes] = useState<Record<string, string>>({});
  const [newActiveLocale, setNewActiveLocale] = useState("");
  const [creating, setCreating] = useState(false);

  // Existing version editing
  const [editingVersion, setEditingVersion] = useState<string | null>(null);
  const [editActiveLocales, setEditActiveLocales] = useState<Record<string, string>>({});
  const [savingNotes, setSavingNotes] = useState<string | null>(null);

  // Upload state per version
  const [uploadingFor, setUploadingFor] = useState<string | null>(null);
  const [uploadErrorFor, setUploadErrorFor] = useState<{ id: string; message: string } | null>(null);
  const fileRefs = useRef<Record<string, HTMLInputElement | null>>({});

  const defaultLocale = locales.find((l) => l.is_default)?.code ?? "";

  // Initialize active locale for new version form
  useEffect(() => {
    if (locales.length > 0 && !newActiveLocale) {
      setNewActiveLocale(defaultLocale || locales[0].code);
    }
  }, [locales, newActiveLocale, defaultLocale]);

  useEffect(() => {
    if (!projectId) return;
    Promise.all([adminListProjects(), adminListApps()])
      .then(async ([projects, apps]) => {
        const project = projects.find((p) => p.id === projectId);
        if (!project) {
          addToast("Project not found", "error");
          navigate("/admin/projects");
          return;
        }
        setProjectTitle(project.title || "(untitled)");

        const app = apps.find((a) => a.project_id === projectId && !a.deleted_at);
        if (!app) {
          addToast("Project has no commerce attached", "error");
          navigate(`/admin/projects/${projectId}/edit`);
          return;
        }
        setCommerceApp(app);

        const [vs, tm] = await Promise.all([
          adminListVersions(app.id),
          adminGetVersionTranslations(app.id),
        ]);
        setVersions(vs);
        setTranslations(tm);
      })
      .catch((err) => setError(err instanceof Error ? err.message : "Failed to load"))
      .finally(() => setLoading(false));
  }, [projectId, addToast, navigate]);

  function getEditLocale(versionId: string): string {
    return editActiveLocales[versionId] || defaultLocale;
  }

  function getReleaseNotes(versionId: string, locale: string): string {
    return translations[versionId]?.[locale]?.release_notes ?? "";
  }

  function updateReleaseNotes(versionId: string, locale: string, value: string) {
    setTranslations((prev) => ({
      ...prev,
      [versionId]: {
        ...(prev[versionId] ?? {}),
        [locale]: { release_notes: value },
      },
    }));
  }

  async function handleSaveNotes(versionId: string) {
    if (!commerceApp) return;
    const locale = getEditLocale(versionId);
    const notes = getReleaseNotes(versionId, locale);
    setSavingNotes(versionId);
    try {
      await adminUpsertVersionTranslation(commerceApp.id, versionId, locale, { release_notes: notes });
      const label = locales.find((l) => l.code === locale)?.name ?? locale;
      addToast(`Release notes (${label}) saved.`, "success");
    } catch (err) {
      addToast(err instanceof APIError ? err.message : "Save failed", "error");
    } finally {
      setSavingNotes(null);
    }
  }

  async function handleCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!commerceApp || !newVersion.trim()) return;
    setCreating(true);
    setError("");
    try {
      // Use the default locale's release notes for the base version record
      const defaultNotes = newReleaseNotes[defaultLocale] ?? "";
      const v = await adminCreateVersion(commerceApp.id, { version: newVersion.trim(), release_notes: defaultNotes });
      setVersions((prev) => [v, ...prev]);

      // Save all locale translations for the new version
      const savePromises = Object.entries(newReleaseNotes)
        .filter(([, notes]) => notes.trim() !== "")
        .map(([locale, notes]) =>
          adminUpsertVersionTranslation(commerceApp.id, v.id, locale, { release_notes: notes }),
        );
      if (savePromises.length > 0) {
        await Promise.allSettled(savePromises);
      }

      // Seed translations state for the new version
      const newTrans: Record<string, { release_notes: string }> = {};
      locales.forEach((l) => {
        newTrans[l.code] = { release_notes: newReleaseNotes[l.code] ?? "" };
      });
      setTranslations((prev) => ({ ...prev, [v.id]: newTrans }));

      setNewVersion("");
      setNewReleaseNotes({});
    } catch (err) {
      setError(err instanceof APIError ? err.message : "Failed to create version");
    } finally {
      setCreating(false);
    }
  }

  async function handleUpload(versionId: string) {
    const file = fileRefs.current[versionId]?.files?.[0];
    if (!file || !commerceApp) return;
    setUploadingFor(versionId);
    setUploadErrorFor(null);
    try {
      await adminUploadBinary(commerceApp.id, versionId, file);
      const updated = await adminListVersions(commerceApp.id);
      setVersions(updated);
    } catch (err) {
      setUploadErrorFor({ id: versionId, message: err instanceof APIError ? err.message : "Upload failed" });
    } finally {
      setUploadingFor(null);
    }
  }

  if (loading || localesLoading)
    return (
      <div className="page">
        <PageHeader title="Versions" />
        <SkeletonTable rows={3} cols={5} />
      </div>
    );

  if (!commerceApp || !projectId) {
    return (
      <div className="page">
        <PageHeader title="Versions" />
        <p className="error">{error || "Project has no commerce attached."}</p>
      </div>
    );
  }

  return (
    <div className="page">
      <PageHeader title="Versions" />
      <AdminProjectTabs projectId={projectId} projectTitle={projectTitle} hasCommerce />

      {error && <p className="error">{error}</p>}

      <div className="admin-section">
        <h2>Add Version</h2>
        <form onSubmit={handleCreate}>
          <div className="form-group">
            <label htmlFor="version">Version string *</label>
            <input
              id="version"
              type="text"
              value={newVersion}
              onChange={(e) => setNewVersion(e.target.value)}
              placeholder="1.0.0"
              required
            />
          </div>
          {locales.length > 1 && (
            <LocaleTabs locales={locales} activeLocale={newActiveLocale} onChange={setNewActiveLocale} />
          )}
          <div className="form-group">
            <label htmlFor="release-notes">
              Release notes{locales.length > 1 ? ` (${newActiveLocale})` : ""}
            </label>
            <textarea
              id="release-notes"
              rows={3}
              value={newReleaseNotes[newActiveLocale] ?? ""}
              onChange={(e) => setNewReleaseNotes((prev) => ({ ...prev, [newActiveLocale]: e.target.value }))}
              placeholder="What's new in this version..."
            />
          </div>
          <div className="form-actions">
            <button type="submit" className="btn btn-primary" disabled={creating}>
              {creating ? "Creating..." : "Create Version"}
            </button>
          </div>
        </form>
      </div>

      <div className="admin-section">
        <h2>Published Versions</h2>
        {versions.length === 0 ? (
          <p className="admin-empty">No versions yet.</p>
        ) : (
          <table className="admin-table">
            <thead>
              <tr>
                <th>Version</th>
                <th>Published</th>
                <th>Binary</th>
                <th>Upload</th>
                <th>Notes</th>
              </tr>
            </thead>
            <tbody>
              {versions.map((v) => {
                const isEditing = editingVersion === v.id;
                const editLocale = getEditLocale(v.id);
                return (
                  <tr key={v.id}>
                    <td>
                      <strong>{v.version}</strong>
                    </td>
                    <td>{new Date(v.published_at).toLocaleDateString()}</td>
                    <td>
                      {v.file_path ? (
                        <span className="tag tag-success">{v.file_path.split("/").pop()}</span>
                      ) : (
                        <span className="tag tag-pending">No binary</span>
                      )}
                    </td>
                    <td>
                      <div className="admin-upload-row">
                        <input
                          type="file"
                          accept=".dmg,.pkg,.zip"
                          ref={(el) => {
                            fileRefs.current[v.id] = el;
                          }}
                        />
                        <button
                          className="btn btn-secondary btn-small"
                          onClick={() => handleUpload(v.id)}
                          disabled={uploadingFor === v.id}
                        >
                          {uploadingFor === v.id ? "Uploading..." : "Upload"}
                        </button>
                      </div>
                      {uploadErrorFor?.id === v.id && (
                        <p className="error admin-upload-error">{uploadErrorFor.message}</p>
                      )}
                    </td>
                    <td>
                      <button
                        className="btn btn-secondary btn-small"
                        onClick={() => setEditingVersion(isEditing ? null : v.id)}
                      >
                        {isEditing ? "Close" : "Edit notes"}
                      </button>
                      {isEditing && (
                        <div className="admin-version-notes-edit">
                          {locales.length > 1 && (
                            <LocaleTabs
                              locales={locales}
                              activeLocale={editLocale}
                              onChange={(loc) => setEditActiveLocales((prev) => ({ ...prev, [v.id]: loc }))}
                            />
                          )}
                          <textarea
                            rows={3}
                            value={getReleaseNotes(v.id, editLocale)}
                            onChange={(e) => updateReleaseNotes(v.id, editLocale, e.target.value)}
                            placeholder="Release notes..."
                          />
                          <button
                            className="btn btn-primary btn-small"
                            onClick={() => handleSaveNotes(v.id)}
                            disabled={savingNotes === v.id}
                          >
                            {savingNotes === v.id ? "Saving..." : "Save"}
                          </button>
                        </div>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
