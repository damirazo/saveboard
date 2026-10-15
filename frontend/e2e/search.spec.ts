import { expect, test } from '@playwright/test';

for (const width of [1280, 390]) test(`search modes and existing tags at ${width}px`, async ({ page }) => {
  await page.setViewportSize({ width, height: 844 });
  const tags = [{ id: '11111111-1111-4111-8111-111111111111', title: 'Alpha', name: 'alpha' }, { id: '22222222-2222-4222-8222-222222222222', title: 'Beta', name: 'beta' }];
  await page.route('**/api/v1/post-tags?*', route => {
    const q = new URL(route.request().url()).searchParams.get('q')!.toLowerCase();
    return route.fulfill({ json: tags.filter(t => t.title.toLowerCase().startsWith(q)) });
  });
  await page.route('**/api/v1/posts?*', route => route.fulfill({ json: { items: [], total: 0 } }));
  await page.goto('/');
  const text = page.getByRole('searchbox', { name: 'Поиск публикаций' });
  await text.fill('needle');
  const toggle = page.getByRole('switch', { name: 'Поиск по тегам' });
  await expect(toggle).toHaveAttribute('aria-checked', 'false');
  await toggle.click();
  await expect(text).toHaveCount(0);
  const included = page.getByRole('combobox', { name: 'Имеющиеся теги' });
  const excluded = page.getByRole('combobox', { name: 'Исключенные теги' });
  await included.fill('unknown');
  await expect(page.getByText('Нет доступных тегов')).toBeVisible();
  await expect(page.getByRole('button', { name: /Создать тег/ })).toHaveCount(0);
  await included.press('ArrowDown'); await included.press('Enter');
  await expect(page.getByRole('button', { name: /Удалить тег/ })).toHaveCount(0);
  await included.fill('Al');
  await expect(page.getByRole('listbox').getByRole('option')).toHaveCount(1);
  const first = page.waitForRequest(r => new URL(r.url()).searchParams.get('include_tags') === tags[0]!.id);
  await included.press('Enter'); await first;
  await included.fill('Be');
  await page.getByRole('button', { name: '# Beta', exact: true }).click();
  await excluded.fill('Be');
  const both = page.waitForRequest(r => {
    const q = new URL(r.url()).searchParams;
    return q.get('include_tags') === tags.map(t => t.id).join(',') && q.get('exclude_tags') === tags[1]!.id && !q.get('q') && q.get('page') === '1';
  });
  await page.getByRole('button', { name: '# Beta', exact: true }).click(); await both;
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: `test-results/search-${width}.png`, fullPage: true });
  const restored = page.waitForRequest(r => {
    const q = new URL(r.url()).searchParams;
    return q.get('q') === 'needle' && !q.has('include_tags') && !q.has('exclude_tags');
  });
  await toggle.click(); await restored;
  await expect(text).toHaveValue('needle');
});
