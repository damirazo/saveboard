import { once } from 'node:events';
import { expect, it } from 'vitest';
import { createCaptureServer } from './server.js';

it('rejects unauthenticated capture and never enables unverified browser execution', async () => {
  const token = 'test-token-'.repeat(4);
  const server = createCaptureServer(token).listen(0, '127.0.0.1');
  await once(server, 'listening');
  try {
    const addr = server.address();
    if (!addr || typeof addr === 'string') throw new Error('missing address');
    const url = `http://127.0.0.1:${addr.port}`;
    expect((await fetch(`${url}/v1/captures`, { method: 'POST' })).status).toBe(401);
    expect((await fetch(`${url}/v1/extract`, { method: 'POST' })).status).toBe(401);
    expect((await fetch(`${url}/v1/extract`, { method: 'POST', headers: { authorization: `Bearer ${token}` } })).status).toBe(501);
    expect((await fetch(`${url}/v1/captures`, { method: 'POST', headers: { authorization: `Bearer ${token}` } })).status).toBe(501);
    expect(await (await fetch(`${url}/health/live`)).json()).toEqual({ status: 'ok', captureEnabled: false });
  } finally { server.closeAllConnections(); await new Promise<void>((resolve, reject) => server.close(e => e ? reject(e) : resolve())); }
});
