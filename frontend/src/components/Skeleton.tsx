interface SkeletonProps {
  variant?: 'text' | 'card' | 'table-row' | 'stat-card' | 'circle';
  width?: string;
  height?: string;
  count?: number;
}

function SkeletonItem({ variant = 'text', width, height }: Omit<SkeletonProps, 'count'>) {
  const className = `skeleton skeleton-${variant}`;
  const style: React.CSSProperties = {};
  if (width) style.width = width;
  if (height) style.height = height;
  return <div className={className} style={style} />;
}

export default function Skeleton({ variant = 'text', width, height, count = 1 }: SkeletonProps) {
  if (count === 1) return <SkeletonItem variant={variant} width={width} height={height} />;
  return (
    <div className="skeleton-group">
      {Array.from({ length: count }, (_, i) => (
        <SkeletonItem key={i} variant={variant} width={width} height={height} />
      ))}
    </div>
  );
}

export function SkeletonTable({ rows = 5, cols = 4 }: { rows?: number; cols?: number }) {
  return (
    <div className="skeleton-table">
      <div className="skeleton-table-header">
        {Array.from({ length: cols }, (_, i) => (
          <div key={i} className="skeleton skeleton-text" style={{ width: `${60 + Math.random() * 40}%` }} />
        ))}
      </div>
      {Array.from({ length: rows }, (_, r) => (
        <div key={r} className="skeleton-table-row">
          {Array.from({ length: cols }, (_, c) => (
            <div key={c} className="skeleton skeleton-text" style={{ width: `${50 + Math.random() * 50}%` }} />
          ))}
        </div>
      ))}
    </div>
  );
}

export function SkeletonStatCards({ count = 5 }: { count?: number }) {
  return (
    <div className="admin-stat-cards">
      {Array.from({ length: count }, (_, i) => (
        <div key={i} className="admin-stat-card">
          <div className="skeleton skeleton-text" style={{ width: '60%', height: '1.8rem' }} />
          <div className="skeleton skeleton-text" style={{ width: '80%', height: '0.85rem' }} />
        </div>
      ))}
    </div>
  );
}
