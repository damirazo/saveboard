import { useEffect, useRef, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@saveboard/api-client';
import { X } from 'lucide-react';
import { useSession } from '../auth/use-session';
import { apiError } from '../library/post-api';
import { AppShell } from '../../components/layout/app-shell';
import { guestViewer } from '../../components/layout/viewer';
import { PageHeading } from '../../components/content/page-heading';
import { Pagination } from '../../components/navigation/pagination';
import { Button } from '../../components/ui/button';

export function NotificationsPage() {
  const session = useSession();
  const [page, setPage] = useState(1); const [selected, setSelected] = useState<string | null>(null);
  const list = useQuery({ queryKey: ['notifications', session?.tokens.session_id, 'list', page], enabled: !!session, refetchInterval: 15000,
    queryFn: async ({ signal }) => { const r = await apiClient.GET('/api/v1/notifications', { params: { query: { page, size: 10 } }, signal }); if (r.error || !r.data) throw apiError(r.error); return r.data; },
  });
  return <AppShell viewer={session ? { kind: 'member', username: session.login, displayName: session.login } : guestViewer}>
    <PageHeading title="Уведомления" description="Результаты фоновой обработки и сообщения сервиса." />
    {list.isPending && <p role="status">Загружаем уведомления…</p>}
    {list.isError && <p role="alert">Не удалось загрузить уведомления. <Button onClick={() => void list.refetch()}>Повторить</Button></p>}
    {list.data && <><ul className="notification-list">{(list.data.items ?? []).map(n => <li key={n.id}><button type="button" className={n.read ? '' : 'is-unread'} onClick={() => setSelected(n.id)}>{!n.read && <span className="unread-dot" aria-label="Непрочитанное" />}{n.title}</button></li>)}</ul>
      {list.data.total === 0 && <p>Уведомлений пока нет.</p>}
      <Pagination total={list.data.total} page={page} pageSize={10} onPageChange={setPage} />
    </>}
    {selected && <NotificationDialog key={selected} id={selected} onClose={() => setSelected(null)} />}
  </AppShell>;
}

const importance = { normal: 'Обычное', important: 'Важное', critical: 'Критическое' };
function NotificationDialog({ id, onClose }: { id: string; onClose: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null); const session = useSession(); const cache = useQueryClient();
  useEffect(() => { const element = dialog.current; element?.showModal(); return () => element?.close(); }, []);
  const detail = useQuery({ queryKey: ['notifications', session?.tokens.session_id, 'detail', id], enabled: !!session,
    queryFn: async ({ signal }) => { const r = await apiClient.GET('/api/v1/notifications/{id}', { params: { path: { id } }, signal }); if (r.error || !r.data) throw apiError(r.error); return r.data; },
  });
  const read = useMutation({ mutationFn: async () => { const r = await apiClient.POST('/api/v1/notifications/{id}/read', { params: { path: { id } } }); if (r.error) throw apiError(r.error); },
    onSuccess: () => cache.invalidateQueries({ queryKey: ['notifications', session?.tokens.session_id] }),
  });
  const { mutate, isIdle } = read;
  useEffect(() => { if (detail.data && !detail.data.read && isIdle) mutate(); }, [detail.data, isIdle, mutate]);
  return <dialog ref={dialog} className="notification-dialog" aria-labelledby="notification-title" onCancel={e => { e.preventDefault(); onClose(); }}>
    <header><h2 id="notification-title">{detail.data?.title ?? 'Уведомление'}</h2><Button variant="icon" aria-label="Закрыть уведомление" onClick={onClose}><X /></Button></header>
    {detail.isPending && <p role="status">Загружаем…</p>}
    {detail.isError && <p role="alert">Не удалось открыть уведомление. <Button onClick={() => void detail.refetch()}>Повторить</Button></p>}
    {detail.data && <><p className={`notification-importance ${detail.data.importance}`}>{importance[detail.data.importance]}</p><p className="notification-text">{detail.data.text}</p>{detail.data.post_id && <Link to="/posts/$postId" params={{ postId: detail.data.post_id }}>Открыть пост</Link>}</>}
    {read.isError && <p role="alert">Не удалось отметить прочитанным. <Button onClick={() => read.mutate()}>Повторить</Button></p>}
  </dialog>;
}
