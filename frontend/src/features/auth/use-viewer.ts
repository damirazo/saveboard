import { useQuery } from '@tanstack/react-query';
import { apiClient } from '@saveboard/api-client';
import { useSession } from './use-session';
import { apiError } from '../library/post-api';

export function useViewer() {
  const session = useSession();
  return useQuery({
    queryKey: ['viewer', session?.tokens.session_id],
    enabled: !!session,
    queryFn: async ({ signal }) => {
      const result = await apiClient.GET('/api/v1/auth/me', { signal });
      if (result.error || !result.data) throw apiError(result.error);
      return result.data;
    },
  });
}
