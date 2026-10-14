import { createHash, createHmac } from 'node:crypto';
import { JSDOM } from 'jsdom';
import postcss from 'postcss';
import values from 'postcss-value-parser';

export type Resource = { url: string; mime: string; data: Buffer };
export type Asset = { mime: string; data: string; path: string };
export const offlineCSP = "default-src 'none'; script-src 'none'; style-src 'unsafe-inline' data:; img-src data:; font-src data:; media-src 'none'; frame-src 'none'; connect-src 'none'; form-action 'none'; base-uri 'none'";
const emptyImage = 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7';
export function snapshot(html: string, base: string, resources: Map<string,Resource>, salt: string) {
  const assets: Record<string,Asset> = {}; const ids = new Map<string,string>();
  const hash = (s:string) => createHmac('sha256',salt).update(s).digest('hex');
  for (const [url,r] of resources) {
    if (!/^(text\/css|image\/(png|jpeg|gif|webp|avif|bmp|svg\+xml|x-icon|vnd.microsoft.icon)|font\/|application\/(font|vnd.ms-fontobject))/i.test(r.mime)) continue;
    ids.set(url,createHash('sha256').update(url).digest('hex'));
  }
  const link = (raw:string, origin:string) => {
    if (raw.startsWith('#')) return raw;
    if (/^data:(image\/(png|jpeg|gif|webp|avif|bmp|svg\+xml)|font\/(woff2?|ttf|otf));base64,[a-z0-9+/=]+$/i.test(raw)) return raw;
    try { const u = new URL(raw,origin); const fragment=u.hash;u.hash=''; const id=ids.get(u.href);return id ? `sbasset:${id}${fragment}` : emptyImage; } catch { return emptyImage; }
  };
  const value = (text:string, origin:string) => {
    const parsed=values(text);
    parsed.walk(node => { if(node.type==='function' && node.value.toLowerCase()==='url') { const raw=values.stringify(node.nodes).replace(/^['"]|['"]$/g,'');node.nodes=[{type:'string',quote:'"',value:link(raw,origin),sourceIndex:0,sourceEndIndex:0}];return false; } return undefined; });
    return parsed.toString();
  };
  const css = (text:string,origin:string) => {
    try {
      const tree=postcss.parse(text);
      tree.walkDecls(d=>{d.value=value(d.value,origin);});
      tree.walkAtRules(rule=>{
        if(rule.name.toLowerCase()==='import') {
          const p=values(rule.params); const first=p.nodes[0];
          if(first?.type==='string') first.value=link(first.value,origin);
          rule.params=value(p.toString(),origin);
        } else if (rule.name.toLowerCase()==='charset') rule.remove();
      });
      return tree.toString();
    } catch { return ''; }
  };
  for (const [url,id] of ids) {
    const r=resources.get(url)!;const u=new URL(url);
    const content=r.mime.startsWith('text/css') ? Buffer.from(css(r.data.toString('utf8'),r.url)) : r.data;
    // Salted hierarchy, with query and content versions to prevent overwriting distinct pages/assets.
    const path=`${hash(u.origin)}/${u.pathname.split('/').filter(Boolean).map(hash).join('/') || hash('/')}/${hash(u.search)}/${createHash('sha256').update(content).digest('hex')}${r.mime==='text/css' ? '.css' : ''}`;
    assets[id]={mime:r.mime,data:content.toString('base64'),path};
  }
  const dom=new JSDOM(html,{url:base});const doc=dom.window.document;
  const clean=(root:ParentNode)=>{
  root.querySelectorAll('script,iframe,frame,frameset,object,embed,portal,base,meta,noscript,audio,video,source,track,animate,animateMotion,animateTransform,set').forEach(e=>e.remove());
  for (const el of root.querySelectorAll('*')) {
    if(el.tagName==='TEMPLATE')clean((el as HTMLTemplateElement).content);
    for (const attr of [...el.attributes]) {
      const name=attr.name.toLowerCase();
      if(name.startsWith('on') || ['srcdoc','action','formaction','ping','target','download','autofocus','contenteditable','nonce','integrity','crossorigin','srcset','imagesrcset','manifest','is'].includes(name)) el.removeAttribute(attr.name);
    }
    if(el.tagName==='LINK') {
      if(el.getAttribute('rel')?.toLowerCase()!=='stylesheet') {el.remove();continue;}
      el.setAttribute('href',link(el.getAttribute('href')??'',base));
    } else if(el.hasAttribute('href')) {
      if(!el.getAttribute('href')?.startsWith('#')) el.removeAttribute('href');
    }
    if(el.hasAttribute('xlink:href') && !el.getAttribute('xlink:href')?.startsWith('#')) el.removeAttribute('xlink:href');
    for (const attr of ['src','poster','background']) if(el.hasAttribute(attr)) el.setAttribute(attr,link(el.getAttribute(attr)!,base));
    if(el.hasAttribute('style')) el.setAttribute('style',value(el.getAttribute('style')!,base));
    if(el.tagName==='STYLE') el.textContent=css(el.textContent??'',base);
    if(['INPUT','BUTTON','SELECT','TEXTAREA'].includes(el.tagName)) el.setAttribute('disabled','');
  }
  };
  clean(doc);
  const policy=doc.createElement('meta');policy.httpEquiv='Content-Security-Policy';policy.content=offlineCSP;doc.head.prepend(policy);
  const charset=doc.createElement('meta');charset.setAttribute('charset','utf-8');doc.head.prepend(charset);
  const result={html:dom.serialize(),assets};dom.window.close();return result;
}
