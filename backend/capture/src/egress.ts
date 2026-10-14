import { lookup } from 'node:dns/promises';
import { BlockList, isIP } from 'node:net';
import http from 'node:http';
import https from 'node:https';
import { createHmac } from 'node:crypto';
import { mkdir,readFile,writeFile,rename,readdir,stat,unlink } from 'node:fs/promises';
import { dirname,join } from 'node:path';
import { gunzipSync,brotliDecompressSync,inflateSync } from 'node:zlib';

const denied = new BlockList();
for (const [net, bits] of [['0.0.0.0',8],['10.0.0.0',8],['100.64.0.0',10],['127.0.0.0',8],['169.254.0.0',16],['172.16.0.0',12],['192.0.0.0',24],['192.0.2.0',24],['192.168.0.0',16],['192.88.99.0',24],['198.18.0.0',15],['198.51.100.0',24],['203.0.113.0',24],['224.0.0.0',4],['240.0.0.0',4]] as const) denied.addSubnet(net,bits);
// Fail closed for IPv6, including mapped IPv4, NAT64 and transition ranges.
export function publicIP(ip: string) { return isIP(ip) === 4 && !denied.check(ip); }
export function target(raw: string) {
  const url = new URL(raw);
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.port || url.hostname.endsWith('.') || raw.length > 8192) throw new Error('blocked URL');
  return url;
}
export async function resolvePublic(host: string, resolve = lookup) {
  const addresses = await resolve(host, { all: true, family: 4 });
  if (!addresses.length || addresses.some(a => !publicIP(a.address))) throw new Error('blocked address');
  return addresses[0]!.address;
}
export type Download = { status: number; headers: Record<string,string>; data: string };
export async function download(raw: string, validators: Record<string,string> = {}): Promise<Download> {
  const url = target(raw); const ip = await resolvePublic(url.hostname);
  return new Promise((resolve,reject) => {
    // No second DNS lookup: connect only to the validated address; retain TLS SNI and Host.
    const req = (url.protocol === 'https:' ? https : http).request(url, {
      method: 'GET', agent: false, family: 4, lookup: (_host,_options,callback) => callback(null,ip,4),
      headers: { 'user-agent': 'Mozilla/5.0 SaveboardCapture/1.0', accept: '*/*', 'accept-encoding': 'identity', ...validators },
    }, res => {
      const parts: Buffer[] = []; let size = 0;
      res.on('data', (part: Buffer) => { size += part.length; if (size > 5*1024*1024) req.destroy(new Error('resource too large')); else parts.push(part); });
      res.on('error',reject);
      res.on('end', () => {
        let body=Buffer.concat(parts);
        try {
          const encoding=res.headers['content-encoding'];const options={maxOutputLength:5*1024*1024};
          if(encoding==='gzip')body=gunzipSync(body,options);
          else if(encoding==='br')body=brotliDecompressSync(body,options);
          else if(encoding==='deflate')body=inflateSync(body,options);
          else if(encoding&&encoding!=='identity')throw new Error('unsupported encoding');
        }catch{reject(new Error('invalid or oversized encoding'));return;}
        const headers: Record<string,string> = {};
        for (const name of ['content-type','location','etag','last-modified','access-control-allow-origin','access-control-allow-methods','access-control-allow-headers']) {
          const v = res.headers[name]; if (typeof v === 'string') headers[name] = v;
        }
        resolve({ status: res.statusCode ?? 502, headers, data: body.toString('base64') });
      });
    });
    const timer = setTimeout(() => req.destroy(new Error('download timeout')),12_000);
    req.on('close',()=>clearTimeout(timer)); req.on('error',reject); req.end();
  });
}

// Only anonymous CSS responses are shared. No credentials, cookies or arbitrary request headers cross this boundary.
export function cssDownloader(fetcher = download, root = '', salt = '') {
  const cache = new Map<string,{ at: number; value: Download; size: number }>();
  const inflight = new Map<string,Promise<Download>>(); let size = 0;
  return async (url: string, css: boolean) => {
    target(url);
    if (!css) return fetcher(url);
    const u=new URL(url); const hash=(s:string)=>createHmac('sha256',salt).update(s).digest('hex');
    const file=root ? join(root,hash(u.origin),hash(u.pathname),`${hash(u.search)}.json`) : '';
    let old = cache.get(url);
    if(!old && file)try{const record=JSON.parse(await readFile(file,'utf8')) as {at:number;value:Download;size:number};old=record;}catch{/* Cache miss. */}
    if (old && Date.now()-old.at < 86_400_000) return old.value;
    if (cache.has(url)) { size -= cache.get(url)!.size; cache.delete(url); }
    const running = inflight.get(url); if (running) return running;
    const validators:Record<string,string>={};
    if(old?.value.headers.etag)validators['if-none-match']=old.value.headers.etag;
    if(old?.value.headers['last-modified'])validators['if-modified-since']=old.value.headers['last-modified'];
    const work = fetcher(url,validators).then(async response => {
      const value=response.status===304 && old ? old.value : response;
      if (value.status === 200 && /^text\/css\b/i.test(value.headers['content-type'] ?? '')) {
        while (size+value.data.length > 64*1024*1024 && cache.size) { const key = cache.keys().next().value!; size -= cache.get(key)!.size; cache.delete(key); }
        const record={at:Date.now(),value,size:value.data.length};
        cache.set(url,record); size += value.data.length;
        if(file)try{await mkdir(dirname(file),{recursive:true});await writeFile(`${file}.tmp`,JSON.stringify(record),{mode:0o600});await rename(`${file}.tmp`,file);await pruneCache(root);}catch{/* A cache failure must not prevent capture. */}
      }
      return value;
    }).finally(()=>inflight.delete(url));
    inflight.set(url,work); return work;
  };
}

async function pruneCache(root:string) {
  const entries=await readdir(root,{recursive:true,withFileTypes:true});
  const files=await Promise.all(entries.filter(e=>e.isFile()&&e.name.endsWith('.json')).map(async e=>{const file=join(e.parentPath,e.name);const s=await stat(file);return {file,size:s.size,at:s.mtimeMs};}));
  let total=files.reduce((n,f)=>n+f.size,0);
  for(const f of files.sort((a,b)=>a.at-b.at)){if(total<=64*1024*1024)break;await unlink(f.file).catch(()=>{});total-=f.size;}
}
