import { render, screen } from '@testing-library/react';
import { expect, it } from 'vitest';
import { PostCard, type Post } from './post-card';

it('uses supplied attachment data and does not fabricate previews or member actions', () => {
  const post: Post = { id: 9, kind: 'file', title: 'Другой файл', text: 'Описание', topic: 'Работа', tags: ['отчёт'], date: 'Сегодня', public: true, author: 'Автор', attachment: { format: 'DOCX', name: 'report.docx', description: '20 КБ' } };
  const { rerender } = render(<PostCard post={post} />);
  expect(screen.getByText('report.docx')).toBeInTheDocument();
  expect(screen.getByText('DOCX')).toBeInTheDocument();
  expect(screen.queryByRole('button', { name: /избранное/ })).not.toBeInTheDocument();
  rerender(<PostCard post={{ ...post, attachment: undefined }} />);
  expect(screen.queryByText('report.docx')).not.toBeInTheDocument();
  expect(screen.queryByText(/personal-knowledge/)).not.toBeInTheDocument();
});
