import { expect, test, type Page } from '@playwright/test';

const user = '11111111-1111-4111-8111-111111111111';
type Post = { id: string; author_id: string; author: string; title: string; type: string; visibility: string; status: string; markdown: string; url: string; tags: { id: string; title: string; name: string }[]; photos: { id: string; url: string }[]; created_at: string; save_copy?: boolean; create_thumbnail?: boolean; capture_state?: string; copy_url?: string; thumbnail_url?: string };
const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a0GQAAAAASUVORK5CYII=', 'base64');
async function setup(page: Page) {
  await page.route('**/api/v1/notifications/unread', route => route.fulfill({ json: { count: 0 } }));
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

test('link options, pending capture, thumbnail and offline sandbox on desktop and mobile', async ({page})=>{
  const posts=await setup(page);
  await page.getByRole('button',{name:'Создать пост'}).click();const form=page.getByRole('dialog');
  await form.getByRole('button',{name:'Ссылка',exact:true}).click();
  await expect(form.getByLabel('Сохранить копию страницы')).not.toBeChecked();
  await expect(form.getByLabel('Создать эскиз страницы')).not.toBeChecked();
  await expect(form.getByLabel('Индексация для RAG')).not.toBeChecked();
  await form.getByLabel('Наименование').fill('Снимок сайта');
  const url=`https://example.com/${'a'.repeat(310)}`;
  await form.getByLabel('Адрес ссылки').fill(url);
  await form.getByRole('combobox',{name:'Теги'}).fill('Сайт');await form.getByRole('button',{name:/Создать тег/}).click();
  await form.getByLabel('Сохранить копию страницы').check();await form.getByLabel('Создать эскиз страницы').check();
  await page.screenshot({path:'test-results/link-options-desktop.png',fullPage:true});
  await form.getByRole('button',{name:'Опубликовать',exact:true}).click();await expect(form).toHaveCount(0);
  expect(posts[0]?.save_copy).toBe(true);expect(posts[0]?.create_thumbnail).toBe(true);
  await expect(page.getByRole('button',{name:'Показать копию'})).toHaveCount(0);
  expect(await page.locator('.post-link').textContent()).toHaveLength(300);await expect(page.locator('.post-link')).toHaveAttribute('href',url);
  Object.assign(posts[0]!,{capture_state:'pending'});await page.reload();
  await expect(page.getByRole('button',{name:'Показать копию'})).toHaveCount(0);
  const cssID='a'.repeat(64);const imageID='b'.repeat(64);let externalRequests=0;
  page.on('request',r=>{if(r.url().startsWith('https://external.invalid'))externalRequests++;});
  await page.route('**/api/v1/posts/*/copy',route=>route.fulfill({json:{html:`<!doctype html><html><head><meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline' data:; img-src data:; script-src 'none'; connect-src 'none'; base-uri 'none'; form-action 'none'"><link rel="stylesheet" href="sbasset:${cssID}"></head><body><h1>Отрендеренный DOM</h1><img src="sbasset:${imageID}" alt="Локальная картинка"><img src="https://external.invalid/leak"><script>parent.document.body.dataset.leaked='yes'</script></body></html>`,assets:{[cssID]:{mime:'text/css',data:Buffer.from('h1{color:rgb(1, 2, 3)}').toString('base64'),path:'hash/css'},[imageID]:{mime:'image/png',data:png.toString('base64'),path:'hash/image'}},thumbnail:''}}));
  await page.route('**/api/v1/posts/*/thumbnail',route=>route.fulfill({contentType:'image/png',body:png}));
  Object.assign(posts[0]!,{capture_state:'ready',copy_url:`/api/v1/posts/${posts[0]!.id}/copy`,thumbnail_url:`/api/v1/posts/${posts[0]!.id}/thumbnail`});
  await expect(page.getByRole('button',{name:'Показать копию'})).toBeVisible({timeout:10_000});
  await expect(page.getByRole('img',{name:'Эскиз страницы «Снимок сайта»'})).toBeVisible();
  await page.getByRole('button',{name:'Показать копию'}).click();
  const frame=page.frameLocator('.archive-dialog iframe');await expect(frame.getByRole('heading')).toHaveText('Отрендеренный DOM');
  await expect(frame.getByRole('heading')).toHaveCSS('color','rgb(1, 2, 3)');
  await expect(frame.getByAltText('Локальная картинка')).toBeVisible();
  expect(externalRequests).toBe(0);expect(await page.locator('body').getAttribute('data-leaked')).toBeNull();
  await expect(page.locator('.archive-dialog iframe')).toHaveAttribute('sandbox','');
  await page.setViewportSize({width:390,height:844});await page.screenshot({path:'test-results/link-copy-mobile.png',fullPage:true});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await page.getByRole('button',{name:'Закрыть копию'}).click();
  await page.getByRole('button',{name:'Редактировать Снимок сайта'}).click();
  await expect(form.getByLabel('Сохранить копию страницы')).toBeChecked();await expect(form.getByLabel('Создать эскиз страницы')).toBeChecked();
});

test('independent RAG checkbox persists on edit and can be disabled on mobile', async ({ page }) => {
  const posts = await setup(page);
  await page.getByRole('button', { name: 'Создать пост' }).click(); const form = page.getByRole('dialog');
  await form.getByRole('button', { name: 'Ссылка', exact: true }).click();
  await expect(form.getByLabel('Индексация для RAG')).not.toBeChecked();
  await form.getByLabel('Наименование').fill('Документ RAG');
  await form.getByLabel('Адрес ссылки').fill('https://example.com/article');
  await form.getByRole('combobox', { name: 'Теги' }).fill('Поиск'); await form.getByRole('button', { name: /Создать тег/ }).click();
  await form.getByLabel('Индексация для RAG').check();
  const saved = page.waitForRequest(r => r.method() === 'POST' && new URL(r.url()).pathname === '/api/v1/posts');
  await form.getByRole('button', { name: 'Опубликовать', exact: true }).click();
  expect((await saved).postDataJSON()).toMatchObject({ index_rag: true, save_copy: false, create_thumbnail: false });
  await page.getByRole('button', { name: 'Редактировать Документ RAG' }).click();
  await expect(form.getByLabel('Индексация для RAG')).toBeChecked();
  await page.setViewportSize({ width: 390, height: 844 });
  await form.getByLabel('Индексация для RAG').uncheck();
  await page.screenshot({ path: 'test-results/rag-form-mobile.png', fullPage: true });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  const edited = page.waitForRequest(r => r.method() === 'PUT');
  await form.getByRole('button', { name: 'Сохранить изменения' }).click();
  expect((await edited).postDataJSON().index_rag).toBe(false);
  expect(posts).toHaveLength(1);
});
