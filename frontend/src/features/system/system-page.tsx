import { api } from '@saveboard/api-client';
import { BookMarked, ArrowUpRight, RefreshCw, Circle } from 'lucide-react';
import { Button } from '../../components/ui/button';

export function SystemPage() {
  const system = api.useQuery('get', '/api/v1/system');
  const health = api.useQuery('get', '/api/health/ready', undefined, { refetchInterval: 30_000, retry: 1 });
  const status = health.isPending ? 'Проверяем соединение…' : health.isError ? 'Бэкенд недоступен' : 'API и PostgreSQL доступны';
  return <div className="min-h-screen bg-stone-50 text-stone-900">
    <header className="border-b border-stone-200 bg-white">
      <div className="mx-auto flex max-w-5xl items-center justify-between px-6 py-5">
        <a href="/" className="flex items-center gap-3 text-xl font-semibold tracking-tight"><BookMarked className="text-emerald-800" />Saveboard</a>
        <span className="rounded-full bg-stone-100 px-3 py-1 text-xs text-stone-600">Среда разработки</span>
      </div>
    </header>
    <main className="mx-auto max-w-5xl px-6 py-16">
      <p className="mb-4 text-xs font-semibold uppercase tracking-[0.2em] text-emerald-800">Основа проекта</p>
      <h1 className="max-w-2xl text-4xl font-semibold leading-tight tracking-tight sm:text-5xl">Место для того,<br />что хочется сохранить.</h1>
      <p className="mt-6 max-w-xl text-lg leading-relaxed text-stone-600">Здесь появятся заметки, сохранённые страницы и поиск по вашей библиотеке. Сейчас подключена базовая инфраструктура приложения.</p>
      <section aria-label="Состояние подключения" className="mt-12 rounded-2xl border border-stone-200 bg-white p-6">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div><h2 className="font-semibold">Соединение с приложением</h2><p role="status" className="mt-2 flex items-center gap-2 text-sm text-stone-600"><Circle size={9} className={health.isSuccess ? 'fill-emerald-600 text-emerald-600' : 'fill-amber-500 text-amber-500'} />{status}</p></div>
          <Button variant="outline" disabled={health.isFetching} onClick={() => { void health.refetch(); void system.refetch(); }}><RefreshCw size={15} />Проверить</Button>
        </div>
        {system.data && <p className="mt-5 border-t border-stone-100 pt-4 text-sm text-stone-500">Вход, сохранение заметок и поиск ещё не включены.</p>}
      </section>
      <div className="mt-8 flex flex-wrap gap-3"><Button asChild><a href="/api/docs" target="_blank" rel="noreferrer">Документация API<ArrowUpRight size={16} /></a></Button><Button variant="outline" asChild><a href="/api/openapi.json" target="_blank" rel="noreferrer">OpenAPI JSON<ArrowUpRight size={16} /></a></Button></div>
    </main>
    <footer className="mx-auto max-w-5xl px-6 pb-8 text-xs text-stone-400">Saveboard · foundation 0.1</footer>
  </div>;
}
