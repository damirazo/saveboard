import { useState } from 'react';
import { Link, useNavigate } from '@tanstack/react-router';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient, type components } from '@saveboard/api-client';
import { Bookmark, Pencil, Plus, Trash2, MessageCircle, NotebookText } from 'lucide-react';
import { Avatar } from '../../components/layout/sidebar';
import { useViewer } from '../auth/use-viewer';
import { Publication } from './publication';
import { AppShell } from '../../components/layout/app-shell';
import { Pagination } from '../../components/navigation/pagination';
import { PageHeading, EmptyState } from '../../components/content/page-heading';
import { SearchPanel } from '../../components/content/search-panel';
import { FilterGroup, FilterPanel } from '../../components/content/filter-panel';
import { Button } from '../../components/ui/button';
import { useSession } from '../auth/use-session';
import { TagPicker } from './tag-picker';
import { apiError, type SavedPost, type PostWrite } from './post-api';

export type LibrarySection = 'all' | 'favorites' | 'archives';
const filters = [{ value: '', label: 'Все' }, { value: 'link', label: 'Ссылки' }, { value: 'note', label: 'Заметки' }, { value: 'photo', label: 'Фото' }];
export function LibraryPage({ member = false, section = 'all', searching = false, author, initialTag, initialMine = false }: { member?: boolean; section?: LibrarySection; searching?: boolean; initialMine?: boolean; author?: string; initialTag?: { id: string; title: string } }) {
  const session = useSession(); const cache = useQueryClient(); const navigate = useNavigate();
  const [kind, setKind] = useState(''); const [query, setQuery] = useState(''); const [sort, setSort] = useState<'new' | 'old'>('new');
  const [mine, setMine] = useState(initialMine);
  const me = useViewer();
  const profile = useQuery({ queryKey: ['profile', author], enabled: !!author, queryFn: async ({ signal }) => {
    const result = await apiClient.GET('/api/v1/profiles/{username}', { params: { path: { username: author! } }, signal });
    if (result.error || !result.data) throw apiError(result.error);
    return result.data;
  } });
  const [includedTags, setIncludedTags] = useState<PostWrite['tags']>(initialTag ? [initialTag] : []);
  const [excludedTags, setExcludedTags] = useState<PostWrite['tags']>([]);
  const [paging, setPaging] = useState({ page: 1, size: 10 });
  const [deleting, setDeleting] = useState(''); const [error, setError] = useState('');
  const hasSearch = !!(query.trim() || includedTags.length || excludedTags.length || kind);
  const enabled = (!author || !!profile.data) && (!searching || hasSearch) && section === 'all' && (!member || !!session);
  const posts = useQuery<components['schemas']['Page']>({ queryKey: ['posts', session?.tokens.session_id, member, kind, query, mine, author, includedTags, excludedTags, sort, paging, section], enabled,
    refetchInterval: query => query.state.data?.items?.some(p => p.capture_state === 'pending') ? 3000 : false,
    queryFn: async ({ signal }) => {
      const result = await apiClient.GET('/api/v1/posts', { params: { query: { author, scope: member || (mine && session) ? 'mine' : 'public', type: kind ? kind as 'note' | 'link' | 'photo' : undefined, q: query || undefined, include_tags: includedTags.map(t => t.id).join(',') || undefined, exclude_tags: excludedTags.map(t => t.id).join(',') || undefined, sort, page: paging.page, size: paging.size } }, signal });
      if (result.error || !result.data) throw apiError(result.error); return result.data;
    },
  });
  const items: SavedPost[] = enabled ? (posts.data?.items ?? []).map(p => ({ ...p, tags: p.tags ?? [], photos: p.photos ?? [] })) : []; const total = enabled ? posts.data?.total ?? 0 : 0;
  async function refresh() { await Promise.all(['posts', 'profile'].map(key => cache.invalidateQueries({ queryKey: [key] }))); }
  async function remove(post: SavedPost) {
    if (!window.confirm(`Переместить «${post.title}» в черновики? Пост исчезнет из лент.`)) return;
    setDeleting(post.id); setError('');
    try { const r = await apiClient.DELETE('/api/v1/posts/{id}', { params: { path: { id: post.id } } }); if (r.error) throw apiError(r.error); setPaging(p => ({ ...p, page: 1 })); await refresh(); }
    catch (e) { setError(apiError(e).message); } finally { setDeleting(''); }
  }
  const resetPage = () => setPaging(p => ({ ...p, page: 1 }));
  const title = author ? 'Профиль пользователя' : searching ? 'Поиск' : member ? 'Библиотека' : 'Главная страница';
  return <AppShell>
    <PageHeading title={title} actions={session && !searching && !author && !(member && posts.isSuccess && !total && !kind) ? <Button onClick={() => void navigate({ to: '/posts/new' })}><Plus />Создать пост</Button> : undefined} />
    {profile.isError && <p role="alert">Не удалось открыть профиль. <Button onClick={() => void profile.refetch()}>Повторить</Button></p>}
    {author && profile.isLoading && <p role="status">Загружаем профиль…</p>}
    <div className={author ? 'profile-layout' : undefined}>
    {profile.data && <aside className="profile-card"><div className="profile-cover" /><Avatar name={profile.data.display_name || profile.data.username} /><h2>{profile.data.display_name || profile.data.username}</h2><p>@{profile.data.username}</p></aside>}
    <section className="feed-column">
    {profile.data && <><div className="profile-stats"><div><NotebookText /><span>Публичные посты<strong>{profile.data.public_posts}</strong></span></div><div><MessageCircle /><span>Комментарии автора<strong title="Комментарии недоступны">—</strong></span></div></div><h2>Публичные публикации</h2></>}
    {searching && <div className="search-panel"><SearchPanel value={query} onChange={v => { setQuery(v); resetPage(); }} label="Поиск публикаций" placeholder="Название или текст публикации" />
      {session && <FilterGroup label="Область поиска" value={mine ? 'mine' : 'public'} options={[{ value: 'public', label: 'Публичные посты' }, { value: 'mine', label: 'Моя библиотека' }]} onChange={v => { setMine(v === 'mine'); resetPage(); }} />}
      <div className="search-tags"><TagPicker label="Включить теги" allowCreate={false} disabled={false} value={includedTags} onChange={v => { setIncludedTags(v); resetPage(); }} />
      <TagPicker label="Исключить теги" allowCreate={false} disabled={false} value={excludedTags} onChange={v => { setExcludedTags(v); resetPage(); }} /></div>
    </div>}
    {section !== 'all' ? <EmptyState>В этом разделе пока нет публикаций. <Link to="/feed" search={{ section: 'all' }}>Перейти в библиотеку</Link></EmptyState> : (!author || profile.data) && <>
      <FilterPanel sort={{ value: sort, options: [{ value: 'new', label: 'Сначала новые' }, { value: 'old', label: 'Сначала старые' }], onChange: v => { setSort(v as 'new' | 'old'); setPaging(p => ({ ...p, page: 1 })); } }}><FilterGroup label="Тип контента" options={filters} value={kind} onChange={v => { setKind(v); setPaging(p => ({ ...p, page: 1 })); }} /></FilterPanel>
      {posts.isLoading && <p role="status">Загружаем публикации…</p>}
      {(posts.error || error) && <p role="alert">{error || apiError(posts.error).message}<Button variant="outline" onClick={() => { setError(''); void posts.refetch(); }}>Повторить</Button></p>}
      {searching && hasSearch && posts.data && <p className="result-count">Найдено: {total}</p>}
      <div className="post-feed">{items.map(post => <Publication key={post.id} post={post} actions={session && me.data?.user_id === post.author_id ? <div className="post-actions">
        <Button variant="icon" aria-label={`Редактировать ${post.title}`} onClick={() => void navigate({ to: '/posts/$postId/edit', params: { postId: post.id } })}><Pencil /></Button>
        <Button variant="icon" aria-label={`Удалить ${post.title}`} disabled={!!deleting} onClick={() => void remove(post)}><Trash2 /></Button></div> : undefined} />)}</div>
      {member && posts.isSuccess && !total && !kind ? <div className="empty-message library-empty"><Bookmark aria-hidden="true" /><h2>В библиотеке пока пусто</h2><p>Создайте первый пост, чтобы сохранить ссылку, заметку или фото.</p><Button onClick={() => void navigate({ to: '/posts/new' })}><Plus />Создать пост</Button></div> : !posts.isLoading && !posts.error && items.length === 0 && <EmptyState>{searching ? hasSearch ? 'Ничего не найдено. Попробуйте другой запрос или измените теги.' : 'Введите запрос или выберите теги для поиска.' : 'Публикаций пока нет.'}</EmptyState>}
      {total > 0 && <Pagination total={total} page={paging.page} pageSize={paging.size} pageSizes={[2, 10, 20, 50]} onPageChange={page => setPaging(p => ({ ...p, page }))} onPageSizeChange={size => setPaging({ page: 1, size })} />}
    </>}
    </section></div>
  </AppShell>;
}
