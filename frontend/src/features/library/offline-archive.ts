import type { components } from '@saveboard/api-client';

// Assets are loaded through the authenticated API once. The opaque sandbox never receives credentials.
export function archiveDocument(bundle:components['schemas']['CaptureResult']) {
  const urls=new Map<string,string>();const visiting=new Set<string>();const assets=bundle.assets ?? {};
  const rewrite=(text:string):string=>text.replace(/sbasset:([a-f0-9]{64})/g,(_match,id:string)=>assetURL(id));
  const assetURL=(id:string):string=>{
    const old=urls.get(id);if(old)return old;
    const asset=assets[id];if(!asset||visiting.has(id))return 'data:,';
    visiting.add(id);
    const bytes=Uint8Array.from(atob(asset.data),c=>c.charCodeAt(0));
    // An opaque sandbox cannot read the parent's blob URLs. Embed resources as data URLs;
    // their persistent originals remain deduplicated in CAS, not duplicated in storage.
    let encoded=asset.data;
    if(asset.mime==='text/css') {
      const css=new TextEncoder().encode(rewrite(new TextDecoder().decode(bytes)));
      if(css.length>32*1024*1024)throw new Error('Копия содержит слишком много вложенных стилей');
      let binary='';for(let i=0;i<css.length;i+=8192)binary+=String.fromCharCode(...css.subarray(i,i+8192));
      encoded=btoa(binary);
    }
    const result=`data:${asset.mime};base64,${encoded}`;
    visiting.delete(id);urls.set(id,result);return result;
  };
  const html=rewrite(bundle.html);
  if(html.length>64*1024*1024)throw new Error('Копия слишком велика для просмотра');
  return {html,dispose:()=>urls.clear()};
}
