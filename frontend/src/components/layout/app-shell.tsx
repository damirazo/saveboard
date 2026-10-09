import type { ReactNode } from 'react';
import { guestViewer, type Viewer } from './viewer';
import { Sidebar } from './sidebar';
import { AppHeader } from './app-header';

export function AppShell({ viewer = guestViewer, children, header, footer }: { viewer?: Viewer; children: ReactNode; header?: ReactNode; footer?: ReactNode }) {
  return <div className="app-shell"><a className="skip-link" href="#main-content">К содержимому</a><Sidebar viewer={viewer} /><div className="app-workspace">{header ?? <AppHeader viewer={viewer} />}<main id="main-content" className="app-main">{children}</main>{footer}</div></div>;
}
