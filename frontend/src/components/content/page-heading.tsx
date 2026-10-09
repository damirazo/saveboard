import type { ReactNode } from 'react';

export function PageHeading({ title, count, description, actions }: { title: string; count?: number; description?: ReactNode; actions?: ReactNode }) {
  return <div className="page-heading"><div><h1>{title}{count !== undefined && <> <span>{count}</span></>}</h1>{description && <p>{description}</p>}</div>{actions}</div>;
}

export function EmptyState({ children, action }: { children: ReactNode; action?: ReactNode }) {
  return <div className="empty-message"><p role="status">{children}</p>{action}</div>;
}
