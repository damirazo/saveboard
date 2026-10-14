import type { ReactNode } from 'react';
import { Link } from '@tanstack/react-router';
import { sessionClient } from '@saveboard/api-client';
import type { Viewer } from './viewer';
import { NotificationBell } from '../../features/notifications/notification-bell';

export function AppHeader({ viewer, title, actions }: { viewer: Viewer; title?: ReactNode; actions?: ReactNode }) {
  return <header className="app-topbar"><span>{title ?? (viewer.kind === 'member' ? 'Личное пространство' : 'Место для того, что хочется сохранить')}</span><div><NotificationBell />{actions ?? (viewer.kind === 'member' ? <button type="button" onClick={() => { void sessionClient.logout().catch(() => { window.alert("Локальный вход завершён, но не удалось отозвать сессию на сервере."); }); }}>Выйти</button> : <><Link to="/login">Войти</Link><Link to="/register" className="topbar-register">Регистрация</Link></>)}</div></header>;
}

export function TemplateNavigation() {
  return <footer className="template-nav" aria-label="Просмотр шаблонов"><span>UI-шаблоны</span><Link to="/">Гость</Link><Link to="/preview/member" search={{ section: 'all' }}>Пользователь</Link><Link to="/login">Вход</Link><Link to="/register">Регистрация</Link></footer>;
}
