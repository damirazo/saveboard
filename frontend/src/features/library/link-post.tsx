import { useEffect, useRef, useState } from 'react';
import { apiClient } from '@saveboard/api-client';
import { X } from 'lucide-react';
import { Button } from '../../components/ui/button';
import { apiError, type SavedPost } from './post-api';
import { usePhotoURLs } from './use-photo-urls';
import { archiveDocument } from './offline-archive';

export function LinkPost({ post }: { post: SavedPost }) {
  const [open,setOpen] = useState(false);
  const photos = post.thumbnail_url ? [{id:post.thumbnail_url,url:post.thumbnail_url}] : [];
  const urls = usePhotoURLs(photos);
  const display = Array.from(post.url); const label = display.length > 300 ? `${display.slice(0,299).join('')}…` : post.url;
  return <div className="link-post">
    {post.thumbnail_url && urls[post.thumbnail_url] && <img className="link-thumbnail" src={urls[post.thumbnail_url]} alt={`Эскиз страницы «${post.title}»`} width={800} height={600} />}
    {post.copy_url && <Button variant="outline" onClick={()=>setOpen(true)}>Показать копию</Button>}
    <a href={/^https?:\/\//i.test(post.url) ? post.url : undefined} target="_blank" rel="noopener noreferrer" className="post-link">{label}</a>
    {open && <ArchiveDialog post={post} onClose={()=>setOpen(false)} />}
  </div>;
}
function ArchiveDialog({post,onClose}:{post:SavedPost;onClose:()=>void}) {
  const dialog=useRef<HTMLDialogElement>(null);const [html,setHTML]=useState('');const [error,setError]=useState('');
  useEffect(()=>{const el=dialog.current;el?.showModal();return()=>el?.close();},[]);
  useEffect(()=>{
    const controller=new AbortController();let dispose:undefined|(()=>void);
    void (async()=>{
      try {
        const r=await apiClient.GET('/api/v1/posts/{id}/copy',{params:{path:{id:post.id}},signal:controller.signal});
        if(r.error||!r.data)throw apiError(r.error);
        if(controller.signal.aborted)return;
        const result=archiveDocument(r.data);dispose=result.dispose;setHTML(result.html);
      }catch(e){if(!controller.signal.aborted)setError(apiError(e).message);}
    })();
    return()=>{controller.abort();dispose?.();};
  },[post.id]);
  return <dialog ref={dialog} className="archive-dialog" aria-label={`Копия: ${post.title}`} onCancel={e=>{e.preventDefault();onClose();}}>
    <header><strong>{post.title}</strong><Button variant="icon" aria-label="Закрыть копию" onClick={onClose}><X /></Button></header>
    {post.capture_warning && <p role="status">{post.capture_warning}</p>}
    {error ? <p role="alert">{error}</p> : html ? <iframe title={`Сохранённая копия ${post.title}`} sandbox="" referrerPolicy="no-referrer" srcDoc={html} /> : <p role="status">Открываем копию…</p>}
  </dialog>;
}
