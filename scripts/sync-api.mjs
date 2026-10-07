import { copyFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
execFileSync('go', ['-C', 'backend', 'run', './cmd/openapi'], { cwd: root, stdio: 'inherit' });
copyFileSync(new URL('../backend/api/openapi.json', import.meta.url), new URL('../frontend/api/openapi.json', import.meta.url));
execFileSync(process.execPath, [
  'frontend/packages/api-client/node_modules/openapi-typescript/bin/cli.js',
  'frontend/api/openapi.json', '-o', 'frontend/packages/api-client/src/schema.d.ts',
], { cwd: root, stdio: 'inherit' });
