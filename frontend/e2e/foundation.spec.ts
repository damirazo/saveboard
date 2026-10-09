import { expect, test } from '@playwright/test';

test('foundation shell consumes API responses and works at mobile width', async ({ page }) => {
  // Browser integration of the frontend only; live Compose is checked separately.
  await page.route('**/api/v1/system', route => route.fulfill({ json: { name: 'Saveboard', stage: 'foundation', authentication: false, capture: false, semanticSearch: false } }));
  await page.route('**/api/health/ready', route => route.fulfill({ json: { status: 'ready' } }));
  await page.goto('/system');
  await expect(page.getByRole('status')).toHaveText('API и PostgreSQL доступны');
  await expect(page.getByRole('link', { name: 'Документация API' })).toHaveAttribute('href', '/api/docs');
  await page.screenshot({ path: 'test-results/foundation-desktop.png', fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.screenshot({ path: 'test-results/foundation-mobile.png', fullPage: true });
});
