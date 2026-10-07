import { createServer } from 'node:http';
import { timingSafeEqual } from 'node:crypto';

// Fail closed until the P0.2 browser egress and archive fidelity gates pass.
// No browser is launched and no user URL is fetched in this foundation.
export function createCaptureServer(token: string) {
  if (token.length < 32) throw new Error('CAPTURE_TOKEN must contain at least 32 characters');
  return createServer((req, res) => {
    res.setHeader('Content-Type', 'application/json');
    res.setHeader('Cache-Control', 'no-store');
    if (req.url === '/health/live' && req.method === 'GET') {
      res.end(JSON.stringify({ status: 'ok', captureEnabled: false }));
      return;
    }
    const actual = Buffer.from(req.headers.authorization ?? '');
    const expected = Buffer.from(`Bearer ${token}`);
    if (actual.length !== expected.length || !timingSafeEqual(actual, expected)) {
      res.writeHead(401).end(JSON.stringify({ code: 'unauthorized' }));
      return;
    }
    if (req.url === '/v1/captures' && req.method === 'POST') {
      res.writeHead(501).end(JSON.stringify({ code: 'capture_not_enabled', detail: 'P0.2 egress isolation and offline fidelity verification required' }));
      return;
    }
    res.writeHead(404).end(JSON.stringify({ code: 'not_found' }));
  });
}
