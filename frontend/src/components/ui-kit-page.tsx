import { useState } from 'react';
import { Check, MessageCircle, Plus } from 'lucide-react';
import { Link } from '@tanstack/react-router';
import { Button, FilterGroup, FormField, FormMessage, PostTypeBadge, Select } from './index';

import { Avatar } from './layout/sidebar';

const accents = [
  ['primary', 'Основное действие'], ['violet', 'Ссылка'], ['amber', 'Заметка'],
  ['mint', 'Фотография'], ['danger', 'Ошибка'],
] as const;
const filters = [{ value: 'all', label: 'Все' }, { value: 'link', label: 'Ссылки' }, { value: 'note', label: 'Заметки' }];

export function UiKitPage() {
  const [type, setType] = useState('all');
  const [saved, setSaved] = useState(false);
  return <main className="kit-page">
    <Link to="/" className="kit-back">← На главную</Link>
    <h1>UI-kit Saveboard</h1>
    <section className="kit-section"><h2>Цветовые роли</h2><div className="kit-row">{accents.map(([token, label]) =>
      <span key={token} className="kit-swatch"><i style={{ background: `var(--sb-${token})` }} />{label}</span>)}</div></section>
    <section className="kit-section"><h2>Действия и состояния</h2><div className="kit-row">
      <Button onClick={() => setSaved(true)}>{saved ? <Check /> : <Plus />}{saved ? 'Сохранено' : 'Сохранить'}</Button>
      <Button variant="outline" onClick={() => setSaved(false)}>Сбросить</Button>
      <Button disabled>Недоступно</Button>
    </div></section>
    <section className="kit-section"><h2>Фильтры и типы публикаций</h2>
      <div className="kit-row"><FilterGroup label="Тип контента" value={type} options={filters} onChange={setType} />
      <Select label="Порядок публикаций" options={[{ value: 'new', label: 'Сначала новые' }, { value: 'old', label: 'Сначала старые' }]} /></div>
      <div className="kit-row">{['link', 'note', 'photo'].map(value => <PostTypeBadge key={value} type={value} />)}</div>
    </section>
    <section className="kit-section"><h2>Подпись публикации</h2>
      <div className="publication-tags"><span>@annamirova</span><span>#заметка</span><span>#архитектура</span><span>#город</span></div>
      <div className="kit-row kit-attribution"><span className="identity"><Avatar name="Анна Мирова" />Анна Мирова</span><time dateTime="2026-11-27T14:30:00+03:00">27.11.2026 · 14:30</time></div>
      <div className="kit-row"><span className="identity"><MessageCircle size={16} />3</span><span>Комментарии доступны</span></div>
      <div className="kit-row"><span>—</span><span>Комментарии недоступны</span></div>
    </section>
    <section className="kit-section"><h2>Поля и обратная связь</h2>
      <FormField label="Название" placeholder="Название публикации" />
      <FormField label="Адрес ссылки" defaultValue="example" error="Введите полный адрес страницы." />
      <FormMessage>Изменения сохранены.</FormMessage>
    </section>
  </main>;
}
