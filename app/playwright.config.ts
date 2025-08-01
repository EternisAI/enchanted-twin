import { defineConfig, devices } from '@playwright/test'
import path from 'path'
import { E2E_CONFIG, AUTH_CONFIG } from './tests/e2e/config'

export default defineConfig({
  testDir: './tests/e2e',
  timeout: E2E_CONFIG.APP_STARTUP_TIMEOUT, // Use shared timeout configuration
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

  projects: [
    // Setup project for authentication
    {
      name: 'setup',
      testMatch: '**/auth.setup.ts',
      teardown: 'cleanup'
    },

    // Cleanup project
    {
      name: 'cleanup',
      testMatch: '**/cleanup.ts' // We can add this later if needed
    },

    // Basic tests that don't require authentication
    {
      name: 'basic',
      testMatch: ['**/app.e2e.ts'],
      use: {
        ...devices['Desktop Chrome']
      }
    },

    // Direct authentication tests (test the login flow itself)
    {
      name: 'auth-flow',
      testMatch: ['**/auth.e2e.ts'],
      use: {
        ...devices['Desktop Chrome']
      }
    },

    // Tests that require authentication (use clean cache with mock auth)
    {
      name: 'authenticated',
      testMatch: ['**/*.auth.e2e.ts'],
      use: {
        ...devices['Desktop Chrome']
        // Removed storageState to ensure clean cache on each run
      }
      // Removed dependencies to avoid relying on setup project
    },

    // Smoke tests (can run independently)
    {
      name: 'smoke',
      testMatch: ['**/smoke.*.ts'],
      use: {
        ...devices['Desktop Chrome']
      }
    }
  ],

  // Global setup and teardown for backend server
  globalSetup: require.resolve('./tests/e2e/global-setup.ts'),
  globalTeardown: require.resolve('./tests/e2e/global-teardown.ts')
})
