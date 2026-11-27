import { expect, test } from '@playwright/test';

test('notifications: bell, pagination, modal, read receipt and post link', async ({ page }) => {
  const items = Array.from({ length: 11 }, (_, i) => ({ id: `notification-${i}`, title: `Сообщение ${i}`, text: 'Готово <script>alert(1)</script>', importance: i === 0 ? 'critical' : 'normal', read: false, post_id: 'post-1', created_at: new Date().toISOString() }));
  let readCalls = 0; let failRead = true;
  await page.route('**/api/v1/auth/**', route => route.fulfill({ json: route.request().url().endsWith('/me') ? { user_id: 'user', session_id: 'session' } : { access_token: 'test', refresh_token: 'test', expires_in: 3600, refresh_expires_at: new Date(Date.now() + 86400000).toISOString(), session_id: 'session', token_type: 'Bearer' } }));
  await page.route('**/api/v1/posts**', route => route.fulfill({ json: route.request().url().endsWith('/post-1') ? { id: 'post-1', author: 'alex', author_name: 'Алексей', created_at: '2026-11-27T12:00:00Z', visibility: 'public', title: 'Готовый пост', type: 'link', url: 'https://example.com', tags: [], photos: [] } : { items: [], total: 0 } }));
  await page.route('**/api/v1/notifications**', async route => {
    const request = route.request(); const url = new URL(request.url());
    if (url.pathname.endsWith('/unread')) { await route.fulfill({ json: { count: items.filter(n => !n.read).length } }); return; }
    const id = url.pathname.split('/')[4];
    if (id) {
      const item = items.find(n => n.id === id)!;
      if (request.method() === 'POST') { readCalls++; if (failRead) { await route.fulfill({ status: 503, json: { detail: 'Failed' } }); return; } item.read = true; await route.fulfill({ status: 204 }); return; }
      await route.fulfill({ json: item }); return;
    }
    const pageNumber = Number(url.searchParams.get('page') ?? 1);
    await route.fulfill({ json: { items: items.slice((pageNumber - 1) * 10, pageNumber * 10), total: items.length } });
  });
  await page.goto('/login');
  await expect(page.getByRole('link', { name: /Уведомления:/ })).toHaveCount(0);
  await page.getByLabel('Username или email', { exact: true }).fill('alex');
  await page.getByLabel('Пароль', { exact: true }).fill('test-password');
  await page.getByRole('button', { name: 'Войти', exact: true }).click();
  await page.getByRole('link', { name: 'Уведомления: 11 непрочитанных' }).click();
  await expect(page).toHaveURL(/\/notifications/);
  await expect(page.locator('.notification-list li')).toHaveCount(10);
  await page.getByRole('button', { name: 'Следующая страница' }).click();
  await expect(page.getByRole('button', { name: /Сообщение 10/ })).toBeVisible();
  await page.getByRole('button', { name: 'Предыдущая страница' }).click();
  const first = page.getByRole('button', { name: /Сообщение 0/ });
  await expect(first).toHaveClass('is-unread'); await first.click();
  const modal = page.getByRole('dialog');
  await expect(modal.getByRole('heading', { name: 'Сообщение 0' })).toBeVisible();
  await expect(modal.getByText('Критическое', { exact: true })).toBeVisible();
  await expect(modal.getByText('Готово <script>alert(1)</script>', { exact: true })).toBeVisible();
  await expect(modal.getByRole('alert')).toContainText('Не удалось отметить');
  failRead = false; await modal.getByRole('button', { name: 'Повторить' }).click();
  await expect(page.getByRole('link', { name: 'Уведомления: 10 непрочитанных' })).toBeVisible();
  await modal.getByRole('button', { name: 'Закрыть уведомление' }).click();
  await expect(first).not.toHaveClass('is-unread'); await first.click();
  await modal.getByRole('link', { name: 'Открыть пост' }).click();
  await expect(page).toHaveURL(/\/posts\/post-1/);
  await expect(page.getByRole('heading', { name: 'Готовый пост' })).toBeVisible();
  expect(readCalls).toBe(2);
});

test('guest cannot open notifications', async ({ page }) => {
  await page.goto('/notifications');
  await expect(page).toHaveURL(/\/login/);
  await expect(page.getByRole('link', { name: /Уведомления:/ })).toHaveCount(0);
});
