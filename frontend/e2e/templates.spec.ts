import { expect, test, type Page } from '@playwright/test';
import { mockPosts } from './post-fixtures';

test.beforeEach(async ({ page }) => {
  await mockPosts(page);
  await page.route('**/api/v1/auth/**', async route => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith('/login') && route.request().postDataJSON().password !== 'test-password') {
      await route.fulfill({ status: 401, json: {} }); return;
    }
    await route.fulfill({ json: path.endsWith('/me') ? { user_id: 'user', session_id: 'session', username: 'alex', display_name: 'Алексей' } : path.endsWith('/logout') ? { revoked: 1 } : {
      access_token: 'test-access', refresh_token: 'test-refresh', expires_in: 3600,
      refresh_expires_at: new Date(Date.now() + 86400000).toISOString(), session_id: 'session', token_type: 'Bearer',
    } });
  });
});
async function login(page: Page) {
  await page.goto('/login');
  await page.getByLabel('Username или email', { exact: true }).fill('alex');
  await page.getByLabel('Пароль', { exact: true }).fill('test-password');
  await page.getByRole('button', { name: 'Войти', exact: true }).click();
  await expect(page).toHaveURL(/\/feed/);
}

test('guest and member use shared navigation without exposing private sample posts to guests', async ({ page }) => {
  await page.goto('/');
  await expect(page.getByRole('heading', { name: 'Главная страница' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Сохранять не только ссылку, но и контекст' })).toHaveCount(0);
  await expect(page.getByRole('navigation', { name: 'Основное меню' }).getByText('Избранное')).toHaveCount(0);
  await page.screenshot({ path: 'test-results/guest-desktop.png', fullPage: true });
  await login(page);
  await expect(page.locator('article')).toHaveCount(3);
  await page.getByRole('link', { name: 'Поиск', exact: true }).click();
  await expect(page.getByRole('searchbox')).toBeVisible();
  await page.getByRole('link', { name: 'Библиотека', exact: true }).click();
  await page.screenshot({ path: 'test-results/member-desktop.png', fullPage: true });
});

test('invalid credentials stay on login; registration validates and starts a session', async ({ page }) => {
  const writes: string[] = [];
  page.on('request', request => { if (request.method() === 'POST') writes.push(request.url()); });
  await page.goto('/login');
  await page.getByLabel('Username или email', { exact: true }).fill('alex@example.com');
  await page.getByLabel('Пароль', { exact: true }).fill('wrong-password');
  await page.getByRole('button', { name: 'Показать пароль', exact: true }).click();
  await expect(page.getByLabel('Пароль', { exact: true })).toHaveAttribute('type', 'text');
  await page.getByRole('button', { name: 'Скрыть пароль', exact: true }).click();
  await page.getByRole('button', { name: 'Войти', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Не удалось войти');
  await expect(page.getByRole('link', { name: 'На главную' })).toHaveAttribute('href', '/');
  await page.screenshot({ path: 'test-results/login-desktop.png', fullPage: true });
  await page.locator('.auth-switch').getByRole('link').click();
  await expect(page.getByLabel('Пароль', { exact: true })).toHaveValue('');
  await page.getByLabel('Username', { exact: true }).fill('bad@name');
  await page.getByLabel('Email', { exact: true }).fill('alex@example.com');
  await page.getByLabel('Пароль', { exact: true }).fill('test-password');
  await page.getByLabel('Повторите пароль', { exact: true }).fill('different-password');
  await page.getByRole('button', { name: 'Зарегистрироваться', exact: true }).click();
  expect(await page.getByLabel('Username', { exact: true }).evaluate((el: HTMLInputElement) => el.checkValidity())).toBe(false);
  await page.getByLabel('Username', { exact: true }).fill('alex');
  await page.getByRole('button', { name: 'Зарегистрироваться', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('Пароли не совпадают');
  await expect(page.getByRole('link', { name: 'На главную' })).toHaveAttribute('href', '/');
  expect(writes).toHaveLength(1);
  await page.screenshot({ path: 'test-results/register-desktop.png', fullPage: true });
  await page.getByLabel('Повторите пароль', { exact: true }).fill('test-password');
  await page.getByRole('button', { name: 'Зарегистрироваться', exact: true }).click();
  await expect(page).toHaveURL(/\/feed/);
  expect(writes).toHaveLength(2);
  await expect(page.locator('.sidebar-profile')).toContainText('alex');
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Библиотека', exact: true })).toBeVisible();
  await page.goto('/register');
  await expect(page).toHaveURL(/\/feed/);
});

test('registration conflict can be corrected without losing entered data', async ({ page }) => {
  let attempts = 0;
  await page.route('**/api/v1/auth/register', async route => {
    attempts++;
    if (attempts === 1) { await route.fulfill({ status: 409, json: {} }); return; }
    await route.fulfill({ status: 201, json: { access_token: 'registered', refresh_token: 'refresh', expires_in: 3600, refresh_expires_at: new Date(Date.now() + 86400000).toISOString(), session_id: 'session', token_type: 'Bearer' } });
  });
  await page.goto('/register');
  await page.getByLabel('Username', { exact: true }).fill('existing');
  await page.getByLabel('Email', { exact: true }).fill('new@example.com');
  await page.getByLabel('Пароль', { exact: true }).fill('test-password');
  await page.getByLabel('Повторите пароль', { exact: true }).fill('test-password');
  await page.getByRole('button', { name: 'Зарегистрироваться', exact: true }).click();
  await expect(page.getByRole('alert')).toContainText('уже зарегистрирован');
  expect(await page.evaluate(() => sessionStorage.getItem('saveboard.session'))).toBeNull();
  await expect(page.getByLabel('Email', { exact: true })).toHaveValue('new@example.com');
  await page.getByLabel('Username', { exact: true }).fill('available');
  await page.getByRole('button', { name: 'Зарегистрироваться', exact: true }).click();
  await expect(page).toHaveURL(/\/feed/);
  expect(attempts).toBe(2);
});

test('all templates fit mobile screens and expose navigation on demand', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 800 });
  for (const path of ['/', '/preview/member?section=all', '/login', '/register']) {
    await page.goto(path);
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    if (path === '/login' || path === '/register') { await expect(page.getByRole('link', { name: 'На главную' })).toBeVisible(); continue; }
    await page.getByRole('button', { name: 'Открыть меню' }).click();
    await expect(page.getByRole('navigation', { name: 'Основное меню' })).toBeVisible();
    await page.getByRole('button', { name: 'Закрыть меню' }).click();
  }
  await page.screenshot({ path: 'test-results/register-mobile.png', fullPage: true });
});

test('pagination combines with search, filters and sorting', async ({ page }) => {
  await login(page);
  await page.getByLabel('Публикаций на странице').selectOption('2');
  await expect(page.locator('article')).toHaveCount(2);
  await page.getByRole('button', { name: 'Следующая страница' }).click();
  await expect(page.locator('article')).toHaveCount(1);
  await expect(page.locator('article')).toContainText('Материалы о цифровых библиотеках');
  await page.getByRole('button', { name: 'Предыдущая страница' }).click();
  await page.getByLabel('Порядок публикаций').selectOption('old');
  await expect(page.locator('article').first()).toContainText('Материалы о цифровых библиотеках');
  await page.getByRole('button', { name: 'Заметки', exact: true }).click();
  await expect(page.locator('article')).toHaveCount(1);
  await expect(page.getByRole('button', { name: 'Следующая страница' })).toBeDisabled();
  await page.setViewportSize({ width: 320, height: 800 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

 test('session survives reload, logout returns to public feed, and feed rejects guests', async ({ page }) => {
  await page.goto('/feed');
  await expect(page).toHaveURL('/');
  await login(page);
  await page.reload();
  await expect(page.getByRole('heading', { name: 'Библиотека', exact: true })).toBeVisible();
  await expect(page.locator('.sidebar-profile')).toContainText('alex');
  const switcher = page.getByRole('navigation', { name: 'Основное меню' });
  await switcher.getByRole('link', { name: 'Главная страница', exact: true }).click();
  await expect(page).toHaveURL('/');
  await expect(page.locator('article')).toHaveCount(2);
  await expect(page.locator('.sidebar-profile')).toContainText('alex');
  await expect(page.getByRole('link', { name: 'Создать свою библиотеку' })).toHaveCount(0);
  await page.reload();
  await expect(switcher.getByRole('link', { name: 'Главная страница', exact: true })).toHaveAttribute('aria-current', 'page');
  await page.setViewportSize({ width: 320, height: 800 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: 'test-results/feed-switcher-mobile.png', fullPage: true });
  await page.getByRole('button', { name: 'Открыть меню' }).click();
  await switcher.getByRole('link', { name: 'Библиотека', exact: true }).click();
  await expect(page).toHaveURL(/\/feed/);
  await expect(page.locator('article')).toHaveCount(3);
  await page.goBack();
  await expect(page).toHaveURL('/');
  await page.getByRole('button', { name: 'Выйти', exact: true }).click();
  await expect(page).toHaveURL('/');
  await expect(page.locator('article')).toHaveCount(2);
  expect(await page.evaluate(() => sessionStorage.getItem('saveboard.session'))).toBeNull();
  await expect(switcher.getByRole('link', { name: 'Библиотека' })).toHaveCount(0);
});

test('public profile keeps long titles and all tags readable on desktop and mobile', async ({ page }) => {
  const title = 'Как меняется восприятие городской архитектуры, когда дневной свет уступает место вечернему освещению';
  const tags = Array.from({ length: 20 }, (_, i) => ({ id: String(i), title: 'наблюдение-' + (i + 1), name: 'tag-' + i }));
  await page.route('**/api/v1/profiles/anna', route => route.fulfill({ json: { username: 'anna', display_name: 'Анна Мирова', public_posts: 1 } }));
  await page.route('**/api/v1/posts?*', route => {
    const query = new URL(route.request().url()).searchParams;
    expect(query.get('author')).toBe('anna'); expect(query.get('scope')).toBe('public');
    return route.fulfill({ json: { total: 1, items: [{ id: 'note-1', type: 'note', status: 'published', visibility: 'public', author: 'anna', author_name: 'Анна Мирова', title, markdown: 'Тёплый свет меняет восприятие пространства. Вечером холодный бетон кажется мягче.', tags, photos: [], created_at: '2026-11-27T11:30:00Z' }] } });
  });
  await page.goto('/profiles/anna');
  const post = page.locator('article');
  await expect(post.getByRole('heading')).toHaveText(title);
  const row = post.locator('.publication-tags');
  await expect(row.locator('> *')).toHaveCount(22);
  await expect(row.locator('> *').first()).toHaveText('@anna');
  await expect(row.locator('> *').nth(1)).toHaveText('#заметка');
  await expect(post.getByLabel('Комментарии недоступны')).toHaveText('—');
  await expect(post.locator('.identity .avatar')).toHaveText('АМ');
  await page.screenshot({ path: 'test-results/profile-desktop.png', fullPage: true });
  await page.setViewportSize({ width: 360, height: 800 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  const first = await row.locator('> *').first().boundingBox();
  const last = await row.locator('> *').last().boundingBox();
  expect(last!.y).toBeGreaterThan(first!.y);
  await page.screenshot({ path: 'test-results/profile-mobile.png', fullPage: true });
});

test('profile settings save the display name and refresh the sidebar', async ({ page }) => {
  await page.goto('/settings');
  await expect(page).toHaveURL('/login');
  let name = 'Алексей';
  await page.route('**/api/v1/auth/me', route => {
    if (route.request().method() === 'PATCH') {
      name = route.request().postDataJSON().display_name;
      return route.fulfill({ status: 204 });
    }
    return route.fulfill({ json: { user_id: 'user', session_id: 'session', username: 'alex', display_name: name } });
  });
  await login(page);
  await page.getByRole('link', { name: 'Настройки' }).click();
  await page.getByLabel('Отображаемое имя').fill('Алексей Миронов');
  await page.getByRole('button', { name: 'Сохранить изменения' }).click();
  await expect(page.getByRole('status')).toHaveText('Имя сохранено.');
  await expect(page.locator('.sidebar-profile')).toContainText('Алексей Миронов');
  await page.reload();
  await expect(page.getByLabel('Отображаемое имя')).toHaveValue('Алексей Миронов');
});
