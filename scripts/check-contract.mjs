import { readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const root = new URL('../', import.meta.url);
const files = ['backend/api/openapi.json', 'frontend/api/openapi.json', 'frontend/packages/api-client/src/schema.d.ts'].map(f => new URL(f, root));
const before = files.map(f => readFileSync(f, 'utf8'));
execFileSync(process.execPath, [fileURLToPath(new URL('./sync-api.mjs', import.meta.url))], { stdio: 'inherit' });
for (let i = 0; i < files.length; i++) {
  if (before[i] !== readFileSync(files[i], 'utf8')) throw new Error(`Generated API drift: ${files[i]}. Run pnpm api:generate and commit the outputs.`);
}
console.log('OpenAPI and TypeScript client match the Go contract.');
