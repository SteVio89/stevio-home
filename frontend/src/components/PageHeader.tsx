import { Link } from 'react-router-dom';
import type { ReactNode } from 'react';

interface PageHeaderProps {
  title: string;
  backTo?: string;
  backLabel?: string;
  actions?: ReactNode;
  subtitle?: string;
}

export default function PageHeader({
  title,
  backTo,
  backLabel = 'Back',
  actions,
  subtitle,
}: PageHeaderProps) {
  return (
    <div className="page-header">
      {backTo && (
        <Link to={backTo} className="back-link">
          &larr; {backLabel}
        </Link>
      )}
      <div className="page-header-row">
        <div>
          <h1>{title}</h1>
          {subtitle && <p className="page-header-subtitle">{subtitle}</p>}
        </div>
        {actions && <div className="page-header-actions">{actions}</div>}
      </div>
    </div>
  );
}
