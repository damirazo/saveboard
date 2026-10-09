import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const env = readFileSync('.env', 'utf8');
const port = env.match(/^HTTP_PORT=(\d+)\s*$/m)?.[1] ?? '8080';
const base = process.env.SMOKE_BASE_URL ?? `http://localhost:${port}`;

async function get(path) {
  return fetch(`${base}${path}`, { signal: AbortSignal.timeout(10_000) });
}

const ready = await get('/api/health/ready');
assert.equal(ready.status, 200);
assert.equal((await ready.json()).status, 'ready');
assert.equal(ready.headers.get('cache-control'), 'no-store');

const system = await get('/api/v1/system');
assert.equal(system.status, 200);
const info = await system.json();
assert.equal(info.stage, 'foundation');
assert.equal(info.authentication, true);
assert.equal(info.capture, false);
assert.equal(info.semanticSearch, false);

const contract = await get('/api/openapi.json');
assert.equal(contract.status, 200);
assert.ok((await contract.json()).paths['/api/v1/system']);
const missing = await get('/api/not-implemented');
assert.equal(missing.status, 404);
assert.ok(!missing.headers.get('content-type')?.includes('text/html'), 'API must not fall through to SPA');

const page = await get('/');
assert.equal(page.status, 200);
const html = await page.text();
assert.ok(html.includes('<div id="root">'));
const asset = html.match(/src="(\/assets\/[^\"]+\.js)"/)?.[1];
assert.ok(asset, 'built JS asset missing');
assert.equal((await get(asset)).status, 200);

const discovery = await get('/identity/realms/saveboard/.well-known/openid-configuration');
assert.equal(discovery.status, 200);
assert.equal((await discovery.json()).issuer, `${base}/identity/realms/saveboard`);
console.log('Live Compose smoke passed: nginx, SPA asset, API readiness, OpenAPI, route isolation and Keycloak discovery.');
