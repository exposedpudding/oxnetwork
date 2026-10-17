import { expect, test } from '@playwright/test';

// Runs against the fixture-backed dev harness (vite dev server), so no backend is required.

test('Overview loads with node cards, services and a stale node', async ({ page }) => {
  await page.goto('/fixtures.html#/dev/overview');
  await expect(page.getByRole('heading', { level: 1, name: 'Overview' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'forge' })).toBeVisible();
  await expect(page.getByRole('heading', { name: 'Services health' })).toBeVisible();
  // stale node is flagged, and its missing metrics are not fabricated
  const sparrow = page.locator('section', { has: page.getByRole('heading', { name: 'sparrow' }) });
  await expect(sparrow.getByText('Stale')).toBeVisible();
  await expect(sparrow.getByText('Not reported')).toHaveCount(2);
  await page.screenshot({ path: 'test-results/overview.png', fullPage: true });
});

test('Compute renders workers and jobs', async ({ page }) => {
  await page.goto('/fixtures.html#/dev/compute');
  const workers = page.getByRole('table', { name: 'Workers' });
  await expect(workers.getByRole('row')).toHaveCount(3); // header + 2
  await expect(workers.getByText('Draining')).toBeVisible();
  const jobs = page.getByRole('table', { name: 'Jobs' });
  await expect(jobs.getByRole('row', { name: /nightly-lint/ })).toContainText('Queued');
  await expect(jobs.getByRole('row', { name: /export-report/ })).toContainText('Failed');
  await page.screenshot({ path: 'test-results/compute.png', fullPage: true });
});

test('Node Detail shows "Not reported" for a missing metric and hides unsupported actions', async ({ page }) => {
  await page.goto('/fixtures.html#/dev/node/nd-0003'); // reports no temperature, no compute capability
  await expect(page.getByRole('heading', { name: 'beacon' })).toBeVisible();
  await expect(page.getByText('Temperature').locator('..')).toContainText('Not reported');
  await expect(page.getByRole('button', { name: 'Drain node' })).toHaveCount(0);

  await page.goto('/fixtures.html#/dev/node/nd-0001'); // reports temperature
  await expect(page.getByText('Temperature').locator('..')).toContainText('54 °C');
  await page.screenshot({ path: 'test-results/node-detail.png', fullPage: true });
});

test('Network node selection changes the details pane', async ({ page }) => {
  await page.goto('/fixtures.html#/dev/network');
  const title = page.getByTestId('detail-title');
  await expect(title).toHaveText('beacon');
  await page.locator('svg [data-node-id="g-a"]').click();
  await expect(title).toHaveText('forge');
  await expect(page.locator('svg [data-node-id="g-a"]')).toHaveAttribute('aria-pressed', 'true');
  // keyboard path
  await page.locator('svg [data-node-id="g-c"]').focus();
  await page.keyboard.press('Enter');
  await expect(title).toHaveText('sparrow');
  // text fallback exists
  await expect(page.getByRole('list', { name: 'Network nodes' })).toBeVisible();
  await page.screenshot({ path: 'test-results/network.png', fullPage: true });
});

test('Automations manual run requires confirmation', async ({ page }) => {
  await page.goto('/fixtures.html#/dev/automations');
  await page.getByRole('button', { name: 'Run now' }).click();
  const dialog = page.getByRole('alertdialog');
  await expect(dialog).toContainText('Run "Nightly backup" now?');
  await dialog.getByRole('button', { name: 'Cancel' }).click();
  await expect(page.getByRole('status').filter({ hasText: 'Run requested' })).toHaveCount(0);

  await page.getByRole('button', { name: 'Run now' }).click();
  await page.getByRole('alertdialog').getByRole('button', { name: 'Run workflow' }).click();
  await expect(page.getByRole('status').filter({ hasText: 'Run requested for Nightly backup' })).toBeVisible();
  await page.screenshot({ path: 'test-results/automations.png', fullPage: true });
});

test('Automations: create a workflow via the editor', async ({ page }) => {
  await page.goto('/fixtures.html#/dev/automations');
  await page.getByRole('button', { name: 'Create workflow' }).click();
  await page.getByRole('button', { name: 'Save workflow' }).click();
  await expect(page.getByText('Name is required.')).toBeVisible();
  await page.getByLabel('Name', { exact: true }).fill('Weekly wake');
  await page.getByRole('button', { name: /Add action step/ }).click();
  await page.getByLabel('Step 1 name').fill('Wake');
  await page.getByLabel('Step 1 action').selectOption('wake');
  await page.getByLabel('Step 1 target').selectOption('nd-0002');
  await page.getByRole('button', { name: 'Save workflow' }).click();
  await expect(page.getByRole('row', { name: /Weekly wake/ })).toBeVisible();
});

test('Destructive action needs a typed confirmation and is cancellable', async ({ page }) => {
  await page.goto('/fixtures.html#/dev/node/nd-0001');
  const opener = page.getByRole('button', { name: 'Drain node' });
  await opener.click();
  const dialog = page.getByRole('alertdialog');
  const confirm = dialog.getByRole('button', { name: 'Drain node' });
  await expect(confirm).toBeDisabled();
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expect(opener).toBeFocused(); // focus returned

  await opener.click();
  await dialog.getByLabel(/Type forge to confirm/).fill('forge');
  await expect(confirm).toBeEnabled();
  await confirm.click();
  await expect(page.getByRole('status').filter({ hasText: 'Drain requested for forge' })).toBeVisible();
});

test('Ctrl+K opens the command palette and navigates', async ({ page }) => {
  await page.goto('/fixtures.html#/dev/overview');
  await expect(page.getByRole('heading', { level: 1, name: 'Overview' })).toBeVisible(); // app mounted
  await page.keyboard.press('Control+k');
  await expect(page.getByRole('dialog', { name: 'Command palette' })).toBeVisible();
  await page.keyboard.type('compute');
  await page.keyboard.press('Enter');
  await expect(page.getByRole('heading', { level: 1, name: 'Compute' })).toBeVisible();
});
