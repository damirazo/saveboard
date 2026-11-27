import { useEffect, useId, useRef, useState } from 'react';
import { apiClient } from '@saveboard/api-client';
import { Input } from '../../components/ui/form-field';
import { X } from 'lucide-react';
import { apiError, type PostTag, type PostWrite } from './post-api';
type Selected = PostWrite['tags'];
export function TagPicker({ value, onChange, disabled, allowCreate = true, fixedTags = [], label = 'Теги *' }: { fixedTags?: string[]; allowCreate?: boolean; label?: string; value: Selected; onChange: (v: Selected) => void; disabled: boolean }) {
  const id = useId();
  const input = useRef<HTMLInputElement>(null);
  const [query, setQuery] = useState('');
  const [result, setResult] = useState<{ query: string; tags: PostTag[] }>({ query: '', tags: [] });
  const [error, setError] = useState('');
  const [cursor, setCursor] = useState(0);
  const [open, setOpen] = useState(false);
  const text = query.trim();
  useEffect(() => {
    if (!text) return;
    const controller = new AbortController();
    const timer = setTimeout(() => {
      apiClient.GET('/api/v1/post-tags', { params: { query: { q: text } }, signal: controller.signal }).then(r => {
        if (controller.signal.aborted) return;
        if (r.error) throw apiError(r.error);
        setResult({ query: text, tags: r.data ?? [] }); setError('');
      }).catch(e => { if (!controller.signal.aborted) setError(apiError(e).message); });
    }, 200);
    return () => { clearTimeout(timer); controller.abort(); };
  }, [text]);
  const ready = result.query === text && !error;
  const tags = ready ? result.tags : [];
  const exact = tags.some(t => t.title.toLowerCase() === text.toLowerCase() || t.name.toLowerCase() === text.toLowerCase());
  const options = [...tags, ...(allowCreate && ready && !exact ? [{ id: '', title: text }] : [])].filter(t => !value.some(v => (t.id && v.id === t.id) || v.title.toLowerCase() === t.title.toLowerCase()));
  function select(t: Selected[number]) { if (value.length >= 20) return; onChange([...value, { id: t.id, title: t.title }]); setQuery(''); setCursor(0); input.current?.focus(); setOpen(false); }
  return <div className="tag-picker">
    <label htmlFor={id}>{label}</label>
    <div className="selected-tags">{fixedTags.map(tag => <span key={tag}>{tag}</span>)}{value.map((t, i) => <span key={t.id || t.title}># {t.title}<button type="button" disabled={disabled} aria-label={`Удалить тег ${t.title}`} onClick={() => onChange(value.filter((_, n) => n !== i))}><X size={14} /></button></span>)}</div>
    <div className="tag-input-wrap" onBlur={event => { if (!event.currentTarget.contains(event.relatedTarget)) setOpen(false); }}>
    <Input ref={input} id={id} role="combobox" aria-expanded={open && !!text} aria-controls={`${id}-options`} aria-autocomplete="list" aria-activedescendant={open && text && options[cursor] ? `${id}-${cursor}` : undefined} value={query} maxLength={80} disabled={disabled || value.length >= 20} placeholder="Начните вводить тег…" onFocus={() => setOpen(true)} onChange={e => { setQuery(e.target.value); setCursor(0); setOpen(true); setError(''); }} onKeyDown={e => {
      if (e.key === 'ArrowDown') { e.preventDefault(); if (!open) { setOpen(true); setCursor(0); return; } setCursor(i => Math.max(0, Math.min(i + 1, options.length - 1))); }
      if (e.key === 'ArrowUp') { e.preventDefault(); setCursor(i => Math.max(0, i - 1)); }
      if (e.key === 'Enter') { e.preventDefault(); if (open && text && options[cursor]) select(options[cursor]); }
      if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); setOpen(false); }
    }} />
    {open && text && <ul id={`${id}-options`} role="listbox" className="tag-suggestions">
      {options.map((t, i) => <li id={`${id}-${i}`} key={t.id || 'create'} role="option" aria-selected={cursor === i}><button type="button" onClick={() => select(t)}>{t.id ? `# ${t.title}` : `+ Создать тег «${text}»`}</button></li>)}
      {ready && options.length === 0 && <li role="presentation">Нет доступных тегов</li>}
      {!ready && <li role="presentation">{error || 'Ищем теги…'}</li>}
    </ul>}</div>
    <small>{allowCreate ? 'Выберите хотя бы один тег, максимум 20. Новые теги сохраняются вместе с постом.' : 'Выберите существующие теги, максимум 20.'}</small>
  </div>;
}
