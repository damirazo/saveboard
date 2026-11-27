import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@saveboard/api-client';
import { useSession } from '../auth/use-session';
import { apiError, type SavedPost } from './post-api';

export function usePost(id?: string) {
  const session = useSession();
  return useQuery<SavedPost>({
    queryKey: ['posts', session?.tokens.session_id, id],
    enabled: !!id,
    refetchInterval: query => query.state.data?.capture_state === 'pending' ? 3000 : false,
    queryFn: async ({ signal }) => {
      const result = await apiClient.GET('/api/v1/posts/{id}', { params: { path: { id: id! } }, signal });
      if (result.error || !result.data) throw apiError(result.error);
      return { ...result.data, photos: result.data.photos ?? [], tags: result.data.tags ?? [] };
    },
  });
}
