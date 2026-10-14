import { expect,it,vi } from 'vitest';
import { publicIP,target,resolvePublic,cssDownloader,download } from './egress.js';
import { mkdtemp,rm,readdir } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
it('rejects private, reserved, metadata, alternate numeric and IPv6 addresses',async()=>{
  for(const ip of ['127.0.0.1','10.1.2.3','172.16.0.1','192.168.1.1','169.254.169.254','100.100.100.200','0.0.0.0','224.0.0.1','198.18.0.1','::1','::ffff:8.8.8.8','2001:4860:4860::8888'])expect(publicIP(ip),ip).toBe(false);
  expect(publicIP('8.8.8.8')).toBe(true);
  for(const url of ['file:///etc/passwd','ftp://example.com','http://user:pass@example.com','http://example.com:8080','http://example.com.'])expect(()=>target(url)).toThrow();
  for(const url of ['http://2130706433','http://0x7f000001','http://127.1','http://[::1]'])await expect(download(url)).rejects.toThrow();
});
it('rejects DNS rebinding answers before opening a socket',async()=>{
  const resolver=vi.fn().mockResolvedValueOnce([{address:'8.8.8.8',family:4}]).mockResolvedValueOnce([{address:'127.0.0.1',family:4}]);
  expect(await resolvePublic('example.com',resolver)).toBe('8.8.8.8');
  await expect(resolvePublic('example.com',resolver)).rejects.toThrow('blocked');
});
it('shares only CSS downloads, including simultaneous requests',async()=>{
  const fetcher=vi.fn().mockResolvedValue({status:200,headers:{'content-type':'text/css'},data:'Ym9keXt9'});
  const get=cssDownloader(fetcher);
  await Promise.all([get('https://example.com/style.css',true),get('https://example.com/style.css',true)]);
  await get('https://example.com/style.css',true);expect(fetcher).toHaveBeenCalledTimes(1);
  await get('https://example.com/style.css?v=2',true);expect(fetcher).toHaveBeenCalledTimes(2);
  await get('https://example.com/page',false);await get('https://example.com/page',false);expect(fetcher).toHaveBeenCalledTimes(4);
});

it('reuses persisted CSS after a gateway restart without another download',async()=>{
  const root=await mkdtemp(join(tmpdir(),'saveboard-css-'));
  try{
    const response={status:200,headers:{'content-type':'text/css',etag:'v1'},data:Buffer.from('body{color:red}').toString('base64')};
    const first=vi.fn().mockResolvedValue(response);
    await cssDownloader(first,root,'test-salt')('https://example.com/css/main.css?v=1',true);
    const second=vi.fn();const get=cssDownloader(second,root,'test-salt');
    expect(await get('https://example.com/css/main.css?v=1',true)).toEqual(response);expect(second).not.toHaveBeenCalled();
    expect((await readdir(root)).every(name=>/^[a-f0-9]{64}$/.test(name))).toBe(true);
  }finally{await rm(root,{recursive:true,force:true});}
});
