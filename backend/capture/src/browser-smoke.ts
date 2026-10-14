// Run inside the isolated capture container: node capture/dist/browser-smoke.js
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { capture } from './capture.js';
import { readJSON } from './rpc.js';
import { cssDownloader,type Download } from './egress.js';

const token=process.env.CAPTURE_TOKEN!;
const gateway=process.env.CAPTURE_EGRESS_URL!;
let cssRequests=0;const blocked=new Set<string>();
const fixtures:Record<string,{mime:string;body:string}>={
  '/a':{mime:'text/html',body:'<!doctype html><link rel="stylesheet" href="/css/a.css"><h1 id="title">Before JS</h1><div id="shadow"></div><script src="/app.js"></script><img src="http://169.254.169.254/latest/meta-data/"><img src="/redirect">'},
  '/b':{mime:'text/html',body:'<!doctype html><link rel="stylesheet" href="/css/a.css"><h1>Second page</h1>'},
  '/article':{mime:'text/html',body:'<!doctype html><title>RAG fixture</title><nav>MENU NOISE</nav><article><h1>RAG fixture</h1><p id="article"></p></article><footer>FOOTER NOISE</footer><script>document.getElementById("article").textContent="Rendered article about saving knowledge and retrieving relevant documents. ".repeat(20)</script>'},
  '/css/a.css':{mime:'text/css',body:'@import "./b.css";h1{background:rgb(4,5,6);font-size:30px}'},
  '/css/b.css':{mime:'text/css',body:'h1{color:rgb(1,2,3)}'},
  '/app.js':{mime:'text/javascript',body:'document.getElementById("title").textContent="After JS";const root=document.getElementById("shadow").attachShadow({mode:"open"});root.innerHTML="<style>p{color:red}</style><p>Shadow rendered</p>";'},
};
const fixtureDownload=cssDownloader(async (url):Promise<Download>=>{
  const u=new URL(url);
  if(u.hostname!=='fixture.test'){
    const response=await fetch(`${gateway}/fetch`,{method:'POST',headers:{authorization:`Bearer ${token}`,'content-type':'application/json'},body:JSON.stringify({url,css:false})});
    assert.equal(response.status,422);blocked.add(url);throw new Error('blocked');
  }
  if(u.pathname==='/redirect')return {status:302,headers:{location:'http://127.0.0.1/private'},data:''};
  const fixture=fixtures[u.pathname];if(!fixture)return {status:404,headers:{},data:''};
  if(fixture.mime==='text/css')cssRequests++;
  return {status:200,headers:{'content-type':fixture.mime},data:Buffer.from(fixture.body).toString('base64')};
});
const server=createServer(async(req,res)=>{
  try{const input=await readJSON(req) as {url:string;css:boolean};res.end(JSON.stringify(await fixtureDownload(input.url,input.css)));}
  catch{res.writeHead(422).end('{}');}
}).listen(0,'127.0.0.1');
await once(server,'listening');
const addr=server.address();assert(addr&&typeof addr!=='string');
try{
  const fixtureGateway=`http://127.0.0.1:${addr.port}`;
  const first=await capture({url:'https://fixture.test/a',save_copy:true,create_thumbnail:true},fixtureGateway,token);
  assert(first.html.includes('After JS'));assert(!first.html.includes('Before JS'));assert(!first.html.includes('<script'));
  assert(first.html.includes('shadowrootmode="open"'));assert(first.html.includes('Shadow rendered'));
  assert.equal(Buffer.from(first.thumbnail,'base64').readUInt32BE(16),800);assert.equal(Buffer.from(first.thumbnail,'base64').readUInt32BE(20),600);
  assert(Object.values(first.assets).filter(a=>a.mime==='text/css').length===2);assert(first.warning);
  assert(blocked.has('http://169.254.169.254/latest/meta-data/'));assert(blocked.has('http://127.0.0.1/private'),'redirect target was not checked');
  const second=await capture({url:'https://fixture.test/b',save_copy:true,create_thumbnail:false},fixtureGateway,token);
  assert.equal(second.thumbnail,'');assert.equal(cssRequests,2,'shared CSS was downloaded again');
  const thumb=await capture({url:'https://fixture.test/b',save_copy:false,create_thumbnail:true},fixtureGateway,token);
  assert.equal(thumb.html,'');assert(thumb.thumbnail.length>0);
  const extracted=await capture({url:'https://fixture.test/article',save_copy:false,create_thumbnail:false},fixtureGateway,token,true);
  assert(extracted.article);assert(extracted.article.text.includes('Rendered article'));assert(!extracted.article.text.includes('MENU NOISE'));assert(!extracted.article.text.includes('FOOTER NOISE'));
  assert.equal(extracted.html,'');assert.equal(extracted.thumbnail,'');
  for(const url of ['http://127.0.0.1','http://10.0.0.1','http://169.254.169.254','http://[::1]','http://2130706433','http://localhost','http://192.168.0.1:8080']){
    const r=await fetch(`${gateway}/fetch`,{method:'POST',headers:{authorization:`Bearer ${token}`,'content-type':'application/json'},body:JSON.stringify({url,css:false})});assert.equal(r.status,422);
  }
  await assert.rejects(fetch('https://example.com',{signal:AbortSignal.timeout(2000)}));
  await assert.rejects(fetch('http://1.1.1.1',{signal:AbortSignal.timeout(2000)}));
  console.log('PASS: rendered DOM, shadow DOM, 800x600 thumbnail, independent flags, shared CSS, SSRF subresource and direct egress isolation');
}finally{server.closeAllConnections();await new Promise<void>(resolve=>server.close(()=>resolve()));}
