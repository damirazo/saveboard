import { timingSafeEqual } from 'node:crypto';
import type { IncomingMessage } from 'node:http';
export function authorized(req: IncomingMessage, token: string) {
  const actual = Buffer.from(req.headers.authorization ?? ''); const expected = Buffer.from(`Bearer ${token}`);
  return actual.length === expected.length && timingSafeEqual(actual,expected);
}
export async function readJSON(req: IncomingMessage): Promise<unknown> {
  const parts: Buffer[] = []; let size = 0;
  for await (const part of req) { size += part.length; if (size > 16_384) throw new Error('body too large'); parts.push(part); }
  return JSON.parse(Buffer.concat(parts).toString());
}
