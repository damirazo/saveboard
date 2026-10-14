import type { components } from './schema';

type Tokens = components['schemas']['Tokens'];
type Session = { tokens: Tokens; expiresAt: number; login: string };
const storageKey = 'saveboard.session';

export function createSessionClient(storage: Storage, transport: typeof fetch) {
  let session: Session | null = null;
  let refreshPromise: Promise<void> | null = null;
  let restorePromise: Promise<void> | null = null;
  let restored = false;
  let revision = 0;
  const listeners = new Set<() => void>();
  try {
    const saved = JSON.parse(storage.getItem(storageKey) ?? 'null') as Session | null;
    if (saved?.tokens?.access_token && saved.tokens.refresh_token && Number.isFinite(saved.expiresAt) && typeof saved.login === 'string' && Date.parse(saved.tokens.refresh_expires_at) > Date.now()) session = saved;
  } catch { /* Invalid or unavailable storage starts a guest session. */ }

  function save(next: Session | null) {
    session = next;
    try { if (next) storage.setItem(storageKey, JSON.stringify(next)); else storage.removeItem(storageKey); } catch { /* Keep an in-memory session when storage is unavailable. */ }
    listeners.forEach(listener => listener());
  }
  function clear() { revision++; save(null); }
  async function refresh() {
    if (refreshPromise) return refreshPromise;
    const current = session;
    const version = revision;
    if (!current) throw new Error('Session unavailable');
    refreshPromise = (async () => {
      if (Date.parse(current.tokens.refresh_expires_at) <= Date.now()) { clear(); throw new Error('Session expired'); }
      const response = await transport('/api/v1/auth/refresh', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ refresh_token: current.tokens.refresh_token }) });
      if (!response.ok) {
        if (revision === version && (response.status === 401 || response.status === 422)) clear();
        throw new Error('Session refresh failed');
      }
      const tokens = await response.json() as Tokens;
      if (revision === version) save({ ...current, tokens, expiresAt: Date.now() + tokens.expires_in * 1000 });
    })().finally(() => { refreshPromise = null; });
    return refreshPromise;
  }
  const authenticatedFetch: typeof fetch = async (input, init) => {
    const request = new Request(input instanceof Request ? input : new URL(String(input), window.location.origin), init);
    const url = new URL(request.url);
    if (url.origin !== window.location.origin || !url.pathname.startsWith('/api/')) return transport(request);
    if (session && session.expiresAt <= Date.now() + 30_000) await refresh();
    const version = revision;
    const token = session?.tokens.access_token;
    const send = () => {
      const copy = request.clone();
      if (session) copy.headers.set('Authorization', `Bearer ${session.tokens.access_token}`);
      return transport(copy);
    };
    let response = await send();
    if (response.status === 401 && token && session && revision === version) {
      if (session.tokens.access_token === token) await refresh();
      if (!session || revision !== version) return response;
      response = await send();
      if (response.status === 401 && revision === version) clear();
    }
    return response;
  };
  async function startSession(path: '/api/v1/auth/login' | '/api/v1/auth/register', body: Record<string, string>, login: string) {
    const version = ++revision;
    let response: Response;
    try { response = await transport(path, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) }); }
    catch { throw new Error('Не удалось связаться с сервером. Проверьте соединение и попробуйте снова.'); }
    if (!response.ok) {
      if (response.status === 409) throw new Error('Этот username или email уже зарегистрирован. Войдите или укажите другие данные.');
      if (response.status === 401) throw new Error('Не удалось войти: неверный username/email или пароль, либо аккаунт недоступен.');
      if (response.status === 422) throw new Error('Проверьте username, email и пароль. Пароль: минимум 8 символов, максимум 72 байта.');
      if (response.status === 429) throw new Error('Слишком много попыток. Подождите немного и повторите.');
      throw new Error('Сервис авторизации временно недоступен. Попробуйте позже.');
    }
    const tokens = await response.json() as Tokens;
    if (revision === version) { restored = true; save({ tokens, login, expiresAt: Date.now() + tokens.expires_in * 1000 }); }
  }
  return {
    fetch: authenticatedFetch,
    subscribe(listener: () => void) { listeners.add(listener); return () => { listeners.delete(listener); }; },
    getSnapshot: () => session,
    async restore() {
      if (restored) return;
      if (!restorePromise) restorePromise = (async () => {
        if (session) {
          try {
            const response = await authenticatedFetch('/api/v1/auth/me');
            if (!response.ok && session) throw new Error('Не удалось проверить сессию. Обновите страницу.');
          } catch (error) { if (session) throw error; }
        }
        restored = true;
      })().finally(() => { restorePromise = null; });
      return restorePromise;
    },
    async login(login: string, password: string) {
      return startSession('/api/v1/auth/login', { login: login.trim(), password }, login.trim());
    },
    async register(username: string, email: string, password: string) {
      return startSession('/api/v1/auth/register', { username: username.trim(), email: email.trim(), password }, username.trim());
    },
    async logout() {
      try {
        const response = await authenticatedFetch('/api/v1/auth/logout', { method: 'POST' });
        if (!response.ok && response.status !== 401) throw new Error('Logout failed');
      } finally { clear(); }
    },
  };
}
