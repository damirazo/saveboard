import { useParams } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@saveboard/api-client';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { AppShell } from '../../components/layout/app-shell';
import { guestViewer } from '../../components/layout/viewer';
import { useSession } from '../auth/use-session';
import { apiError, type SavedPost } from './post-api';
import { LinkPost } from './link-post';
import { PhotoGallery } from './photo-gallery';

export function PostPage() {
  const { postId } = useParams({ from: '/posts/$postId' }); const session = useSession();
  const query = useQuery<SavedPost>({ queryKey: ['posts', session?.tokens.session_id, postId],
    refetchInterval: q => q.state.data?.capture_state === 'pending' ? 3000 : false,
    queryFn: async ({ signal }) => { const r = await apiClient.GET('/api/v1/posts/{id}', { params: { path: { id: postId } }, signal }); if (r.error || !r.data) throw apiError(r.error); return { ...r.data, photos: r.data.photos ?? [], tags: r.data.tags ?? [] }; },
  });
  const post = query.data;
  return <AppShell viewer={session ? { kind: 'member', username: session.login, displayName: session.login } : guestViewer}>
    {query.isPending && <p role="status">Загружаем пост…</p>}
    {query.isError && <p role="alert">Пост недоступен или не удалось его загрузить.</p>}
    {post && <article className="post-card"><h1>{post.title}</h1>
      {post.type === 'link' && <LinkPost post={post} />}
      {post.type === 'photo' && <PhotoGallery photos={post.photos} />}
      {post.type === 'note' && <div className="markdown-body"><ReactMarkdown remarkPlugins={[remarkGfm]} skipHtml disallowedElements={['img']} urlTransform={url => /^https?:\/\//i.test(url) ? url : ''} components={{ a: props => <a {...props} target="_blank" rel="noopener noreferrer" /> }}>{post.markdown}</ReactMarkdown></div>}
    </article>}
  </AppShell>;
}
