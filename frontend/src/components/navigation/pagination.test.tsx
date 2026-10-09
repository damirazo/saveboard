import { useState } from 'react';
import { fireEvent, render, screen } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { Pagination } from './pagination';

it('stops at both boundaries and changes the controlled page', () => {
  function Example() {
    const [page, setPage] = useState(1);
    return <Pagination page={page} pageSize={2} total={3} onPageChange={setPage} />;
  }
  render(<Example />);
  expect(screen.getByRole('button', { name: 'Предыдущая страница' })).toBeDisabled();
  fireEvent.click(screen.getByRole('button', { name: 'Следующая страница' }));
  expect(screen.getByText('3–3 из 3')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Страница 2' })).toHaveAttribute('aria-current', 'page');
  expect(screen.getByRole('button', { name: 'Следующая страница' })).toBeDisabled();
});

it('keeps large and empty result sets bounded', () => {
  const change = vi.fn();
  const { rerender } = render(<Pagination page={500} pageSize={10} total={10000} onPageChange={change} />);
  expect(screen.getAllByRole('button').length).toBeLessThanOrEqual(7);
  rerender(<Pagination page={500} pageSize={10} total={0} onPageChange={change} />);
  expect(screen.getByText('0 результатов')).toBeInTheDocument();
  for (const button of screen.getAllByRole('button')) expect(button).toBeDisabled();
  expect(change).not.toHaveBeenCalled();
});
