import { randomBytes } from 'node:crypto';
import { readFileSync, writeFileSync, existsSync } from 'node:fs';

if (existsSync('.env')) {
  console.log('.env already exists; keeping it unchanged.');
} else {
  const template = readFileSync('.env.example', 'utf8');
  const result = template.replace(/replace-with-at-least-32-random-characters|replace-me/g, () => randomBytes(32).toString('hex'));
  writeFileSync('.env', result, { flag: 'wx', mode: 0o600 });
  console.log('Created .env with random local credentials.');
}
