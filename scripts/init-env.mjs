import { randomBytes } from 'node:crypto';
import { readFileSync, writeFileSync, appendFileSync, existsSync } from 'node:fs';

if (existsSync('.env')) {
  const current = readFileSync('.env', 'utf8');
  const missing = ['AUTH_JWT_KEY', 'AUTH_ENCRYPTION_KEY'].filter(key => !new RegExp(`^${key}=`, 'm').test(current));
  if (missing.length) {
    appendFileSync('.env', '\n' + missing.map(key => `${key}=${randomBytes(32).toString('hex')}`).join('\n') + '\n');
  }
  console.log(missing.length ? 'Added missing auth keys; existing values preserved.' : '.env already exists; keeping it unchanged.');
} else {
  const template = readFileSync('.env.example', 'utf8');
  const result = template.replace(/replace-with-at-least-32-random-characters|replace-me/g, () => randomBytes(32).toString('hex'));
  writeFileSync('.env', result, { flag: 'wx', mode: 0o600 });
  console.log('Created .env with random local credentials.');
}
