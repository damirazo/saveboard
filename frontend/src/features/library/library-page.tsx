import { useState } from 'react';
import { Link, useNavigate } from '@tanstack/react-router';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient, type components } from '@saveboard/api-client';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Pencil, Plus, Trash2 } from 'lucide-react';
import { AppShell } from '../../components/layout/app-shell';
import { guestViewer } from '../../components/layout/viewer';
import { Breadcrumbs } from '../../components/navigation/breadcrumbs';
import { Pagination } from '../../components/navigation/pagination';
import { PageHeading, EmptyState } from '../../components/content/page-heading';
import { SearchPanel } from '../../components/content/search-panel';
import { FilterGroup, FilterPanel } from '../../components/content/filter-panel';
import { TagList } from '../../components/content/post-card';
import { Button } from '../../components/ui/button';
import { useSession } from '../auth/use-session';
import { PhotoGallery } from './photo-gallery';
import { LinkPost } from './link-post';
import { TagPicker } from './tag-picker';
import { PostForm } from './post-form';
import { apiError, type SavedPost, type PostWrite } from './post-api';

export type LibrarySection = 'all' | 'favorites' | 'archives';
const filters = [{ value: '', label: 'Все' }, { value: 'link', label: 'Ссылки' }, { value: 'note', label: 'Заметки' }, { value: 'photo', label: 'Фото' }];
export function LibraryPage({ member = false, section = 'all' }: { member?: boolean; section?: LibrarySection }) {
  const session = useSession(); const cache = useQueryClient(); const navigate = useNavigate();
  const [kind, setKind] = useState(''); const [query, setQuery] = useState(''); const [sort, setSort] = useState<'new' | 'old'>('new');
  const [tagMode, setTagMode] = useState(false);
  const [includedTags, setIncludedTags] = useState<PostWrite['tags']>([]);
  const [excludedTags, setExcludedTags] = useState<PostWrite['tags']>([]);
  const [paging, setPaging] = useState({ page: 1, size: 10 });
  const [editing, setEditing] = useState<SavedPost | 'new' | null>(null);
  const [deleting, setDeleting] = useState(''); const [error, setError] = useState('');
  const enabled = section === 'all' && (!member || !!session);
  const posts = useQuery<components['schemas']['Page']>({ queryKey: ['posts', session?.tokens.session_id, member, kind, query, tagMode, includedTags, excludedTags, sort, paging, section], enabled,
    refetchInterval: query => query.state.data?.items?.some(p => p.capture_state === 'pending') ? 3000 : false,
    queryFn: async ({ signal }) => {
      const result = await apiClient.GET('/api/v1/posts', { params: { query: { scope: member ? 'mine' : 'public', type: kind ? kind as 'note' | 'link' | 'photo' : undefined, q: tagMode ? undefined : query, include_tags: tagMode ? includedTags.map(t => t.id).join(',') : undefined, exclude_tags: tagMode ? excludedTags.map(t => t.id).join(',') : undefined, sort, page: paging.page, size: paging.size } }, signal });
      if (result.error || !result.data) throw apiError(result.error); return result.data;
    },
  });
  const me = useQuery({ queryKey: ['post-viewer', session?.tokens.session_id], enabled: !!session, queryFn: async () => {
    const r = await apiClient.GET('/api/v1/auth/me'); if (r.error) throw apiError(r.error); return r.data;
  } });
  const items: SavedPost[] = enabled ? (posts.data?.items ?? []).map(p => ({ ...p, tags: p.tags ?? [], photos: p.photos ?? [] })) : []; const total = enabled ? posts.data?.total ?? 0 : 0;
  async function refresh() { await cache.invalidateQueries({ queryKey: ['posts'] }); }
  async function remove(post: SavedPost) {
    if (!window.confirm(`Переместить «${post.title}» в черновики? Пост исчезнет из лент.`)) return;
    setDeleting(post.id); setError('');
    try { const r = await apiClient.DELETE('/api/v1/posts/{id}', { params: { path: { id: post.id } } }); if (r.error) throw apiError(r.error); setPaging(p => ({ ...p, page: 1 })); await refresh(); }
    catch (e) { setError(apiError(e).message); } finally { setDeleting(''); }
  }
  const title = !member ? 'Интересное рядом' : section === 'favorites' ? 'Избранное' : section === 'archives' ? 'Архив страниц' : 'Все публикации';
  return <AppShell viewer={session ? { kind: 'member', username: session.login, displayName: session.login } : guestViewer}>
    <Breadcrumbs items={[{ label: member ? 'Личное пространство' : 'Сообщество' }, { label: member ? 'Библиотека' : 'Публичная лента' }]} />
    {session && <nav className="feed-switcher" aria-label="Выбор ленты"><Link to="/feed" search={{ section: 'all' }} aria-current={member ? 'page' : undefined}>Моя лента</Link><Link to="/" aria-current={!member ? 'page' : undefined}>Публичная лента</Link></nav>}
    <PageHeading title={title} count={total} description={member ? 'Ссылки, мысли и фотографии, к которым хочется вернуться.' : 'Открытые публикации сообщества.'} actions={session ? <Button onClick={() => setEditing('new')}><Plus size={18} />Создать пост</Button> : <Button asChild><Link to="/register">Создать свою библиотеку</Link></Button>} />
    {section !== 'all' ? <EmptyState>Этот раздел появится позже. Ваши посты доступны в разделе «Все публикации».</EmptyState> : <>
      <div className="search-controls">{!tagMode && <SearchPanel value={query} onChange={v => { setQuery(v); setPaging(p => ({ ...p, page: 1 })); }} label="Поиск публикаций" placeholder="Поиск по названию или тексту…" />}
        <Button variant={tagMode ? 'default' : 'outline'} role="switch" aria-checked={tagMode} onClick={() => { setTagMode(v => !v); setPaging(p => ({ ...p, page: 1 })); }}>Поиск по тегам</Button>
      </div>
      {tagMode && <div className="search-tags">
        <TagPicker label="Имеющиеся теги" allowCreate={false} disabled={false} value={includedTags} onChange={v => { setIncludedTags(v); setPaging(p => ({ ...p, page: 1 })); }} />
        <TagPicker label="Исключенные теги" allowCreate={false} disabled={false} value={excludedTags} onChange={v => { setExcludedTags(v); setPaging(p => ({ ...p, page: 1 })); }} />
      </div>}
      <FilterPanel sort={{ value: sort, options: [{ value: 'new', label: 'Сначала новые' }, { value: 'old', label: 'Сначала старые' }], onChange: v => { setSort(v as 'new' | 'old'); setPaging(p => ({ ...p, page: 1 })); } }}><FilterGroup label="Тип контента" options={filters} value={kind} onChange={v => { setKind(v); setPaging(p => ({ ...p, page: 1 })); }} /></FilterPanel>
      {posts.isLoading && <p role="status">Загружаем публикации…</p>}
      {(posts.error || error) && <p role="alert">{error || apiError(posts.error).message}<Button variant="outline" onClick={() => { setError(''); void posts.refetch(); }}>Повторить</Button></p>}
      <div className="post-feed">{items.map(post => <article className="post-card" key={post.id}>
        <div className="post-heading"><span className="post-category">{post.type === 'note' ? 'Заметка' : post.type === 'photo' ? 'Фото' : 'Ссылка'}</span>{session && me.data?.user_id === post.author_id && <div className="post-actions"><Button variant="icon" aria-label={`Редактировать ${post.title}`} onClick={() => setEditing(post)}><Pencil size={18} /></Button><Button variant="icon" aria-label={`Удалить ${post.title}`} disabled={!!deleting} onClick={() => void remove(post)}><Trash2 size={18} /></Button></div>}</div>
        <h2>{post.title}</h2>
        {post.type === 'note' && <div className="markdown-body"><ReactMarkdown remarkPlugins={[remarkGfm]} skipHtml disallowedElements={['img']} urlTransform={url => /^https?:\/\//i.test(url) ? url : ''} components={{ a: props => <a {...props} target="_blank" rel="noopener noreferrer" /> }}>{post.markdown}</ReactMarkdown></div>}
        {post.type === 'link' && <LinkPost post={post} />}
        {post.type === 'photo' && <PhotoGallery photos={post.photos} />}
        <footer><TagList tags={post.tags.map(t => t.title)} onTagClick={v => { const tag = post.tags.find(t => t.title === v); if (tag) setIncludedTags([tag]); setExcludedTags([]); setTagMode(true); setPaging(p => ({ ...p, page: 1 })); }} /><span className="post-meta">{new Date(post.created_at).toLocaleDateString('ru-RU')} · {post.author} · {post.visibility === 'public' ? 'Публично' : 'Только я'}</span></footer>
      </article>)}</div>
      {!posts.isLoading && !posts.error && items.length === 0 && <EmptyState>Публикаций пока нет. Создайте пост или измените фильтры.</EmptyState>}
      <Pagination total={total} page={paging.page} pageSize={paging.size} pageSizes={[2, 10, 20, 50]} onPageChange={page => setPaging(p => ({ ...p, page }))} onPageSizeChange={size => setPaging({ page: 1, size })} />
    </>}
    {editing && session && <PostForm post={editing === 'new' ? undefined : editing} onClose={() => setEditing(null)} onSaved={() => { setEditing(null); setPaging(p => ({ ...p, page: 1 })); void refresh(); if (!member || section !== 'all') void navigate({ to: '/feed', search: { section: 'all' } }); }} />}
  </AppShell>;
}
