import { expect, test } from '@playwright/test'

test('automations lists simulated demo workflows with real summary data', async ({ page }) => {
  await page.goto('/automations')
  await expect(page.getByRole('heading', { name: 'Automations', exact: true })).toBeVisible()
  await expect(page.getByText('These workflows are simulated')).toBeVisible()
  const table = page.getByRole('table', { name: 'Workflows' })
  // exact: the row's Run/Disable buttons also contain the workflow name in their labels.
  await expect(table.getByRole('button', { name: 'Health check sweep', exact: true })).toBeVisible()
  await expect(table.getByRole('button', { name: 'Check worker on demand', exact: true })).toBeVisible()
  await expect(table.getByText('Disabled')).toBeVisible()
  await expect(page.getByRole('group', { name: 'Scheduled triggers' })).toContainText('Next run')
  await expect(page.getByRole('list', { name: /workflow$/ })).toBeVisible()
  await page.getByRole('searchbox', { name: 'Search workflows' }).fill('absent')
  await expect(page.getByText('No workflows match your search')).toBeVisible()
})

test('a manual run needs confirmation and records step results', async ({ page }) => {
  await page.goto('/automations')
  await page.getByRole('button', { name: 'Run Check worker on demand' }).click()
  const dialog = page.getByRole('alertdialog', { name: /Run .Check worker on demand. now\?/ })
  await expect(dialog).toBeVisible()
  await dialog.getByRole('button', { name: 'Cancel' }).click()
  await expect(dialog).toBeHidden()
  await expect(page.getByText('Started a run of Check worker on demand.')).toHaveCount(0)

  await page.getByRole('button', { name: 'Run Check worker on demand' }).click()
  await page.getByRole('alertdialog').getByRole('button', { name: 'Run now' }).click()
  await expect(page.getByText('Started a run of Check worker on demand.')).toBeVisible()
  await page.getByRole('tab', { name: 'Run history' }).click()
  await page.getByRole('button', { name: 'Show steps' }).first().click()
  const steps = page.getByRole('list', { name: /Steps for run/ })
  await expect(steps).toContainText('Check worker report freshness')
  await expect(steps).toContainText('Failure notification')
})

test('a new workflow can be created and run-gated like the rest', async ({ page }) => {
  const name = `Smoke workflow ${Date.now()}`
  await page.goto('/automations')
  await page.getByRole('button', { name: /Create workflow/ }).click()
  await page.getByLabel('Name', { exact: true }).fill(name)
  await page.getByRole('button', { name: /Add action step/ }).click()
  await page.getByLabel('Step 1 name').fill('Check the gateway')
  await page.getByLabel('Step 1 action').selectOption('check_node_health')
  await page.getByLabel('Step 1 target').selectOption('demo-gateway')
  await page.getByRole('button', { name: 'Save workflow' }).click()
  await expect(page.getByText(`Saved “${name}”.`)).toBeVisible()
  await expect(page.getByRole('table', { name: 'Workflows' }).getByRole('button', { name, exact: true })).toBeVisible()
  await page.getByRole('button', { name: `Run ${name}` }).click()
  await expect(page.getByRole('alertdialog')).toBeVisible()
})
