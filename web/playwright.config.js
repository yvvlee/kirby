import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests/e2e',
  globalTeardown: './tests/e2e/teardown.mjs',
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: 'list',
  use: {
    baseURL: 'http://127.0.0.1:14173',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  webServer: {
    command: 'node tests/e2e/harness.mjs',
    url: 'http://127.0.0.1:14173/login',
    timeout: 120_000,
    reuseExistingServer: false,
  },
  projects: [{ name: 'chromium', use: { browserName: 'chromium', channel: 'chromium' } }],
})
