import { expect, test } from '@playwright/test'

test('network page uses recorded demo graph and opens selected node', async ({ page }) => {
  await page.goto('/network')
  await expect(page.getByRole('heading', { name: 'Network', exact: true })).toBeVisible()
  await expect(page.getByText('This map does not describe the live network.')).toBeVisible()
  await expect(page.getByRole('group', { name: 'Network topology' })).toBeVisible()
  const connections = page.getByRole('table', { name: 'Network connections' })
  await expect(connections.locator('tbody tr')).toHaveCount(3)
  await expect(connections.getByText('stale', { exact: true })).toBeVisible()
  await page.locator('[data-node-id="node:demo-worker"]').click()
  await expect(page.getByRole('heading', { name: 'Demo Worker' })).toBeVisible()
  await page.getByRole('link', { name: 'Open node details' }).click()
  await expect(page).toHaveURL(/\/nodes\/demo-worker$/)
})

test('network search does not invent connections between unrelated inventory', async ({ page }) => {
  await page.goto('/network')
  await page.getByRole('searchbox', { name: 'Search network resources' }).fill('Monitoring')
  await expect(page.getByText('No connections found')).toBeVisible()
  await expect(page.getByText('No recorded connections match your search.')).toBeVisible()
})
