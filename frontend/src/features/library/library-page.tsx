import { useState } from 'react';
import { Link } from '@tanstack/react-router';
import { AppShell } from '../../components/layout/app-shell';
import { guestViewer } from '../../components/layout/viewer';
import { Breadcrumbs } from '../../components/navigation/breadcrumbs';
import { Pagination } from '../../components/navigation/pagination';
import { PageHeading, EmptyState } from '../../components/content/page-heading';
import { SearchPanel } from '../../components/content/search-panel';
import { FilterGroup, FilterPanel } from '../../components/content/filter-panel';
import { PostCard } from '../../components/content/post-card';
import { Button } from '../../components/ui/button';
import { useSession } from '../auth/use-session';
import { demoPosts } from './demo-posts';

export type LibrarySection = 'all' | 'favorites' | 'archives';
const filters = [{ value: 'all', label: 'Все' }, { value: 'web', label: 'Страницы' }, { value: 'text', label: 'Заметки' }, { value: 'file', label: 'Файлы' }];
const sortOptions = [{ value: 'new', label: 'Сначала новые' }, { value: 'old', label: 'Сначала старые' }];

export function LibraryPage({ member = false, section = 'all' }: { member?: boolean; section?: LibrarySection }) {
  const session = useSession();
  const [kind, setKind] = useState('all');
  const [query, setQuery] = useState('');
  const [favorites, setFavorites] = useState([1]);
  const [sort, setSort] = useState('new');
  const [paging, setPaging] = useState({ page: 1, size: 10, scope: '' });
  function changeQuery(value: string) { setQuery(value); setPaging(current => ({ ...current, page: 1 })); }
  function changeKind(value: string) { setKind(value); setPaging(current => ({ ...current, page: 1 })); }
  function changeSort(value: string) { setSort(value); setPaging(current => ({ ...current, page: 1 })); }
  const scope = JSON.stringify([member, section, kind, query, sort]);
  const visible = demoPosts.filter(p => (member || p.public) && (kind === 'all' || p.kind === kind) && (section !== 'favorites' || favorites.includes(p.id)) && (section !== 'archives' || p.kind === 'web') && `${p.title} ${p.text} ${p.tags.join(' ')}`.toLowerCase().includes(query.toLowerCase()));
  if (sort === 'old') visible.reverse();
  const page = Math.min(paging.scope === scope ? paging.page : 1, Math.max(1, Math.ceil(visible.length / paging.size)));
  const pagePosts = visible.slice((page - 1) * paging.size, page * paging.size);
  const title = !member ? 'Интересное рядом' : section === 'favorites' ? 'Избранное' : section === 'archives' ? 'Архив страниц' : 'Все публикации';

  return <AppShell viewer={session ? { kind: 'member', username: session.login, displayName: session.login } : guestViewer}>
    <Breadcrumbs items={[{ label: member ? 'Личное пространство' : 'Сообщество' }, { label: member ? 'Библиотека' : 'Публичная лента' }]} />
    {session && <nav className="feed-switcher" aria-label="Выбор ленты">
      <Link to="/feed" search={{ section: 'all' }} aria-current={member ? 'page' : undefined}>Моя лента</Link>
      <Link to="/" aria-current={!member ? 'page' : undefined}>Публичная лента</Link>
    </nav>}
    <PageHeading title={title} count={visible.length} description={member ? 'Ссылки, мысли и материалы, к которым хочется вернуться.' : 'Открытые подборки, полезные ссылки и идеи сообщества.'} actions={!session && <Button asChild><Link to="/register">Создать свою библиотеку</Link></Button>} />
    <SearchPanel value={query} onChange={changeQuery} label="Поиск публикаций" placeholder="Поиск по названию, тексту или тегу…" />
    <FilterPanel sort={{ value: sort, options: sortOptions, onChange: changeSort }}><FilterGroup label="Тип контента" options={filters} value={kind} onChange={changeKind} /></FilterPanel>
    <div className="post-feed">{pagePosts.map(post => <PostCard key={post.id} post={post} showVisibility={member} favorite={favorites.includes(post.id)} onFavoriteChange={member ? () => { setFavorites(current => current.includes(post.id) ? current.filter(id => id !== post.id) : [...current, post.id]); setPaging(current => ({ ...current, page: 1, scope })); } : undefined} onTagClick={changeQuery} />)}</div>
    {visible.length === 0 && <EmptyState>Публикаций не найдено. Попробуйте другой запрос или фильтр.</EmptyState>}
    <Pagination total={visible.length} page={page} pageSize={paging.size} pageSizes={[2, 10, 20, 50]} onPageChange={next => setPaging(current => ({ ...current, page: next, scope }))} onPageSizeChange={size => setPaging({ page: 1, size, scope })} />
    <p className="demo-caption">Демонстрационные публикации · {member ? 'персональная лента' : 'публичная лента'}</p>
  </AppShell>;
}
