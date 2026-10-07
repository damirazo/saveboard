import { createRootRoute, createRoute, createRouter, Outlet } from '@tanstack/react-router';
import { SystemPage } from '../features/system/system-page';
const rootRoute = createRootRoute({ component: Outlet });
const indexRoute = createRoute({ getParentRoute: () => rootRoute, path: '/', component: SystemPage });
export const router = createRouter({ routeTree: rootRoute.addChildren([indexRoute]), defaultPreload: 'intent' });
declare module '@tanstack/react-router' { interface Register { router: typeof router } }
