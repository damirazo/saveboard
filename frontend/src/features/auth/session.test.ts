import { beforeEach, expect, it, vi } from 'vitest';
import { createSessionClient } from '../../../packages/api-client/src/session';

const tokens = (access = 'access', expires = 3600) => ({ access_token: access, refresh_token: `refresh-${access}`, expires_in: expires, refresh_expires_at: new Date(Date.now() + 86400000).toISOString(), session_id: 'session', token_type: 'Bearer' });
beforeEach(() => sessionStorage.clear());

it('stores tokens without the password and restores through an authorized me request', async () => {
  const transport = vi.fn<typeof fetch>().mockResolvedValueOnce(Response.json(tokens())).mockResolvedValueOnce(Response.json({ user_id: 'user' }));
  const client = createSessionClient(sessionStorage, transport);
  await client.login('alex', 'secret-password');
  expect(sessionStorage.getItem('saveboard.session')).not.toContain('secret-password');
  const restored = createSessionClient(sessionStorage, transport);
  await restored.restore();
  expect((transport.mock.calls[1]![0] as Request).headers.get('Authorization')).toBe('Bearer access');
  expect(restored.getSnapshot()?.login).toBe('alex');
});

it('rotates once for concurrent requests and uses the new bearer token', async () => {
  const transport = vi.fn<typeof fetch>().mockResolvedValueOnce(Response.json(tokens('old', 0))).mockResolvedValueOnce(Response.json(tokens('new'))).mockImplementation(async () => Response.json({}));
  const client = createSessionClient(sessionStorage, transport);
  await client.login('alex', 'password');
  await Promise.all([client.fetch('/api/v1/auth/me'), client.fetch('/api/v1/system')]);
  expect(transport.mock.calls.filter(([url]) => url === '/api/v1/auth/refresh')).toHaveLength(1);
  for (const [request] of transport.mock.calls.slice(2)) expect((request as Request).headers.get('Authorization')).toBe('Bearer new');
  expect(client.getSnapshot()?.tokens.refresh_token).toBe('refresh-new');
});

it('replays a rejected POST once, preserving its body, and clears a revoked session', async () => {
  const transport = vi.fn<typeof fetch>().mockResolvedValueOnce(Response.json(tokens())).mockResolvedValueOnce(new Response(null, { status: 401 })).mockResolvedValueOnce(Response.json(tokens('new'))).mockResolvedValueOnce(new Response(null, { status: 401 }));
  const client = createSessionClient(sessionStorage, transport);
  await client.login('alex', 'password');
  const response = await client.fetch('/api/v1/auth/logout', { method: 'POST', body: 'payload' });
  expect(response.status).toBe(401);
  expect(await (transport.mock.calls[3]![0] as Request).text()).toBe('payload');
  expect(client.getSnapshot()).toBeNull();
  expect(sessionStorage.getItem('saveboard.session')).toBeNull();
});

it('restores a revoked session as a guest and does not send tokens to another origin', async () => {
  const transport = vi.fn<typeof fetch>().mockResolvedValueOnce(Response.json(tokens())).mockResolvedValueOnce(Response.json({})).mockResolvedValueOnce(new Response(null, { status: 401 })).mockResolvedValueOnce(new Response(null, { status: 401 }));
  const client = createSessionClient(sessionStorage, transport);
  await client.login('alex', 'password');
  await client.fetch('https://example.com/api/test');
  expect((transport.mock.calls[1]![0] as Request).headers.has('Authorization')).toBe(false);
  const restored = createSessionClient(sessionStorage, transport);
  await restored.restore();
  expect(restored.getSnapshot()).toBeNull();
});
it('registers and stores a session without retaining credentials', async () => {
  const transport = vi.fn<typeof fetch>().mockResolvedValueOnce(Response.json(tokens(), { status: 201 }));
  const client = createSessionClient(sessionStorage, transport);
  await client.register(' alice ', ' alice@example.com ', 'secret-password');
  expect(transport.mock.calls[0]![0]).toBe('/api/v1/auth/register');
  expect(JSON.parse(String(transport.mock.calls[0]![1]?.body))).toEqual({ username: 'alice', email: 'alice@example.com', password: 'secret-password' });
  expect(client.getSnapshot()?.login).toBe('alice');
  expect(sessionStorage.getItem('saveboard.session')).not.toContain('secret-password');
});

it('reports duplicate registration and network errors without creating a session', async () => {
  const transport = vi.fn<typeof fetch>().mockResolvedValueOnce(new Response(null, { status: 409 })).mockRejectedValueOnce(new TypeError('Failed to fetch'));
  const client = createSessionClient(sessionStorage, transport);
  await expect(client.register('alice', 'alice@example.com', 'secret-password')).rejects.toThrow('уже зарегистрирован');
  await expect(client.login('alice', 'secret-password')).rejects.toThrow('связаться с сервером');
  expect(client.getSnapshot()).toBeNull();
  expect(sessionStorage.getItem('saveboard.session')).toBeNull();
});
