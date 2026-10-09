import { useSyncExternalStore } from 'react';
import { sessionClient } from '@saveboard/api-client';

export function useSession() {
  return useSyncExternalStore(sessionClient.subscribe, sessionClient.getSnapshot);
}
