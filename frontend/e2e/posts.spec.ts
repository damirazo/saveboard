import { expect, test, type Page } from '@playwright/test';

const user = '11111111-1111-4111-8111-111111111111';
type Post = { id: string; author_id: string; author: string; title: string; type: string; visibility: string; status: string; markdown: string; url: string; tags: { id: string; title: string; name: string }[]; photos: { id: string; url: string }[]; created_at: string };
const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a0GQAAAAASUVORK5CYII=', 'base64');
async function setup(page: Page) {
  const posts: Post[] = []; let uploads = 0;
  await page.route('**/api/v1/auth/**', route => route.fulfill({ json: route.request().url().endsWith('/me') ? { user_id: user, session_id: 'session' } : { access_token: 'test', refresh_token: 'test', expires_in: 3600, refresh_expires_at: new Date(Date.now() + 86400000).toISOString(), session_id: 'session', token_type: 'Bearer' } }));
  await page.route('**/api/v1/post-tags?*', route => { const q = new URL(route.request().url()).searchParams.get('q') || ''; return route.fulfill({ json: 'Технологии'.toLowerCase().startsWith(q.toLowerCase()) ? [{ id: 'tag1', name: 'tekhnologii', title: 'Технологии' }] : [] }); });
  await page.route('**/api/v1/photos**', route => {
    if (route.request().method() === 'POST') { uploads++; return route.fulfill({ json: { id: `photo-${uploads}`, url: `/api/v1/photos/photo-${uploads}` } }); }
    return route.fulfill({ contentType: 'image/png', body: png });
  });
  await page.route('**/api/v1/posts**', async route => {
    const req = route.request(); const url = new URL(req.url());
    if (req.method() === 'DELETE') { const p = posts.find(p => url.pathname.endsWith(p.id)); if (p) p.status = 'draft'; await route.fulfill({ status: 204 }); return; }
    if (req.method() === 'POST' || req.method() === 'PUT') {
      const data = req.postDataJSON(); const id = req.method() === 'PUT' ? url.pathname.split('/').pop()! : `post-${posts.length + 1}`;
      const p: Post = { ...data, id, author_id: user, author: 'alex', status: 'published', created_at: new Date().toISOString(), tags: data.tags.map((t: { title: string; id: string }) => ({ ...t, name: t.title })), photos: data.photo_ids.map((id: string) => ({ id, url: `/api/v1/photos/${id}` })) };
      const i = posts.findIndex(p => p.id === id); if (i < 0) posts.unshift(p); else posts[i] = p;
      await route.fulfill({ json: p }); return;
    }
    const items = posts.filter(p => p.status === 'published' && (url.searchParams.get('scope') === 'mine' || p.visibility === 'public'));
    await route.fulfill({ json: { items, total: items.length } });
  });
  await page.goto('/login'); await page.getByLabel('Username или email', { exact: true }).fill('alex'); await page.getByLabel('Пароль', { exact: true }).fill('test-password'); await page.getByRole('button', { name: 'Войти', exact: true }).click();
  await expect(page).toHaveURL(/\/feed/);
  return posts;
}
test('create, edit, render markdown and remove into drafts', async ({ page }) => {
  const posts = await setup(page);
  await page.getByRole('button', { name: 'Создать пост' }).click();
  const form = page.getByRole('dialog');
  await form.getByLabel('Наименование').fill('Первая заметка');
  await form.getByRole('combobox', { name: 'Теги' }).fill('Тех');
  await expect(form.getByRole('listbox').getByRole('option').first()).toContainText('Создать тег');
  await form.getByRole('button', { name: '# Технологии', exact: true }).click();
  await form.getByRole('combobox', { name: 'Теги' }).fill('Технологии');
  await expect(form.getByRole('button', { name: /Создать тег/ })).toHaveCount(0);
  await form.getByRole('combobox', { name: 'Теги' }).fill('Новый тег');
  await form.getByRole('button', { name: 'Создать тег «Новый тег»', exact: false }).click();
  await form.getByRole('button', { name: 'Удалить тег Новый тег' }).click();
  await form.getByRole('button', { name: 'Markdown', exact: true }).click();
  await form.getByRole('textbox', { name: 'Markdown', exact: true }).fill('# Заголовок\n\n**Важная мысль**');
  await form.getByRole('button', { name: 'Визуальный', exact: true }).click();
  await expect(form.locator('[contenteditable=true]')).toContainText('Важная мысль');
  await page.screenshot({ path: 'test-results/post-editor-desktop.png', fullPage: true });
  await form.getByRole('button', { name: 'Опубликовать', exact: true }).click();
  await expect(form).toHaveCount(0); await expect(page.locator('article strong')).toHaveText('Важная мысль');
  expect(posts[0]?.tags).toHaveLength(1);
  await page.getByRole('button', { name: 'Редактировать Первая заметка' }).click();
  await form.getByLabel('Наименование').fill('Обновлённая заметка');
  await form.getByRole('button', { name: 'Сохранить изменения' }).click();
  await expect(page.getByRole('heading', { name: 'Обновлённая заметка' })).toBeVisible();
  page.on('dialog', d => d.accept()); await page.getByRole('button', { name: 'Удалить Обновлённая заметка' }).click();
  await expect(page.locator('article')).toHaveCount(0); expect(posts[0]?.status).toBe('draft');
});

test('32 photos, responsive collapse and fullscreen navigation', async ({ page }) => {
  await setup(page); await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole('button', { name: 'Создать пост' }).click(); const form = page.getByRole('dialog');
  await form.getByLabel('Наименование').fill('Альбом'); await form.getByRole('button', { name: 'Фото', exact: true }).click();
  await form.getByRole('combobox', { name: 'Теги' }).fill('Фото'); await form.getByRole('button', { name: /Создать тег/ }).click();
  const file = { name: 'image.png', mimeType: 'image/png', buffer: png };
  await form.locator('input[type=file]').setInputFiles(Array.from({ length: 33 }, (_, i) => ({ ...file, name: `${i}.png` })));
  await expect(form.getByRole('alert')).toContainText('До 32');
  await form.locator('input[type=file]').setInputFiles(Array.from({ length: 32 }, (_, i) => ({ ...file, name: `${i}.png` })));
  await expect(form.locator('.upload-preview')).toHaveCount(32); await expect(form.getByRole('button', { name: 'Опубликовать', exact: true })).toBeEnabled();
  await page.screenshot({ path: 'test-results/post-photos-mobile.png', fullPage: true });
  await form.getByRole('button', { name: 'Опубликовать', exact: true }).click();
  await expect(page.locator('article .photo-grid button')).toHaveCount(6);
  await page.getByRole('button', { name: 'Показать все фото (32)' }).click(); await expect(page.locator('article .photo-grid button')).toHaveCount(32);
  await page.getByRole('button', { name: 'Открыть фото 1', exact: true }).click();
  await expect(page.getByRole('button', { name: 'Закрыть', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Следующее фото', exact: true }).click(); await page.keyboard.press('Escape');
  await page.getByRole('button', { name: 'Свернуть фото' }).click(); await expect(page.locator('article .photo-grid button')).toHaveCount(6);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});
