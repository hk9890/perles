import { defineConfig } from '@playwright/test'

export default defineConfig({
  testDir: './tests',
  timeout: 30000,
  use: {
    screenshot: 'on',
    trace: 'on-first-retry',
  },
})
