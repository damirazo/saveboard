import { useNavigate, useParams, useRouter } from '@tanstack/react-router';
import { useQueryClient } from '@tanstack/react-query';
import { AppShell } from '../../components/layout/app-shell';
import { Button } from '../../components/ui/button';
import { useViewer } from '../auth/use-viewer';
import { PostForm } from './post-form';
import { usePost } from './use-post';

export function EditorPage() {
  const { postId } = useParams({ strict: false });
  const navigate = useNavigate();
  const router = useRouter();
  const cache = useQueryClient();
  const me = useViewer();
  const post = usePost(postId);
  function library() { void navigate({ to: '/feed', search: { section: 'all' } }); }
  function close() { if (router.history.canGoBack()) router.history.back(); else library(); }
  if (postId && (post.isError || me.isError || (post.data && me.data && me.data.user_id !== post.data.author_id))) {
    return <AppShell><p role="alert">Редактирование недоступно. <Button onClick={library}>В библиотеку</Button></p></AppShell>;
  }
  if (postId && (!post.data || !me.data)) return <AppShell><p role="status">Загружаем публикацию…</p></AppShell>;
  return <AppShell><PostForm key={postId ?? 'new'} post={postId ? post.data : undefined} username={me.data?.username} onClose={close} onSaved={() => {
    void cache.invalidateQueries({ queryKey: ['posts'] });
    void cache.invalidateQueries({ queryKey: ['profile'] });
    library();
  }} /></AppShell>;
}
