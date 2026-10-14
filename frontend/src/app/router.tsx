import { createRootRoute, createRoute, createRouter, Outlet, redirect } from '@tanstack/react-router';
import { SystemPage } from '../features/system/system-page';
import { LibraryPage, type LibrarySection } from '../features/library/library-page';
import { AuthPage } from '../features/auth/auth-page';
import { sessionClient } from '@saveboard/api-client';
import { MemberPreview } from '../features/library/member-preview';
import { NotificationsPage } from '../features/notifications/notifications';
import { PostPage } from '../features/library/post-page';
const rootRoute = createRootRoute({ component: Outlet, beforeLoad: () => sessionClient.restore() });
const indexRoute = createRoute({ getParentRoute: () => rootRoute, path: '/', component: LibraryPage });
const memberRoute = createRoute({ getParentRoute: () => rootRoute, path: '/feed', beforeLoad: () => { if (!sessionClient.getSnapshot()) throw redirect({ to: '/' }); }, validateSearch: (search: Record<string, unknown>): { section: LibrarySection } => ({ section: search.section === 'favorites' || search.section === 'archives' ? search.section : 'all' }), component: MemberPreview });
const previewRoute = createRoute({ getParentRoute: () => rootRoute, path: '/preview/member', beforeLoad: () => { throw redirect({ to: '/feed', search: { section: 'all' } }); } });
const loginRoute = createRoute({ getParentRoute: () => rootRoute, path: '/login', beforeLoad: () => { if (sessionClient.getSnapshot()) throw redirect({ to: "/feed", search: { section: "all" } }); }, component: () => <AuthPage key="login" mode="login" onSubmit={async values => { await sessionClient.login(values.login, values.password); await router.navigate({ to: "/feed", search: { section: "all" } }); }} /> });
const registerRoute = createRoute({ getParentRoute: () => rootRoute, path: '/register', beforeLoad: () => { if (sessionClient.getSnapshot()) throw redirect({ to: '/feed', search: { section: 'all' } }); }, component: () => <AuthPage key="register" mode="register" onSubmit={async values => { await sessionClient.register(values.username, values.email, values.password); await router.navigate({ to: '/feed', search: { section: 'all' } }); }} /> });
const systemRoute = createRoute({ getParentRoute: () => rootRoute, path: '/system', component: SystemPage });
const notificationsRoute = createRoute({ getParentRoute: () => rootRoute, path: '/notifications', beforeLoad: () => { if (!sessionClient.getSnapshot()) throw redirect({ to: '/login' }); }, component: NotificationsPage });
const postRoute = createRoute({ getParentRoute: () => rootRoute, path: '/posts/$postId', component: PostPage });
export const router = createRouter({ routeTree: rootRoute.addChildren([indexRoute, memberRoute, previewRoute, loginRoute, registerRoute, systemRoute, notificationsRoute, postRoute]), defaultPreload: 'intent' });
declare module '@tanstack/react-router' { interface Register { router: typeof router } }
