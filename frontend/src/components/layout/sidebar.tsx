import { useId, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { Layers, Menu, X, LayoutList, Star, Archive, LockKeyhole, LogIn, UserRound } from 'lucide-react';
import type { Viewer } from './viewer';
import { Button } from '../ui/button';
// Navigation and profile are shared by every template. Viewer is supplied by the
// caller; it is presentation state, never proof of API authorization.
export function Sidebar({ viewer }: { viewer: Viewer }) {
  const contentId = useId();
  const [expanded, setExpanded] = useState(false);
  return <aside className="app-sidebar">
    <div className="sidebar-brand"><Link to="/" className="brand"><Layers aria-hidden="true" />Saveboard<span>.</span></Link><Button variant="icon" className="mobile-menu" aria-label={expanded ? 'Закрыть меню' : 'Открыть меню'} aria-expanded={expanded} aria-controls={contentId} onClick={() => setExpanded(!expanded)}>{expanded ? <X /> : <Menu />}</Button></div>
    <div id={contentId} className={`sidebar-content ${expanded ? 'is-expanded' : ''}`}>
      <div className="sidebar-profile"><span className="avatar">{viewer.kind === 'member' ? viewer.displayName.slice(0, 1) : <UserRound aria-hidden="true" />}</span><div><strong>{viewer.kind === 'member' ? viewer.displayName : 'Добро пожаловать'}</strong><small>{viewer.kind === 'member' ? `@${viewer.username}` : 'Вы просматриваете как гость'}</small></div></div>
      <nav aria-label="Основное меню"><p className="sidebar-label">{viewer.kind === 'member' ? 'ВАШЕ ПРОСТРАНСТВО' : 'ОТКРОЙТЕ ДЛЯ СЕБЯ'}</p>
        {viewer.kind === 'member' ? <>
          <Link to="/feed" search={{ section: 'all' }} activeOptions={{ exact: true, includeSearch: true }}><LayoutList />Все публикации</Link>
          <Link to="/feed" search={{ section: 'favorites' }} activeOptions={{ exact: true, includeSearch: true }}><Star />Избранное</Link>
          <Link to="/feed" search={{ section: 'archives' }} activeOptions={{ exact: true, includeSearch: true }}><Archive />Архив страниц</Link>
        </> : <><Link to="/" activeOptions={{ exact: true }}><LayoutList />Публичная лента</Link><Link to="/login"><LogIn />Войти в аккаунт</Link></>}
      </nav>
      <div className="sidebar-note">{viewer.kind === 'member' ? <><LockKeyhole /><span>Личное пространство<small>Доступ к каждому посту выбираете вы</small></span></> : <><p>Соберите свою библиотеку ссылок, заметок и файлов.</p><Link to="/register" className="sidebar-register">Создать аккаунт</Link></>}</div>
    </div>
  </aside>;
}
