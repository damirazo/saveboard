export type Viewer = { kind: 'guest' } | { kind: 'member'; username: string; displayName: string };
export const guestViewer: Viewer = { kind: 'guest' };
export const demoViewer: Viewer = { kind: 'member', username: 'damir', displayName: 'Дамир' };
