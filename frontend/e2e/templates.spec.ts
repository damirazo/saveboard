import { expect, test, type Page } from '@playwright/test';

test.beforeEach(async ({ page }) => {
  await page.route('**/api/v1/auth/**', async route => {
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith('/login') && route.request().postDataJSON().password !== 'test-password') {
      await route.fulfill({ status: 401, json: {} }); return;
    }
    await route.fulfill({ json: path.endsWith('/me') ? { user_id: 'user', session_id: 'session' } : path.endsWith('/logout') ? { revoked: 1 } : {
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
  await expect(page.getByRole('heading', { name: 'Интересное рядом' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Сохранять не только ссылку, но и контекст' })).toHaveCount(0);
  await expect(page.getByRole('navigation', { name: 'Основное меню' }).getByText('Избранное')).toHaveCount(0);
  await page.screenshot({ path: 'test-results/guest-desktop.png', fullPage: true });
  await login(page);
  await expect(page.locator('article')).toHaveCount(3);
  await page.getByRole('link', { name: 'Избранное', exact: true }).click();
  await expect(page.locator('article')).toHaveCount(1);
  await page.getByRole('button', { name: 'Убрать из избранного' }).click();
  await expect(page.getByRole('status')).toContainText('Публикаций не найдено');
  await page.getByRole('link', { name: 'Все публикации', exact: true }).click();
  await page.screenshot({ path: 'test-results/member-desktop.png', fullPage: true });
});

test('invalid credentials stay on login; registration remains a placeholder', async ({ page }) => {
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
  await expect(page.getByRole('button', { name: /Продолжить через OAuth/ })).toBeDisabled();
  await page.screenshot({ path: 'test-results/login-desktop.png', fullPage: true });
  await page.locator('.auth-switch').getByRole('link').click();
  await expect(page.getByLabel('Пароль', { exact: true })).toHaveValue('');
  await page.getByLabel('Username', { exact: true }).fill('bad@name');
  await page.getByLabel('Email', { exact: true }).fill('alex@example.com');
  await page.getByLabel('Пароль', { exact: true }).fill('test-password');
  await page.getByRole('button', { name: 'Зарегистрироваться', exact: true }).click();
  expect(await page.getByLabel('Username', { exact: true }).evaluate((el: HTMLInputElement) => el.checkValidity())).toBe(false);
  await page.getByLabel('Username', { exact: true }).fill('alex');
  await page.getByRole('button', { name: 'Зарегистрироваться', exact: true }).click();
  await expect(page.getByRole('status')).toContainText('Аккаунт не создан');
  await expect(page.getByRole('button', { name: /Продолжить через OAuth/ })).toBeDisabled();
  expect(writes).toHaveLength(1);
  await page.screenshot({ path: 'test-results/register-desktop.png', fullPage: true });
});

test('all templates fit mobile screens and expose navigation on demand', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 800 });
  for (const path of ['/', '/preview/member?section=all', '/login', '/register']) {
    await page.goto(path);
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
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
  await page.getByRole('searchbox', { name: 'Поиск публикаций' }).fill('контекст');
  await expect(page.locator('article')).toHaveCount(1);
  await expect(page.getByRole('button', { name: 'Предыдущая страница' })).toBeDisabled();
  await page.getByRole('searchbox', { name: 'Поиск публикаций' }).fill('');
  await expect(page.getByRole('button', { name: 'Предыдущая страница' })).toBeDisabled();
  await expect(page.locator('article')).toHaveCount(2);
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
  await expect(page.getByRole('heading', { name: 'Все публикации' })).toBeVisible();
  await expect(page.locator('.sidebar-profile')).toContainText('alex');
  const switcher = page.getByRole('navigation', { name: 'Выбор ленты' });
  await switcher.getByRole('link', { name: 'Публичная лента', exact: true }).click();
  await expect(page).toHaveURL('/');
  await expect(page.locator('article')).toHaveCount(2);
  await expect(page.locator('.sidebar-profile')).toContainText('alex');
  await expect(page.getByRole('link', { name: 'Создать свою библиотеку' })).toHaveCount(0);
  await page.reload();
  await expect(switcher.getByRole('link', { name: 'Публичная лента', exact: true })).toHaveAttribute('aria-current', 'page');
  await page.setViewportSize({ width: 320, height: 800 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: 'test-results/feed-switcher-mobile.png', fullPage: true });
  await switcher.getByRole('link', { name: 'Моя лента', exact: true }).click();
  await expect(page).toHaveURL(/\/feed/);
  await expect(page.locator('article')).toHaveCount(3);
  await page.goBack();
  await expect(page).toHaveURL('/');
  await page.getByRole('button', { name: 'Выйти', exact: true }).click();
  await expect(page).toHaveURL('/');
  await expect(page.locator('article')).toHaveCount(2);
  expect(await page.evaluate(() => sessionStorage.getItem('saveboard.session'))).toBeNull();
  await expect(switcher).toHaveCount(0);
});
