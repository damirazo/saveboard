import { lazy, Suspense, useRef, useState } from 'react';
import { useDropzone } from 'react-dropzone';
import { ImagePlus, X } from 'lucide-react';
import { Button } from '../../components/ui/button';
import { TagPicker } from './tag-picker';
import { usePhotoURLs } from './use-photo-urls';
import { apiError, savePost, uploadPhoto, type Photo, type PostWrite, type SavedPost } from './post-api';
import { postTypes } from '../../components/content/post-types';
const MarkdownEditor = lazy(() => import('./markdown-editor'));

export function PostForm({ post, username, onClose, onSaved }: { post?: SavedPost; username?: string; onClose: () => void; onSaved: () => void }) {
  const [title, setTitle] = useState(post?.title ?? '');
  const [type, setType] = useState<PostWrite['type']>((post?.type as PostWrite['type']) ?? 'note');
  const [visibility, setVisibility] = useState<PostWrite['visibility']>(post?.visibility === 'public' ? 'public' : 'private');
  const [tags, setTags] = useState<PostWrite['tags']>(post?.tags.map(t => ({ id: t.id, title: t.title })) ?? []);
  const [markdown, setMarkdown] = useState(post?.markdown ?? ''); const [url, setURL] = useState(post?.url ?? '');
  const [indexRAG, setIndexRAG] = useState(post?.index_rag ?? false);
  const [saveCopy, setSaveCopy] = useState(post?.save_copy ?? false);
  const [createThumbnail, setCreateThumbnail] = useState(post?.create_thumbnail ?? false);
  const [photos, setPhotos] = useState<Photo[]>(post?.photos ?? []); const urls = usePhotoURLs(photos);
  const [busy, setBusy] = useState(false); const [uploading, setUploading] = useState(false); const [error, setError] = useState('');
  const uploadLock = useRef(false);
  const dropzone = useDropzone({ disabled: busy || uploading, multiple: true, maxFiles: 32, maxSize: 10 * 1024 * 1024,
    accept: { 'image/jpeg': ['.jpg', '.jpeg'], 'image/png': ['.png'], 'image/gif': ['.gif'], 'image/bmp': ['.bmp'], 'image/webp': ['.webp'] },
    onDrop: async (files, rejected) => {
      if (uploadLock.current) return;
      if (rejected.length || photos.length + files.length > 32) { setError('До 32 фотографий: JPG, PNG, GIF, BMP, WebP; до 10 МБ каждая.'); return; }
      uploadLock.current = true; setUploading(true); setError('');
      try { for (const file of files) { const p = await uploadPhoto(file); setPhotos(current => [...current, p]); } }
      catch (e) { setError(apiError(e).message); } finally { uploadLock.current = false; setUploading(false); }
    },
  });
  async function submit(e: React.FormEvent) {
    e.preventDefault(); if (busy || uploading) return;
    if (!tags.length) { setError('Добавьте хотя бы один тег.'); return; }
    if (type === 'photo' && !photos.length) { setError('Добавьте хотя бы одну фотографию.'); return; }
    setBusy(true); setError('');
    try { await savePost({ title, type, visibility, tags, index_rag: type === 'link' && indexRAG, save_copy: type === 'link' && saveCopy, create_thumbnail: type === 'link' && createThumbnail, markdown: type === 'note' ? markdown : '', url: type === 'link' ? url : '', photo_ids: type === 'photo' ? photos.map(p => p.id) : [] }, post?.id); onSaved(); }
    catch (e) { setError(apiError(e).message); } finally { setBusy(false); }
  }
  return <form className="post-dialog editor-page" aria-labelledby="post-form-title" onSubmit={submit}>
      <header><div><h1 id="post-form-title">{post ? 'Редактировать пост' : 'Новая публикация'}</h1></div><Button variant="icon" aria-label="Закрыть форму" disabled={busy || uploading} onClick={onClose}><X /></Button></header>
      <fieldset disabled={busy || uploading}>
        <label className="type-field">Тип поста<select value={type} onChange={e => setType(e.target.value as PostWrite['type'])}>{postTypes.map(t => <option key={t.value} value={t.value}>{t.label}</option>)}</select></label>
        <label>Наименование *<input autoFocus required maxLength={200} value={title} onChange={e => setTitle(e.target.value)} placeholder="Дайте публикации название" /></label>
        {type === 'note' && <Suspense fallback={<p>Загрузка редактора…</p>}><MarkdownEditor value={markdown} onChange={setMarkdown} disabled={busy || uploading} /></Suspense>}
        {type === 'link' && <section className="link-options"><label>Адрес ссылки *<input type="url" required maxLength={4096} value={url} onChange={e => setURL(e.target.value)} placeholder="https://example.com" /></label><label className="capture-option"><input type="checkbox" checked={saveCopy} onChange={e => setSaveCopy(e.target.checked)} />Сохранить копию страницы</label><label className="capture-option"><input type="checkbox" checked={createThumbnail} onChange={e => setCreateThumbnail(e.target.checked)} />Создать эскиз страницы</label><label className="capture-option"><input type="checkbox" checked={indexRAG} onChange={e => setIndexRAG(e.target.checked)} />Индексация для RAG</label><small>Индексация очищает текст страницы и сохраняет его для поиска. Результат придёт в уведомлении.</small>{post?.rag_state === 'failed' && <small role="status">Индексация не завершилась. Сохраните пост с включённым флагом для повторной попытки.</small>}<small>Копия и эскиз появятся после фоновой обработки. Можно сохранить пост и продолжить работу.</small>{post?.capture_state === 'failed' && <small role="status">Не удалось обработать страницу. Сохраните пост, чтобы повторить попытку.</small>}</section>}
        {type === 'photo' && <section><div {...dropzone.getRootProps({ className: 'photo-dropzone', 'aria-label': 'Добавить фотографии' })}><input {...dropzone.getInputProps()} /><ImagePlus /><strong>{uploading ? 'Проверяем и загружаем…' : 'Перетащите фотографии или выберите файлы'}</strong><small>До 32 фото · JPG, PNG, GIF, BMP, WebP · до 10 МБ / 20 Мп · GIF: первый кадр</small></div><div className="photo-grid upload-grid">{photos.map((p, i) => <div className="upload-preview" key={p.id}>{urls[p.id] ? <img src={urls[p.id]} alt={`Загруженное фото ${i + 1}`} /> : <span>Загружаем превью…</span>}<button type="button" aria-label={`Удалить фото ${i + 1}`} onClick={() => setPhotos(current => current.filter(x => x.id !== p.id))}><X size={16} /></button></div>)}</div><small>{photos.length} / 32</small></section>}
        <TagPicker value={tags} onChange={setTags} disabled={busy || uploading} fixedTags={[...(username ? ['@' + username] : []), '#' + postTypes.find(t => t.value === type)!.label.toLocaleLowerCase('ru')]} />
        <label>Кто увидит пост<select value={visibility} onChange={e => setVisibility(e.target.value as PostWrite['visibility'])}><option value="private">Только я</option><option value="public">Все — публичная лента</option></select></label>
      </fieldset>
      {error && <p className="form-error" role="alert">{error}</p>}
      <footer><Button variant="outline" disabled={busy || uploading} onClick={onClose}>Отмена</Button><Button type="submit" disabled={busy || uploading}>{busy ? 'Сохраняем…' : post ? 'Сохранить изменения' : 'Опубликовать'}</Button></footer>
  </form>;
}
