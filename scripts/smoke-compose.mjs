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
assert.equal(info.capture, true);
assert.equal(info.semanticSearch, false);

const contract = await get('/api/openapi.json');
assert.equal(contract.status, 200);
const paths = (await contract.json()).paths;
for (const path of ['/api/v1/system', '/api/v1/posts', '/api/v1/posts/{id}', '/api/v1/post-tags', '/api/v1/photos', '/api/v1/photos/{id}']) {
  assert.ok(paths[path], `missing API contract: ${path}`);
}
const posts = await get('/api/v1/posts?scope=public');
assert.equal(posts.status, 200);
const feed = await posts.json();
assert.ok(Array.isArray(feed.items));
assert.equal(typeof feed.total, 'number');
assert.ok(feed.items.every(post => post.status === 'published' && post.visibility === 'public'));
assert.equal((await get('/api/v1/posts?scope=mine')).status, 401);
assert.equal((await get('/api/v1/post-tags?q=test')).status, 401);
const guestCreate = await fetch(`${base}/api/v1/posts`, {
  method: 'POST', headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ title: 'Smoke access check', type: 'note', visibility: 'private', markdown: 'Test', tags: [{ id: '', title: 'Test' }], url: '', photo_ids: [] }),
  signal: AbortSignal.timeout(10_000),
});
assert.equal(guestCreate.status, 401);
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
console.log('Live Compose smoke passed: nginx, SPA asset, API readiness, post feed/access checks, OpenAPI, route isolation and Keycloak discovery.');
