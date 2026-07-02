import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  adminListProjects,
  adminDeleteProject,
  adminRestoreProject,
  adminReorderProjects,
  adminDetachCommerce,
  type AdminProject,
} from '../../api/client';
import PageHeader from '../../components/PageHeader';
import ConfirmModal from '../../components/ConfirmModal';
import { useToast } from '../../context/ToastContext';
import { SkeletonTable } from '../../components/Skeleton';

export default function AdminProjects() {
  const [projects, setProjects] = useState<AdminProject[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [deleteTarget, setDeleteTarget] = useState<AdminProject | null>(null);
  const [detachTarget, setDetachTarget] = useState<AdminProject | null>(null);
  const { addToast } = useToast();

  useEffect(() => {
    adminListProjects()
      .then(setProjects)
      .catch((err) => {
        setError(err instanceof Error ? err.message : 'Failed to load projects');
      })
      .finally(() => setLoading(false));
  }, []);

  async function handleDelete(project: AdminProject) {
    try {
      await adminDeleteProject(project.id);
      setProjects(prev => prev.map(p =>
        p.id === project.id ? { ...p, deleted_at: new Date().toISOString() } : p
      ));
      addToast('Project deleted', 'success');
    } catch {
      addToast('Failed to delete project', 'error');
    }
    setDeleteTarget(null);
  }

  async function handleRestore(id: string) {
    try {
      await adminRestoreProject(id);
      setProjects(prev => prev.map(p =>
        p.id === id ? { ...p, deleted_at: null } : p
      ));
      addToast('Project restored', 'success');
    } catch {
      addToast('Failed to restore project', 'error');
    }
  }

  async function handleDetach(project: AdminProject) {
    try {
      await adminDetachCommerce(project.id);
      setProjects(prev => prev.map(p =>
        p.id === project.id ? { ...p, commerce: undefined } : p
      ));
      addToast('Commerce detached', 'success');
    } catch {
      addToast('Failed to detach commerce', 'error');
    }
    setDetachTarget(null);
  }

  async function moveItem(index: number, direction: -1 | 1) {
    const target = index + direction;
    if (target < 0 || target >= projects.length) return;
    const newProjects = [...projects];
    [newProjects[index], newProjects[target]] = [newProjects[target], newProjects[index]];

    const positions: Record<string, number> = {
      [projects[index].id]: target,
      [projects[target].id]: index,
    };

    const oldProjects = projects;
    setProjects(newProjects);
    try {
      await adminReorderProjects(positions);
    } catch {
      setProjects(oldProjects);
      addToast('Failed to reorder', 'error');
    }
  }

  if (loading) {
    return (
      <div className="page">
        <PageHeader title="Projects" />
        <SkeletonTable rows={4} cols={6} />
      </div>
    );
  }

  return (
    <div className="page">
      <PageHeader
        title="Projects"
        actions={
          <Link to="/admin/projects/new" className="btn btn-primary btn-small">+ New Project</Link>
        }
      />

      {error && <p className="error">{error}</p>}

      {deleteTarget && (
        <ConfirmModal
          title="Delete Project"
          message="Are you sure you want to delete this project? It can be restored later."
          confirmLabel="Delete"
          danger
          onConfirm={() => handleDelete(deleteTarget)}
          onCancel={() => setDeleteTarget(null)}
        />
      )}

      {detachTarget && (
        <ConfirmModal
          title="Detach Commerce"
          message={`Detach commerce from "${detachTarget.title}"? The project (and its history) stays. Existing licenses are preserved. You can re-attach commerce later.`}
          confirmLabel="Detach"
          danger
          onConfirm={() => handleDetach(detachTarget)}
          onCancel={() => setDetachTarget(null)}
        />
      )}

      {projects.length === 0 ? (
        <div className="admin-empty-state">
          <p>No projects yet</p>
          <p>Add your first project to showcase on your landing page.</p>
          <Link to="/admin/projects/new" className="btn btn-primary">+ New Project</Link>
        </div>
      ) : (
        <table className="admin-table">
          <thead>
            <tr>
              <th>Image</th>
              <th>Title</th>
              <th>Slug</th>
              <th>Type</th>
              <th>Detail Page</th>
              <th>Position</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {projects.map((project, i) => (
              <tr key={project.id} className={project.deleted_at ? 'admin-row-inactive' : ''}>
                <td>
                  {project.image_url ? (
                    <img src={project.image_url} alt="" className="admin-project-thumb" />
                  ) : (
                    <div className="admin-project-thumb" style={{ background: 'var(--color-surface-raised)' }} />
                  )}
                </td>
                <td>
                  {project.title
                    ? project.title
                    : <em style={{ color: 'var(--color-text-muted)' }}>(untitled)</em>
                  }
                </td>
                <td>
                  <code className="app-id">{project.slug || '—'}</code>
                </td>
                <td>
                  {project.external_url ? (
                    <span className="badge badge-muted">External</span>
                  ) : project.commerce ? (
                    <span className="badge badge-success">Commerce</span>
                  ) : (
                    <span className="badge badge-muted">Showcase</span>
                  )}
                </td>
                <td>
                  {project.has_detail_page ? (
                    <span className="badge badge-success">Yes</span>
                  ) : (
                    <span className="admin-empty-cell">No</span>
                  )}
                </td>
                <td>
                  <div className="order-actions">
                    <button
                      className="btn btn-secondary btn-small order-btn"
                      onClick={() => moveItem(i, -1)}
                      disabled={i === 0}
                      aria-label="Move up"
                      style={i === 0 ? { opacity: 0.4 } : undefined}
                    >
                      ↑
                    </button>
                    <button
                      className="btn btn-secondary btn-small order-btn"
                      onClick={() => moveItem(i, 1)}
                      disabled={i === projects.length - 1}
                      aria-label="Move down"
                      style={i === projects.length - 1 ? { opacity: 0.4 } : undefined}
                    >
                      ↓
                    </button>
                  </div>
                </td>
                <td>
                  <div className="actions">
                    {project.deleted_at ? (
                      <button
                        className="btn btn-secondary btn-small"
                        onClick={() => handleRestore(project.id)}
                      >
                        Restore
                      </button>
                    ) : (
                      <>
                        <Link
                          to={`/admin/projects/${project.id}/edit`}
                          className="btn btn-secondary btn-small"
                        >
                          Edit
                        </Link>
                        {project.commerce && (
                          <button
                            className="btn btn-secondary btn-small"
                            onClick={() => setDetachTarget(project)}
                          >
                            Detach commerce
                          </button>
                        )}
                        <button
                          className="btn btn-danger btn-small"
                          onClick={() => setDeleteTarget(project)}
                        >
                          Delete
                        </button>
                      </>
                    )}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
