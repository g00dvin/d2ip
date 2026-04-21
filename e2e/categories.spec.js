import { test, expect } from './fixtures.js';

test('available categories load with geosite prefix', async ({ page }) => {
  await page.goto('/');
  await page.locator('.nav-item[data-section="categories"]').click();
  const available = page.locator('#available-categories');
  await expect(available).not.toHaveText('loading...', { timeout: 15_000 });
  const addButtons = page.locator('#available-categories button:has-text("add")');
  await expect(await addButtons.count()).toBeGreaterThan(0);
  const firstCategory = page.locator('#available-categories > div > span').first();
  const text = await firstCategory.textContent();
  expect(text).toMatch(/^geosite:/);
});

test('search filters available categories', async ({ page }) => {
  await page.goto('/');
  await page.locator('.nav-item[data-section="categories"]').click();
  await expect(page.locator('#available-categories')).not.toHaveText('loading...', { timeout: 15_000 });
  const initialButtons = await page.locator('#available-categories button:has-text("add")').count();
  const searchInput = page.locator('#available-search');
  await searchInput.fill('google');
  const filteredButtons = await page.locator('#available-categories button:has-text("add")').count();
  expect(filteredButtons).toBeLessThanOrEqual(initialButtons);
  await searchInput.clear();
  const restoredButtons = await page.locator('#available-categories button:has-text("add")').count();
  expect(restoredButtons).toBe(initialButtons);
});

test('add category from available to configured', async ({ page }) => {
  await page.goto('/');
  await page.locator('.nav-item[data-section="categories"]').click();
  await expect(page.locator('#available-categories')).not.toHaveText('loading...', { timeout: 15_000 });
  const firstCatEl = page.locator('#available-categories > div > span').first();
  const categoryName = await firstCatEl.textContent();
  const firstRow = page.locator('#available-categories > div').first();
  await firstRow.locator('button:has-text("add")').click();
  await expect(page.locator('#toast')).toContainText('added:', { timeout: 10_000 });
  // Wait for API to confirm the category was added
  await page.waitForFunction(async (name) => {
    const resp = await fetch('/api/categories');
    const data = await resp.json();
    return data.configured?.some(c => c.code === name);
  }, categoryName, { timeout: 15_000 });
  // Navigate to categories page fresh
  await page.goto('/');
  await page.locator('.nav-item[data-section="categories"]').click();
  await expect(page.locator('#configured-categories')).not.toHaveText('loading...', { timeout: 15_000 });
  await expect(page.locator('#configured-categories')).toContainText(categoryName, { timeout: 10_000 });
});

test('remove category from configured', async ({ page }) => {
  await page.goto('/');
  await page.locator('.nav-item[data-section="categories"]').click();
  await expect(page.locator('#configured-categories')).not.toHaveText('loading...', { timeout: 15_000 });
  await expect(page.locator('#available-categories')).not.toHaveText('loading...', { timeout: 15_000 });
  const firstCatEl = page.locator('#available-categories > div > span').first();
  const categoryName = await firstCatEl.textContent();
  const firstRow = page.locator('#available-categories > div').first();
  await firstRow.locator('button:has-text("add")').click();
  await expect(page.locator('#toast')).toContainText('added:', { timeout: 10_000 });
  // Wait for API to confirm the category was added
  await page.waitForFunction(async (name) => {
    const resp = await fetch('/api/categories');
    const data = await resp.json();
    return data.configured?.some(c => c.code === name);
  }, categoryName, { timeout: 15_000 });
  // Navigate to categories page fresh
  await page.goto('/');
  await page.locator('.nav-item[data-section="categories"]').click();
  await expect(page.locator('#configured-categories')).not.toHaveText('loading...', { timeout: 15_000 });
  await expect(page.locator('#configured-categories')).toContainText(categoryName, { timeout: 10_000 });
  // Now remove it
  page.on('dialog', (dialog) => dialog.accept());
  const row = page.locator('#configured-categories tr', { hasText: categoryName });
  await row.locator('button:has-text("✕")').click();
  await expect(page.locator('#toast')).toContainText('removed:', { timeout: 10_000 });
  // Wait for API to confirm removal
  await page.waitForFunction(async (name) => {
    const resp = await fetch('/api/categories');
    const data = await resp.json();
    return !data.configured?.some(c => c.code === name);
  }, categoryName, { timeout: 15_000 });
  // Navigate to categories page fresh
  await page.goto('/');
  await page.locator('.nav-item[data-section="categories"]').click();
  await expect(page.locator('#configured-categories')).not.toHaveText('loading...', { timeout: 15_000 });
  await expect(page.locator('#configured-categories')).not.toContainText(categoryName, { timeout: 10_000 });
});

test('browse category domains', async ({ page }) => {
  await page.goto('/');
  await page.locator('.nav-item[data-section="categories"]').click();
  await expect(page.locator('#configured-categories')).not.toHaveText('loading...', { timeout: 15_000 });

  // Check if we have any configured categories by looking for a table
  const panelText = await page.locator('#configured-categories').textContent().catch(() => '');
  const hasCategories = panelText.includes('geosite:');

  if (!hasCategories) {
    await expect(page.locator('#available-categories')).not.toHaveText('loading...', { timeout: 15_000 });
    const firstRow = page.locator('#available-categories > div').first();
    const categoryName = await firstRow.locator('span').textContent();
    await firstRow.locator('button:has-text("add")').click();
    await expect(page.locator('#toast')).toContainText('added:', { timeout: 10_000 });
    // Wait for API to confirm the category was added
    await page.waitForFunction(async (name) => {
      const resp = await fetch('/api/categories');
      const data = await resp.json();
      return data.configured?.some(c => c.code === name);
    }, categoryName, { timeout: 15_000 });
    // Navigate to categories page fresh
    await page.goto('/');
    await page.locator('.nav-item[data-section="categories"]').click();
    await expect(page.locator('#configured-categories')).not.toHaveText('loading...', { timeout: 15_000 });
    // Wait for the table to appear (it may take a moment for HTMX to render)
    await expect(page.locator('#configured-categories table')).toBeVisible({ timeout: 10_000 });
  }

  // Now the table should be visible
  const configuredTable = page.locator('#configured-categories table');

  // Click "browse" on the first configured category
  const browseRow = configuredTable.locator('tr').first();
  await browseRow.locator('button:has-text("browse")').click();
  await expect(page.locator('#category-domains-panel')).toBeVisible({ timeout: 10_000 });
  const domainList = page.locator('#domain-list');
  await expect(domainList).not.toBeEmpty({ timeout: 10_000 });
});
