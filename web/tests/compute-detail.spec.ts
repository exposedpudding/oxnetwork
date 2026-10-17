import { expect, test } from '@playwright/test'

test('compute page uses API capacity and links to a stale worker', async ({ page }) => {
  await page.goto('/compute')
  await expect(page.getByRole('heading', { name: 'Compute', exact: true })).toBeVisible()
  await expect(page.getByText('No job provider or scheduler is connected.')).toBeVisible()
  await expect(page.getByText('Worker capacity is stale')).toBeVisible()
  await expect(page.getByRole('group', { name: 'Reported capacity' })).toContainText('8 vCPU')
  await expect(page.getByRole('table', { name: 'Compute workers' }).getByRole('link', { name: 'Demo Worker' })).toBeVisible()
  await expect(page.getByText('No jobs reported')).toBeVisible()
  await page.getByRole('searchbox', { name: 'Search workers and jobs' }).fill('absent')
  await expect(page.getByText('No workers match your search.')).toBeVisible()
  await page.getByRole('searchbox', { name: 'Search workers and jobs' }).clear()
  await page.getByRole('table', { name: 'Compute workers' }).getByRole('link', { name: 'Demo Worker' }).click()
  await expect(page).toHaveURL(/\/nodes\/demo-worker$/)
  await expect(page.getByRole('heading', { name: 'Demo Worker' })).toBeVisible()
})

test('node detail shows missing metrics and only reported capabilities', async ({ page }) => {
  await page.goto('/')
  await page.getByRole('link', { name: 'Demo Sensor' }).click()
  await expect(page).toHaveURL(/\/nodes\/demo-sensor$/)
  await expect(page.getByRole('heading', { name: 'Demo Sensor' })).toBeVisible()
  await expect(page.getByRole('group', { name: 'vCPU' })).toContainText('Not reported')
  await expect(page.getByText('Not reported').first()).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Capabilities' })).toBeVisible()
  await expect(page.getByText('monitor', { exact: true })).toBeVisible()
  await expect(page.getByText('No node actions are available in demo mode.')).toBeVisible()
})
