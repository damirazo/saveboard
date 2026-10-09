import type { ReactNode } from 'react';

export type BreadcrumbItem = { label: string; link?: ReactNode };
export function Breadcrumbs({ items }: { items: readonly BreadcrumbItem[] }) {
  return <nav className="breadcrumb" aria-label="Хлебные крошки"><ol>{items.map((item, index) => <li key={`${index}-${item.label}`} aria-current={index === items.length - 1 ? 'page' : undefined}>{index === items.length - 1 ? item.label : item.link ?? item.label}</li>)}</ol></nav>;
}
