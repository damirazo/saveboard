import type { ReactNode } from 'react';
import { Link } from '@tanstack/react-router';
import { Globe, LockKeyhole } from 'lucide-react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { Avatar } from '../../components/layout/sidebar';
import { PostTypeBadge } from '../../components/content/post-type-badge';
import { postTypes } from '../../components/content/post-types';
import { LinkPost } from './link-post';
import { PhotoGallery } from './photo-gallery';
import type { SavedPost } from './post-api';

export function Publication({ post, detail, actions }: { post: SavedPost; detail?: boolean; actions?: ReactNode }) {
  const author = post.author_name || post.author;
  const publicPost = post.visibility === 'public';
  const kind = postTypes.find(t => t.value === post.type)?.label ?? post.type;
  const date = new Date(post.created_at);
  return <article className={detail ? 'post-card post-detail' : 'post-card'}>
    <header className="post-heading"><PostTypeBadge type={post.type} />
      <span className="visibility">{publicPost ? <Globe /> : <LockKeyhole />}{publicPost ? 'Публично' : 'Только я'}</span>
    </header>
    {detail ? <h1>{post.title}</h1> : <h2><Link to="/posts/$postId" params={{ postId: post.id }}>{post.title}</Link></h2>}
    {post.type === 'note' && <div className="markdown-body"><ReactMarkdown remarkPlugins={[remarkGfm]} skipHtml disallowedElements={['img']} urlTransform={url => /^https?:\/\//i.test(url) ? url : ''} components={{ a: props => <a {...props} target="_blank" rel="noopener noreferrer" /> }}>{post.markdown}</ReactMarkdown></div>}
    {post.type === 'link' && <LinkPost post={post} />}
    {post.type === 'photo' && <PhotoGallery photos={post.photos} />}
    <div className="publication-tags" aria-label="Теги поста">
      <Link to="/profiles/$username" params={{ username: post.author }}>@{post.author}</Link>
      <span>#{kind.toLocaleLowerCase('ru')}</span>
      {post.tags.map(tag => <Link key={tag.id} to="/search" search={{ tag: tag.id, label: tag.title, scope: publicPost ? undefined : 'mine' }} aria-label={`# ${tag.title}`}>#{tag.title}</Link>)}
    </div>
    <footer><div className="post-meta"><Link className="identity" to="/profiles/$username" params={{ username: post.author }}><Avatar name={author} />{author}</Link>
      <time dateTime={post.created_at}>{date.toLocaleDateString('ru-RU')} · {date.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })}</time></div>
      {actions}<span className="comments-off" aria-label="Комментарии недоступны" title="Комментарии недоступны">—</span>
    </footer>
  </article>;
}
