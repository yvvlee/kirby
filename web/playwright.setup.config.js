import { defineConfig } from '@playwright/test'

import configuration from './playwright.config.js'

export default defineConfig({
  ...configuration,
  testMatch: 'setup.spec.js',
  metadata: { setupMode: true },
  webServer: {
    ...configuration.webServer,
    env: {
      KIRBY_E2E_SETUP: '1',
    },
  },
})
