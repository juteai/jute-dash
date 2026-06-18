import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  testIgnore: /real-stack\.spec\.ts/,
  outputDir: 'test-results',
  reporter: process.env.CI ? [['html', { open: 'never' }], ['line']] : 'list',
  workers: process.env.CI ? 2 : undefined,
  retries: process.env.CI ? 1 : 0,
  webServer: {
    command: 'npm run dev -- --host 127.0.0.1',
    url: 'http://127.0.0.1:5173/__visual__',
    reuseExistingServer: !process.env.CI,
    timeout: 120_000
  },
  use: {
    baseURL: 'http://127.0.0.1:5173',
    trace: 'on-first-retry'
  },
  projects: [
    {
      name: 'chromium-desktop',
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 1280, height: 800 }
      }
    },
    {
      name: 'chromium-phone',
      use: { ...devices['Pixel 5'] }
    }
  ]
});
