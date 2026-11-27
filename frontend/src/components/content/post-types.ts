import { Image, Link2, NotebookPen } from 'lucide-react';

export const postTypes = [
  { value: 'note', label: 'Заметка', Icon: NotebookPen, tone: 'amber' },
  { value: 'link', label: 'Ссылка', Icon: Link2, tone: 'violet' },
  { value: 'photo', label: 'Фото', Icon: Image, tone: 'mint' },
] as const;
