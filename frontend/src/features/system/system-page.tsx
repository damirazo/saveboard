import { api } from '@saveboard/api-client';
import { Bookmark, ArrowUpRight, RefreshCw, Circle } from 'lucide-react';
import { Button } from '../../components/ui/button';

export function SystemPage() {
  const system = api.useQuery('get', '/api/v1/system');
  const health = api.useQuery('get', '/api/health/ready', undefined, { refetchInterval: 30_000, retry: 1 });
  const status = health.isPending ? 'Проверяем соединение…' : health.isError ? 'Бэкенд недоступен' : 'API и PostgreSQL доступны';
  return <div className="min-h-screen bg-(--sb-background) text-(--sb-text)">
    <header className="border-b border-(--sb-border) bg-(--sb-surface)">
      <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-5">
        <a href="/" className="flex items-center gap-3 text-xl font-semibold tracking-tight"><Bookmark fill="currentColor" className="text-(--sb-primary)" />Saveboard</a>
        <span className="rounded-full bg-(--sb-raised) px-3 py-1 text-xs text-(--sb-muted)">Среда разработки</span>
      </div>
    </header>
    <main className="mx-auto max-w-5xl px-6 py-16">
      <h1 className="text-3xl font-semibold tracking-tight">Состояние системы</h1>
      <section aria-label="Состояние подключения" className="mt-6 rounded-2xl bg-(--sb-surface) p-6">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div><h2 className="font-semibold">Соединение с приложением</h2><p role="status" className="mt-2 flex items-center gap-2 text-sm text-(--sb-muted)"><Circle size={9} className={health.isSuccess ? 'fill-(--sb-primary) text-(--sb-primary)' : 'fill-(--sb-amber) text-(--sb-amber)'} />{status}</p></div>
          <Button variant="outline" disabled={health.isFetching} onClick={() => { void health.refetch(); void system.refetch(); }}><RefreshCw size={15} />Проверить</Button>
        </div>
        {system.data && <p className="mt-5 border-t border-(--sb-border) pt-4 text-sm text-(--sb-muted)">{system.data.name} · {system.data.stage}</p>}
      </section>
      <div className="mt-8 flex flex-wrap gap-3"><Button asChild><a href="/api/docs" target="_blank" rel="noreferrer">Документация API<ArrowUpRight size={16} /></a></Button><Button variant="outline" asChild><a href="/api/openapi.json" target="_blank" rel="noreferrer">OpenAPI JSON<ArrowUpRight size={16} /></a></Button></div>
    </main>

  </div>;
}
