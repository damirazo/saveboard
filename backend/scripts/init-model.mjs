import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { sameDigest } from './model-digest.mjs';

const lock = JSON.parse(readFileSync(new URL('../config/embedding-model.json', import.meta.url), 'utf8'));
const base = process.env.OLLAMA_URL ?? 'http://ollama:11434';
async function request(path, body, timeout = 120_000) {
  const response = await fetch(`${base}${path}`, {
    method: body === undefined ? 'GET' : 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(timeout),
  });
  if (!response.ok) throw new Error(`Ollama ${path}: HTTP ${response.status}`);
  return response;
}
const existing = await (await request('/api/tags')).json();
if (!existing.models.some(m => m.name === lock.localName && sameDigest(m.digest, lock.digest))) {
  // The upstream tag is mutable: refuse a changed manifest before downloading.
  const manifest = await fetch(lock.sourceManifest, { signal: AbortSignal.timeout(30_000) });
  if (!manifest.ok) throw new Error(`Model registry: HTTP ${manifest.status}`);
  const digest = `sha256:${createHash('sha256').update(Buffer.from(await manifest.arrayBuffer())).digest('hex')}`;
  if (digest !== lock.digest) throw new Error('Upstream model changed. Review and update the model lock explicitly.');
  console.log('Downloading locked BGE-M3 model (~1.2 GB)...');
  await (await request('/api/pull', { model: lock.source, stream: false }, 30*60_000)).json();
  const tags = await (await request('/api/tags')).json();
  if (!tags.models.some(m => m.name === lock.source && sameDigest(m.digest, lock.digest))) throw new Error('Downloaded model digest does not match lock');
  await request('/api/copy', { source: lock.source, destination: lock.localName });
}
const result = await (await request('/api/embed', { model: lock.localName, input: ['Сохранённая заметка', 'Saved note'], truncate: false })).json();
if (result.embeddings?.length !== 2 || !result.embeddings.every(v => v.length === lock.dimensions && v.every(Number.isFinite))) throw new Error('Invalid embedding dimensions or values');
console.log(`Embedding smoke passed: ${lock.localName}, ${lock.dimensions} dimensions. Quality evaluation remains P0.3.`);
