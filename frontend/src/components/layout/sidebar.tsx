import { useId, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { Bookmark, House, LibraryBig, Menu, Search, Settings, UserRound, X } from 'lucide-react';
import type { Viewer } from './viewer';
import { Button } from '../ui/button';

export function Brand() {
  return <Link to="/" className="brand"><Bookmark aria-hidden="true" fill="currentColor" />Saveboard</Link>;
}

export function Avatar({ name }: { name: string }) {
  const initials = name.trim().split(/\s+/).slice(0, 2).map(word => Array.from(word)[0]).join('').toLocaleUpperCase('ru');
  return <span className="avatar" aria-hidden="true">{initials || <UserRound />}</span>;
}

export function Sidebar({ viewer }: { viewer: Viewer }) {
  const id = useId();
  const [expanded, setExpanded] = useState(false);
  const member = viewer.kind === 'member';
  const profile = member && <><Avatar name={viewer.displayName} /><strong>{viewer.displayName}</strong>{viewer.username && <small>@{viewer.username}</small>}</>;
  return <aside className="app-sidebar">
    <div className="sidebar-brand"><Brand /><Button variant="icon" className="mobile-menu" aria-label={expanded ? 'Закрыть меню' : 'Открыть меню'} aria-expanded={expanded} aria-controls={id} onClick={() => setExpanded(!expanded)}>{expanded ? <X /> : <Menu />}</Button></div>
    <div id={id} className={expanded ? 'sidebar-content is-expanded' : 'sidebar-content'} onClick={event => { if ((event.target as HTMLElement).closest('a')) setExpanded(false); }}>
      {member && (viewer.username ? <Link className="sidebar-profile" to="/profiles/$username" params={{ username: viewer.username }}>{profile}</Link> : <div className="sidebar-profile">{profile}</div>)}
      {!member && <div className="sidebar-profile guest-profile"><Avatar name="" /><strong>Гость</strong></div>}
      <nav aria-label="Основное меню">
        <Link to="/" activeOptions={{ exact: true }}><House />Главная страница</Link>
        {member && <Link to="/feed" search={{ section: 'all' }}><LibraryBig />Библиотека</Link>}
        <Link to="/search"><Search />Поиск</Link>
      </nav>
      {member && <nav className="sidebar-bottom" aria-label="Аккаунт"><Link to="/settings"><Settings />Настройки</Link></nav>}
    </div>
  </aside>;
}
