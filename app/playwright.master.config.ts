import { defineConfig, devices } from '@playwright/test'
import path from 'path'

/**
 * Standalone Playwright configuration for master test
 * This bypasses the global setup that's causing file descriptor issues
 */
export default defineConfig({
  testDir: './tests/e2e',
  timeout: 300000, // 5 minutes for complete auth + chat flow
  expect: {
    timeout: 10000
  },
  fullyParallel: false, // Electron apps should run sequentially
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1, // Only one worker for Electron
  reporter: [
    ['html', { outputFolder: 'test-results/html' }],
    ['json', { outputFile: 'test-results/results.json' }],
    ['line'] // Add line reporter for better console output
  ],
  outputDir: 'test-results/artifacts',

  use: {
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure'
  },

  // NO GLOBAL SETUP - the master test will handle backend setup internally
  // But we still need teardown to clean up any running processes
  globalTeardown: require.resolve('./tests/e2e/global-teardown.ts'),

  projects: [
    {
      name: 'master',
      testMatch: ['**/master.e2e.ts'],
      use: {
        ...devices['Desktop Chrome']
      }
    }
  ]
})
