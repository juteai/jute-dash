import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './e2e',
  testMatch: /real-stack\.spec\.ts/,
  outputDir: 'test-results/real-stack',
  webServer: [
    {
      command:
        'cd ../../examples/agents/mock-agent && MOCK_A2A_LISTEN=127.0.0.1:9797 go run .',
      url: 'http://127.0.0.1:9797/.well-known/agent-card.json',
      reuseExistingServer: !process.env.CI,
      timeout: 120_000
    },
    {
      command:
        'cd ../hub && go run ./cmd/juted -config ../../examples/config/local/config.yaml',
      url: 'http://127.0.0.1:8787/healthz',
      reuseExistingServer: !process.env.CI,
      timeout: 120_000
    },
    {
      command: 'npm run dev -- --host 127.0.0.1',
      url: 'http://127.0.0.1:5173/',
      reuseExistingServer: !process.env.CI,
      timeout: 120_000
    }
  ],
  use: {
    baseURL: 'http://127.0.0.1:5173',
    trace: 'on-first-retry'
  },
  projects: [
    {
      name: 'chromium-real-stack',
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 1280, height: 800 }
      }
    }
  ]
});
