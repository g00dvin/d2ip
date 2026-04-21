import { test, expect } from './fixtures.js';

const SECTIONS = [
  { nav: 'Dashboard', sectionId: 'section-dashboard' },
  { nav: 'Pipeline', sectionId: 'section-pipeline' },
  { nav: 'Config', sectionId: 'section-config' },
  { nav: 'Categories', sectionId: 'section-categories' },
  { nav: 'Cache', sectionId: 'section-cache' },
  { nav: 'Source', sectionId: 'section-source' },
  { nav: 'Routing', sectionId: 'section-routing' },
];

test('loads page with sidebar and dashboard visible', async ({ page }) => {
  await page.goto('/');
  await expect(page).toHaveTitle(/d2ip/);

  // Sidebar brand
  await expect(page.locator('.sidebar-brand')).toBeVisible();
  await expect(page.locator('.brand-text')).toHaveText('d2ip');

  // All 7 nav items present
  const navItems = page.locator('.nav-item');
  await expect(navItems).toHaveCount(7);

  // Dashboard visible by default, others hidden
  await expect(page.locator('#section-dashboard')).toBeVisible();
  for (const s of SECTIONS.slice(1)) {
    await expect(page.locator(`#${s.sectionId}`)).not.toBeVisible();
  }
});

test('navigates between all 7 sections', async ({ page }) => {
  await page.goto('/');

  for (const { nav, sectionId } of SECTIONS) {
    await page.getByRole('link', { name: nav }).click();
    await expect(page.locator(`#${sectionId}`)).toBeVisible();

    // Verify active nav class
    const activeNav = page.locator('.nav-item.active');
    await expect(activeNav).toContainText(nav);
  }
});

test('health check shows healthy status', async ({ page }) => {
  await page.goto('/');

  // Health check polls every 10s, wait for the "healthy" indicator
  await expect(page.locator('.status-ok')).toContainText('healthy', {
    timeout: 15_000,
  });
});
