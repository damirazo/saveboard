import { chromium } from 'playwright';
import { z } from 'zod';
import { target, type Download } from './egress.js';
import { snapshot, type Resource } from './snapshot.js';
import { extractText } from './extract.js';
export const captureInput=z.object({url:z.string().max(4096),save_copy:z.boolean(),create_thumbnail:z.boolean()}).refine(v=>v.save_copy||v.create_thumbnail);
export type CaptureInput=z.infer<typeof captureInput>;
export async function capture(input:CaptureInput, gateway:string, token:string, extract = false) {
  target(input.url);
  const browser=await chromium.launch({headless:true,args:['--disable-quic','--disable-background-networking','--force-webrtc-ip-handling-policy=disable_non_proxied_udp']});
  const deadline=setTimeout(()=>void browser.close(),75_000);
  try {
    const context=await browser.newContext({viewport:{width:800,height:600},serviceWorkers:'block',acceptDownloads:false});
    await context.routeWebSocket('**/*',ws=>ws.close());
    const resources=new Map<string,Resource>();let requests=0;let bytes=0;let missing=0;
    const load=async(raw:string,css:boolean)=>{
      let url=target(raw).href;
      for(let redirects=0;redirects<=5;redirects++){
        const response=await fetch(`${gateway}/fetch`,{method:'POST',headers:{authorization:`Bearer ${token}`,'content-type':'application/json'},body:JSON.stringify({url,css}),signal:AbortSignal.timeout(15_000)});
        if(!response.ok)throw new Error('resource blocked');
        const result=await response.json() as Download;
        if([301,302,303,307,308].includes(result.status)&&result.headers.location){url=target(new URL(result.headers.location,url).href).href;continue;}
        if(result.status>=300&&result.status<400)throw new Error('unsupported redirect');
        return {url,result};
      }
      throw new Error('too many redirects');
    };
    // Never deliver an HTTP redirect to Chromium: routing may skip redirect targets.
    // Resolve each hop through the pinned-IP gateway, then navigate to the final document URL.
    let initial:Awaited<ReturnType<typeof load>>|undefined=await load(input.url,false);
    const initialURL=initial.url;
    await context.route('**/*',async route=>{
      try {
        const request=route.request();
        if(request.method()!=='GET'||++requests>250) {missing++;await route.abort();return;}
        const url=target(request.url()).href;
        const loaded=initial && url===initialURL ? initial : await load(url,request.resourceType()==='stylesheet');
        if(loaded===initial)initial=undefined;
        const r=loaded.result;const data=Buffer.from(r.data,'base64');bytes+=data.length;
        if(bytes>24*1024*1024) throw new Error('page too large');
        let mime=(r.headers['content-type']??'application/octet-stream').split(';')[0]!.toLowerCase();
        if(request.resourceType()==='font' && mime==='application/octet-stream')mime=data.subarray(0,4).toString()==='wOF2' ? 'font/woff2' : 'font/woff';
        if(r.status===200) resources.set(url,{url:loaded.url,mime,data});
        await route.fulfill({status:r.status,headers:r.headers,body:data});
      } catch {missing++;await route.abort().catch(()=>{});}
    });
    const page=await context.newPage();
    context.on('page',other=>{if(other!==page)void other.close();});
    const response=await page.goto(initialURL,{waitUntil:'domcontentloaded',timeout:30_000});
    if(!response?.ok())throw new Error('page unavailable');
    await page.waitForLoadState('networkidle',{timeout:8_000}).catch(()=>{});
    // Trigger a bounded amount of lazy content, then return to the original viewport.
    for(let i=1;i<=6;i++){await page.evaluate(y=>window.scrollTo(0,y),i*600);await page.waitForTimeout(150);}
    await page.evaluate(()=>window.scrollTo(0,0));await page.waitForTimeout(500);
    if (extract) {
      const article = extractText(await page.content(), page.url());
      return { html: '', assets: {}, thumbnail: '', warning: '', article };
    }
    const thumbnail=input.create_thumbnail ? (await page.screenshot({type:'png',animations:'disabled',timeout:10_000})).toString('base64') : '';
    const embedded=await page.locator('iframe,object,embed,video,audio').count();
    const warning=missing||embedded ? 'Часть ресурсов или встроенных интерактивных блоков не вошла в статическую копию.' : '';
    if(!input.save_copy)return {html:'',assets:{},thumbnail,warning};
    // Capture current form values, canvas pixels and selected responsive image sources before serialization.
    await page.evaluate(()=>{
      const visit=(root:Document|ShadowRoot)=>{
        for(const el of root.querySelectorAll('*'))if(el.shadowRoot){
          visit(el.shadowRoot);
          const template=document.createElement('template');template.setAttribute('shadowrootmode','open');template.innerHTML=el.shadowRoot.innerHTML;
          el.append(template);
        }
        for(const sheet of root.adoptedStyleSheets){const style=document.createElement('style');style.textContent=Array.from(sheet.cssRules).map(r=>r.cssText).join('\n');(root instanceof Document ? root.head : root).append(style);}
      };
      visit(document);
      for(const img of document.querySelectorAll('img')){if(img.currentSrc)img.src=img.currentSrc;img.removeAttribute('srcset');img.loading='eager';}
      for(const canvas of document.querySelectorAll('canvas'))try{const img=document.createElement('img');img.src=canvas.toDataURL();img.width=canvas.width;img.height=canvas.height;img.className=canvas.className;img.style.cssText=canvas.style.cssText;canvas.replaceWith(img);}catch{/* Tainted canvases cannot be exported. */}
      for(const input of document.querySelectorAll('input')){if(input.type==='password')input.value='';input.setAttribute('value',input.value);if(input.checked)input.setAttribute('checked','');else input.removeAttribute('checked');}
      for(const area of document.querySelectorAll('textarea'))area.textContent=area.value;
      for(const option of document.querySelectorAll('option')){if(option.selected)option.setAttribute('selected','');else option.removeAttribute('selected');}
      for(const style of document.querySelectorAll('style'))try{if(style.sheet)style.textContent=Array.from(style.sheet.cssRules).map(r=>r.cssText).join('\n');}catch{/* Keep original CSS. */}
    });
    const result=snapshot(await page.content(),await page.evaluate(()=>document.baseURI),resources,token);
    if(Buffer.byteLength(result.html)>5*1024*1024)throw new Error('DOM too large');
    return {...result,thumbnail,warning};
  }finally{clearTimeout(deadline);await browser.close();}
}
