import type { Page } from '@playwright/test';
import { demoPosts } from '../src/features/library/demo-posts';
export async function mockPosts(page: Page) {
  await page.route('**/api/v1/notifications/unread', route => route.fulfill({ json: { count: 0 } }));
  await page.route('**/api/v1/posts?*', async route => {
    const url = new URL(route.request().url()); const q = url.searchParams.get('q')?.toLowerCase() || '';
    const kind = url.searchParams.get('type'); const scope = url.searchParams.get('scope');
    let items = demoPosts.map(p => ({ id: String(p.id), author_id: 'user', author: p.author, title: p.title, type: p.kind === 'text' ? 'note' : p.kind === 'web' ? 'link' : 'photo', visibility: p.public ? 'public' : 'private', status: 'published', markdown: p.text, url: 'https://example.com', photos: [], tags: p.tags.map((title, i) => ({ id: String(i), name: title, title })), created_at: '2026-10-09T10:00:00Z' }))
      .filter(p => (scope === 'mine' || p.visibility === 'public') && (!kind || p.type === kind) && `${p.title} ${p.markdown}`.toLowerCase().includes(q));
    if (url.searchParams.get('sort') === 'old') items = items.reverse();
    const total = items.length; const size = Number(url.searchParams.get('size') || 10); const offset = (Number(url.searchParams.get('page') || 1) - 1) * size;
    await route.fulfill({ json: { items: items.slice(offset, offset + size), total } });
  });
}
