import { defineConfig } from '@playwright/test'

// Multi-screen visual smoke tests against the fixture-backed dev harness (fixtures.html on the Vite dev server).
// No Go backend is needed. The production smoke test (playwright.config.ts) runs against the Go server.
export default defineConfig({
  testDir: './tests/fixtures',
  fullyParallel: true,
  reporter: 'list',
  use: {
    baseURL: 'http://127.0.0.1:5173',
    browserName: 'chromium',
    launchOptions: process.env.CHROMIUM_PATH ? { executablePath: process.env.CHROMIUM_PATH, args: ['--no-sandbox'] } : undefined,
  },
  webServer: { command: 'npm run dev -- --strictPort --port 5173', url: 'http://127.0.0.1:5173/fixtures.html', reuseExistingServer: !process.env.CI },
})
