import { Link } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@saveboard/api-client';
import { Bell } from 'lucide-react';
import { useSession } from '../auth/use-session';
import { apiError } from '../library/post-api';

export function NotificationBell() {
  const session = useSession();
  const unread = useQuery({ queryKey: ['notifications', session?.tokens.session_id, 'unread'], enabled: !!session, refetchInterval: 15000,
    queryFn: async ({ signal }) => { const r = await apiClient.GET('/api/v1/notifications/unread', { signal }); if (r.error || !r.data) throw apiError(r.error); return r.data.count; },
  });
  if (!session) return null;
  return <Link to="/notifications" className="notification-bell" aria-label={unread.isError ? 'Уведомления: счётчик недоступен' : `Уведомления: ${unread.data ?? 0} непрочитанных`}><Bell size={21} /><span aria-hidden="true">{unread.isError ? '—' : unread.data ?? 0}</span></Link>;
}
