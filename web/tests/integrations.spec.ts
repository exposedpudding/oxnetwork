import { expect, test } from '@playwright/test'

test('integrations page lists every source and says which are not configured', async ({ page }) => {
  await page.goto('/integrations')
  await expect(page.getByRole('heading', { name: 'Integrations', exact: true })).toBeVisible()
  const table = page.getByRole('table', { name: 'Integration health' })
  await expect(table.locator('tbody tr')).toHaveCount(6)
  await expect(table.getByText('Demo inventory provider')).toBeVisible()
  await expect(table.getByText('Simulated data. No live infrastructure is connected.')).toBeVisible()
  await expect(table.getByText('Not configured').first()).toBeVisible()
  await page.getByRole('link', { name: 'Integrations' }).click()
  await expect(page).toHaveURL(/\/integrations$/)
})
