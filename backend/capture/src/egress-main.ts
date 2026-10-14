import { createServer } from 'node:http';
import { z } from 'zod';
import { authorized, readJSON } from './rpc.js';
import { cssDownloader } from './egress.js';
const token = process.env.CAPTURE_TOKEN ?? '';
if (token.length < 32) throw new Error('CAPTURE_TOKEN required');
const fetchResource = cssDownloader(undefined,process.env.CAPTURE_CACHE_ROOT ?? '',token); let active = 0;
const server = createServer(async (req,res) => {
  res.setHeader('Content-Type','application/json');
  if (!authorized(req,token)) { res.writeHead(401).end('{}'); return; }
  if (req.method !== 'POST' || req.url !== '/fetch') { res.writeHead(404).end('{}'); return; }
  if (active >= 32) { res.writeHead(429).end('{}'); return; }
  active++;
  try { const input = z.object({url:z.string().max(8192),css:z.boolean()}).parse(await readJSON(req)); res.end(JSON.stringify(await fetchResource(input.url,input.css))); }
  catch { res.writeHead(422).end('{"error":"resource unavailable"}'); }
  finally { active--; }
});
server.requestTimeout=20_000;server.headersTimeout=5_000;server.listen(3002,'0.0.0.0');
