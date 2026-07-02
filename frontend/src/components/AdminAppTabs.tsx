import { Link, useLocation } from 'react-router-dom';

interface Props {
  projectId: string;
  projectTitle: string;
  hasCommerce: boolean;
}

// AdminProjectTabs renders the project context bar shown on edit / images /
// versions screens. The Versions tab only appears when the project has commerce
// attached (no commerce → no app row → no versions table).
export default function AdminProjectTabs({ projectId, projectTitle, hasCommerce }: Props) {
  const { pathname } = useLocation();

  function tabClass(path: string) {
    return `tab-bar-item${pathname === path ? ' tab-bar-item-active' : ''}`;
  }

  return (
    <>
      <nav className="admin-breadcrumb">
        <Link to="/admin/projects">Projects</Link>
        <span className="admin-breadcrumb-sep">/</span>
        <span>{projectTitle}</span>
      </nav>
      <div className="tab-bar">
        <Link to={`/admin/projects/${projectId}/edit`} className={tabClass(`/admin/projects/${projectId}/edit`)}>
          Edit
        </Link>
        <Link to={`/admin/projects/${projectId}/images`} className={tabClass(`/admin/projects/${projectId}/images`)}>
          Images
        </Link>
        {hasCommerce && (
          <Link to={`/admin/projects/${projectId}/versions`} className={tabClass(`/admin/projects/${projectId}/versions`)}>
            Versions
          </Link>
        )}
      </div>
    </>
  );
}
