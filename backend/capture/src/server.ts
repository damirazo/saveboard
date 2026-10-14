import { createServer } from 'node:http';
import { authorized, readJSON } from './rpc.js';
import { capture, captureInput } from './capture.js';
import { z } from 'zod';

export function createCaptureServer(token: string, gateway = '', run = capture) {
  if (token.length < 32) throw new Error('CAPTURE_TOKEN must contain at least 32 characters');
  let active = false;
  return createServer(async (req, res) => {
    res.setHeader('Content-Type', 'application/json');
    res.setHeader('Cache-Control', 'no-store');
    if (req.url === '/health/live' && req.method === 'GET') {
      res.end(JSON.stringify({ status: 'ok', captureEnabled: !!gateway }));
      return;
    }
    if (!authorized(req,token)) {
      res.writeHead(401).end(JSON.stringify({ code: 'unauthorized' }));
      return;
    }
    if ((req.url === '/v1/captures' || req.url === '/v1/extract') && req.method === 'POST') {
      if (!gateway) { res.writeHead(501).end('{"code":"capture_not_enabled"}'); return; }
      if (active) { res.writeHead(429).end('{"code":"capture_busy"}'); return; }
      active = true;
      try {
        const extract = req.url === '/v1/extract';
        const raw = await readJSON(req);
        const input = extract ? { ...z.object({ url: z.string().max(4096) }).parse(raw), save_copy: false, create_thumbnail: false } : captureInput.parse(raw);
        const result = await run(input,gateway,token,extract);
        res.end(JSON.stringify(extract ? result.article : result));
      }
      catch { res.writeHead(422).end('{"code":"capture_failed"}'); }
      finally { active=false; }
      return;
    }
    res.writeHead(404).end(JSON.stringify({ code: 'not_found' }));
  });
}
