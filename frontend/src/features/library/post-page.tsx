import { Link, useParams, useRouter } from '@tanstack/react-router';
import { Publication } from './publication';
import { AppShell } from '../../components/layout/app-shell';

import { usePost } from './use-post';

export function PostPage() {
  const { postId } = useParams({ from: '/posts/$postId' });
  const query = usePost(postId);
  const post = query.data;
  const router = useRouter();
  return <AppShell>
    <Link className="back-posts" to="/" onClick={event => {
      if (router.history.canGoBack() && !event.ctrlKey && !event.metaKey && !event.shiftKey && !event.altKey) { event.preventDefault(); router.history.back(); }
    }}>← К публикациям</Link>
    {query.isPending && <p role="status">Загружаем пост…</p>}
    {query.isError && <p role="alert">Пост недоступен или не удалось его загрузить.</p>}
    {post && <Publication post={post} detail /> }
  </AppShell>;
}
