import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider } from '@tanstack/react-router';
import { router } from './app/router';
import './styles.css';
import { sessionClient } from '@saveboard/api-client';
const queryClient = new QueryClient({ defaultOptions: { queries: { staleTime: 15_000, retry: 1 } } });
let previousSession = sessionClient.getSnapshot()?.tokens.session_id;
sessionClient.subscribe(() => {
  const nextSession = sessionClient.getSnapshot()?.tokens.session_id;
  if (previousSession !== nextSession) {
    previousSession = nextSession;
    queryClient.clear();
    void router.invalidate();
  }
});
createRoot(document.getElementById('root')!).render(<StrictMode><QueryClientProvider client={queryClient}><RouterProvider router={router} /></QueryClientProvider></StrictMode>);
