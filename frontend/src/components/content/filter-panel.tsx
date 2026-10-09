import type { ReactNode } from 'react';
import { Button } from '../ui/button';
import { Select, type SelectOption } from '../ui/form-field';

export function FilterGroup({ options, value, onChange, label }: { options: readonly SelectOption[]; value: string; onChange: (value: string) => void; label: string }) {
  return <div className="content-filters" role="group" aria-label={label}>{options.map(option => <Button key={option.value} variant="filter" aria-pressed={value === option.value} onClick={() => onChange(option.value)}>{option.label}</Button>)}</div>;
}

export function FilterPanel({ children, sort }: { children: ReactNode; sort?: { value: string; options: readonly SelectOption[]; onChange: (value: string) => void; label?: string } }) {
  return <div className="feed-toolbar">{children}{sort && <Select label={sort.label ?? 'Порядок публикаций'} options={sort.options} value={sort.value} onChange={event => sort.onChange(event.target.value)} />}</div>;
}
