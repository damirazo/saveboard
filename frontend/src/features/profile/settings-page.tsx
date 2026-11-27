import { useState, type FormEvent } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { apiClient } from '@saveboard/api-client';
import { AppShell } from '../../components/layout/app-shell';
import { Avatar } from '../../components/layout/sidebar';
import { PageHeading } from '../../components/content/page-heading';
import { FormField } from '../../components/ui/form-field';
import { Button } from '../../components/ui/button';
import { useViewer } from '../auth/use-viewer';
import { apiError } from '../library/post-api';

export function SettingsPage() {
  const me = useViewer();
  const cache = useQueryClient();
  const [draft, setDraft] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');
  const [error, setError] = useState('');
  async function save(event: FormEvent) {
    event.preventDefault();
    if (!draft?.trim() || busy) return;
    setBusy(true); setError(''); setMessage('');
    try {
      const result = await apiClient.PATCH('/api/v1/auth/me', { body: { display_name: draft.trim() } });
      if (result.error) throw apiError(result.error);
      await Promise.all(['viewer', 'profile', 'posts'].map(key => cache.invalidateQueries({ queryKey: [key] })));
      setDraft(null); setMessage('Имя сохранено.');
    } catch (e) { setError(apiError(e).message); } finally { setBusy(false); }
  }
  return <AppShell><PageHeading title="Настройки" />
    {me.isPending && <p role="status">Загружаем профиль…</p>}
    {me.isError && <p role="alert">{apiError(me.error).message} <Button onClick={() => void me.refetch()}>Повторить</Button></p>}
    {me.data && <form className="settings-card" onSubmit={save}><h2>Профиль</h2>
      <div className="identity"><Avatar name={me.data.display_name || me.data.username} /><span>@{me.data.username}</span></div>
      <FormField label="Отображаемое имя" required maxLength={100} value={draft ?? me.data.display_name} onChange={e => { setDraft(e.target.value); setMessage(''); }} />
      <div className="settings-actions"><Button variant="outline" disabled={busy || draft === null} onClick={() => { setDraft(null); setMessage(''); setError(''); }}>Отмена</Button>
      <Button type="submit" disabled={busy || !draft?.trim() || draft.trim() === me.data.display_name}>{busy ? 'Сохраняем…' : 'Сохранить изменения'}</Button></div>
      {message && <p role="status">{message}</p>}{error && <p role="alert">{error}</p>}
    </form>}</AppShell>;
}
