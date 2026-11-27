import type { ReactNode } from 'react';
import { guestViewer, type Viewer } from './viewer';
import { useSession } from '../../features/auth/use-session';
import { useViewer } from '../../features/auth/use-viewer';
import { Sidebar } from './sidebar';
import { AppHeader } from './app-header';

export function AppShell({ viewer: providedViewer, children, header, footer }: { viewer?: Viewer; children: ReactNode; header?: ReactNode; footer?: ReactNode }) {
  const session = useSession();
  const me = useViewer();
  const login = session?.login.includes('@') ? '' : session?.login ?? '';
  const viewer: Viewer = providedViewer ?? (session ? { kind: 'member', username: me.data?.username ?? login, displayName: me.data?.display_name || me.data?.username || login || 'Профиль' } : guestViewer);
  return <div className="app-shell"><a className="skip-link" href="#main-content">К содержимому</a><Sidebar viewer={viewer} /><div className="app-workspace">{header ?? <AppHeader viewer={viewer} />}<main id="main-content" className="app-main">{children}</main>{footer}</div></div>;
}
