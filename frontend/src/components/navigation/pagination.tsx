import { ChevronLeft, ChevronRight } from 'lucide-react';
import { Button } from '../ui/button';
import { Select } from '../ui/form-field';

type PaginationProps = {
  page: number; pageSize: number; total: number; onPageChange: (page: number) => void;
  onPageSizeChange?: (size: number) => void; pageSizes?: readonly number[]; label?: string;
};
export function Pagination({ page, pageSize, total, onPageChange, onPageSizeChange, pageSizes = [10, 20, 50], label = 'Пагинация' }: PaginationProps) {
  const size = Math.max(1, Math.trunc(pageSize));
  const count = Math.max(1, Math.ceil(total / size));
  const current = Math.min(count, Math.max(1, Math.trunc(page)));
  // Only nearby pages are rendered; a large result set does not create thousands of buttons.
  const pages = [...new Set([1, current - 1, current, current + 1, count])].filter(value => value >= 1 && value <= count).sort((a, b) => a - b);
  const sizes = [...new Set([...pageSizes, size])].filter(value => value > 0).sort((a, b) => a - b);
  return <nav className="pagination" aria-label={label}>
    <span className="pagination-summary">{total ? `${(current - 1) * size + 1}–${Math.min(current * size, total)} из ${total}` : '0 результатов'}</span>
    <div className="pagination-pages"><Button variant="outline" aria-label="Предыдущая страница" disabled={current === 1 || total === 0} onClick={() => onPageChange(current - 1)}><ChevronLeft /></Button>
      {pages.map((value, index) => <span key={value}>{index > 0 && value - pages[index - 1]! > 1 && <span className="pagination-gap" aria-hidden="true">…</span>}<Button variant="filter" aria-label={`Страница ${value}`} aria-current={value === current ? 'page' : undefined} disabled={total === 0} onClick={() => onPageChange(value)}>{value}</Button></span>)}
      <Button variant="outline" aria-label="Следующая страница" disabled={current === count || total === 0} onClick={() => onPageChange(current + 1)}><ChevronRight /></Button></div>
    {onPageSizeChange && <Select label="Публикаций на странице" value={size} options={sizes.map(value => ({ value: String(value), label: `${value} на странице` }))} onChange={event => onPageSizeChange(Number(event.target.value))} />}
  </nav>;
}
