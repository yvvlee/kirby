import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: '.',
  testMatch: process.env.KIRBY_E2E_SETUP === '1' ? 'setup.spec.js' : 'user-journey.spec.js',
  timeout: 60_000,
  expect: { timeout: 10_000 },
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: 'list',
  use: {
    baseURL: process.env.KIRBY_LOCAL_WEB_URL || 'http://127.0.0.1:15173',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [
    {
      name: 'chromium',
      use: { browserName: 'chromium', channel: 'chromium' },
    },
  ],
})
