import { createRootRoute, createRoute, createRouter, Outlet, redirect } from '@tanstack/react-router';
import { UiKitPage } from '../components/ui-kit-page';
import { SystemPage } from '../features/system/system-page';
import { LibraryPage, type LibrarySection } from '../features/library/library-page';
import { AuthPage } from '../features/auth/auth-page';
import { sessionClient } from '@saveboard/api-client';
import { MemberPreview } from '../features/library/member-preview';
import { NotificationsPage } from '../features/notifications/notifications';
import { PostPage } from '../features/library/post-page';
import { ProfilePage, SearchPage } from '../features/library/feed-routes';
import { EditorPage } from '../features/library/editor-page';
import { SettingsPage } from '../features/profile/settings-page';
const rootRoute = createRootRoute({ component: Outlet, beforeLoad: () => sessionClient.restore() });
const indexRoute = createRoute({ getParentRoute: () => rootRoute, path: '/', component: LibraryPage });
const memberRoute = createRoute({ getParentRoute: () => rootRoute, path: '/feed', beforeLoad: () => { if (!sessionClient.getSnapshot()) throw redirect({ to: '/' }); }, validateSearch: (search: Record<string, unknown>): { section: LibrarySection } => ({ section: search.section === 'favorites' || search.section === 'archives' ? search.section : 'all' }), component: MemberPreview });
const previewRoute = createRoute({ getParentRoute: () => rootRoute, path: '/preview/member', beforeLoad: () => { throw redirect({ to: '/feed', search: { section: 'all' } }); } });
const loginRoute = createRoute({ getParentRoute: () => rootRoute, path: '/login', beforeLoad: () => { if (sessionClient.getSnapshot()) throw redirect({ to: "/feed", search: { section: "all" } }); }, component: () => <AuthPage key="login" mode="login" onSubmit={async values => { await sessionClient.login(values.login, values.password); await router.navigate({ to: "/feed", search: { section: "all" } }); }} /> });
const registerRoute = createRoute({ getParentRoute: () => rootRoute, path: '/register', beforeLoad: () => { if (sessionClient.getSnapshot()) throw redirect({ to: '/feed', search: { section: 'all' } }); }, component: () => <AuthPage key="register" mode="register" onSubmit={async values => { await sessionClient.register(values.username, values.email, values.password); await router.navigate({ to: '/feed', search: { section: 'all' } }); }} /> });
const searchRoute = createRoute({ getParentRoute: () => rootRoute, path: '/search',
  validateSearch: (s: Record<string, unknown>): { tag?: string; label?: string; scope?: 'mine' } => ({
    scope: s.scope === 'mine' ? 'mine' : undefined, tag: typeof s.tag === 'string' ? s.tag : undefined, label: typeof s.label === 'string' ? s.label : undefined,
  }), component: SearchPage });

const profileRoute = createRoute({ getParentRoute: () => rootRoute, path: '/profiles/$username', component: ProfilePage });

const requireSession = () => { if (!sessionClient.getSnapshot()) throw redirect({ to: '/login' }); };
const createPostRoute = createRoute({ getParentRoute: () => rootRoute, path: '/posts/new', beforeLoad: requireSession, component: EditorPage });
const editPostRoute = createRoute({ getParentRoute: () => rootRoute, path: '/posts/$postId/edit', beforeLoad: requireSession, component: EditorPage });
const settingsRoute = createRoute({ getParentRoute: () => rootRoute, path: '/settings', beforeLoad: () => { if (!sessionClient.getSnapshot()) throw redirect({ to: '/login' }); }, component: SettingsPage });
const kitRoute = createRoute({ getParentRoute: () => rootRoute, path: '/ui-kit', component: UiKitPage });
const systemRoute = createRoute({ getParentRoute: () => rootRoute, path: '/system', component: SystemPage });
const notificationsRoute = createRoute({ getParentRoute: () => rootRoute, path: '/notifications', beforeLoad: () => { if (!sessionClient.getSnapshot()) throw redirect({ to: '/login' }); }, component: NotificationsPage });
const postRoute = createRoute({ getParentRoute: () => rootRoute, path: '/posts/$postId', component: PostPage });
export const router = createRouter({ routeTree: rootRoute.addChildren([indexRoute, memberRoute, previewRoute, loginRoute, registerRoute, searchRoute, profileRoute, createPostRoute, editPostRoute, settingsRoute, kitRoute, systemRoute, notificationsRoute, postRoute]), defaultPreload: 'intent' });
declare module '@tanstack/react-router' { interface Register { router: typeof router } }
