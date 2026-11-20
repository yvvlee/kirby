import { defineConfig } from '@playwright/test'
export default defineConfig({
  testDir: './tests/e2e',
  testMatch: 'project-access-live.spec.js',
  workers: 1,
  timeout: 60000,
  use: {
    channel: 'chromium',
    baseURL: process.env.KIRBY_TEST_URL ?? 'http://127.0.0.1:18089',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
})
