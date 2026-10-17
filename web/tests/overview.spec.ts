import { expect, test } from '@playwright/test'

test('demo overview renders API inventory and clearly identifies simulation', async ({ page }) => {
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()
  await expect(page.getByText('No live infrastructure is connected')).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Demo Gateway' })).toBeVisible()
  await expect(page.getByText('Stale').first()).toBeVisible()
  await page.getByRole('searchbox', { name: 'Search nodes and services' }).fill('Worker')
  await expect(page.getByRole('heading', { name: 'Demo Worker' })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Demo Gateway' })).toHaveCount(0)
})
