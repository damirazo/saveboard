import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { afterEach, expect, it, vi } from 'vitest';
import { SystemPage } from './system-page';

afterEach(() => vi.unstubAllGlobals());
it('shows unavailable API honestly instead of reporting a ready system', async () => {
  vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('offline')));
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, retryDelay: 0 } } });
  render(<QueryClientProvider client={client}><SystemPage /></QueryClientProvider>);
  expect(await screen.findByText('Бэкенд недоступен')).toBeInTheDocument();
  expect(screen.getByRole('link', { name: 'Документация API' })).toHaveAttribute('href', '/api/docs');
});
