import { expect,it } from 'vitest';
import { JSDOM } from 'jsdom';
import { snapshot,type Resource } from './snapshot.js';
it('removes active content and rewrites styles, imports and images into immutable assets',()=>{
  const resources=new Map<string,Resource>();
  for(const [url,mime,data] of [['https://example.com/css/a.css','text/css','@import "./b.css";body{background:url(../i.png)}'],['https://example.com/css/b.css','text/css','p{color:red}'],['https://example.com/i.png','image/png','image']] as const)resources.set(url,{url,mime,data:Buffer.from(data)});
  const out=snapshot('<html><head><base href="https://evil.test"><meta http-equiv="refresh" content="0;url=https://evil.test"><link rel="stylesheet" href="/css/a.css"></head><body onload="alert(1)"><script>alert(1)</script><iframe src="https://evil.test"></iframe><img src="/i.png" srcset="https://evil.test/i.png 2x"><form action="https://evil.test"><input><button formaction="https://evil.test">send</button></form><a href="https://evil.test">link</a></body></html>','https://example.com/page',resources,'salt');
  const doc=new JSDOM(out.html).window.document;
  expect(doc.querySelector('script,iframe,base,[onload],[srcset],[action],[formaction]')).toBeNull();
  expect(doc.querySelector('meta[http-equiv=refresh]')).toBeNull();expect(doc.querySelector('input')?.hasAttribute('disabled')).toBe(true);
  expect(doc.querySelector('link')?.getAttribute('href')).toMatch(/^sbasset:/);
  const css=Object.values(out.assets).filter(a=>a.mime==='text/css').map(a=>Buffer.from(a.data,'base64').toString()).join('');expect(css).toContain('sbasset:');expect(css).not.toContain('../i.png');
  expect(out.html).toContain("script-src 'none'");
  expect(Object.values(out.assets).every(a=>!a.path.includes('example.com'))).toBe(true);
});
