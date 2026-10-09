import type { ReactNode } from 'react';
import { Globe, NotebookPen, FileText, LockKeyhole, Star, Check, Link2 } from 'lucide-react';
import { Button } from '../ui/button';

type PostBase = { id: number; topic?: string; title: string; text: string; tags: readonly string[]; date: string; public: boolean; author: string };
export type Post = PostBase & (
  { kind: 'web'; preview?: { title: string; description?: string; archived?: boolean } } |
  { kind: 'text'; quote?: string } |
  { kind: 'file'; attachment?: { format: string; name: string; description?: string } }
);
const kinds = { web: { Icon: Globe, label: 'Веб-страница' }, text: { Icon: NotebookPen, label: 'Заметка' }, file: { Icon: FileText, label: 'Файл' } };

export function TagList({ tags, onTagClick }: { tags: readonly string[]; onTagClick?: (tag: string) => void }) {
  return <div className="post-tags">{tags.map(tag => onTagClick ? <Button key={tag} variant="tag" onClick={() => onTagClick(tag)}># {tag}</Button> : <span className="tag-button" key={tag}># {tag}</span>)}</div>;
}
export function LinkPreview({ title, description, archived }: { title: string; description?: string; archived?: boolean }) {
  return <div className="link-preview"><Link2 aria-hidden="true" /><div><strong>{title}</strong>{description && <small>{description}</small>}</div>{archived && <span><Check aria-hidden="true" />Копия сохранена</span>}</div>;
}
export function FilePreview({ format, name, description }: { format: string; name: string; description?: string }) {
  return <div className="file-preview"><span>{format}</span><div><strong>{name}</strong>{description && <small>{description}</small>}</div></div>;
}
export function PostCard({ post, showVisibility = false, favorite, onFavoriteChange, onTagClick, actions }: { post: Post; showVisibility?: boolean; favorite?: boolean; onFavoriteChange?: () => void; onTagClick?: (tag: string) => void; actions?: ReactNode }) {
  const { Icon, label } = kinds[post.kind];
  return <article className="post-card">
    <div className="post-heading"><span className={`post-category category-${post.kind}`}><Icon aria-hidden="true" />{label}{post.topic && <><span>/</span>{post.topic}</>}</span><div className="post-actions">{actions}{onFavoriteChange && <Button variant="icon" className="favorite-button" aria-label={favorite ? 'Убрать из избранного' : 'Добавить в избранное'} aria-pressed={!!favorite} onClick={onFavoriteChange}><Star /></Button>}</div></div>
    <h2>{post.title}</h2><p>{post.text}</p>
    {post.kind === 'web' && post.preview && <LinkPreview {...post.preview} />}
    {post.kind === 'text' && post.quote && <blockquote>{post.quote}</blockquote>}
    {post.kind === 'file' && post.attachment && <FilePreview {...post.attachment} />}
    <footer><TagList tags={post.tags} onTagClick={onTagClick} /><span className="post-meta">{post.date} · {showVisibility ? <>{post.public ? <Globe /> : <LockKeyhole />}{post.public ? 'Публично' : 'Только я'}</> : post.author}</span></footer>
  </article>;
}
