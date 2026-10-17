import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests',
  use: { baseURL: 'http://127.0.0.1:8080', browserName: 'chromium', launchOptions: process.env.CHROMIUM_PATH ? { executablePath: process.env.CHROMIUM_PATH, args: ['--no-sandbox'] } : undefined },
  webServer: { command: 'cd .. && go run ./cmd/server', url: 'http://127.0.0.1:8080/readyz', reuseExistingServer: !process.env.CI, timeout: 120000 }
})
