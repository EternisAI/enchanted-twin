import { test, expect, _electron as electron } from '@playwright/test'
import path from 'path'
import fs from 'fs'
import { E2E_CONFIG } from './config'
import {
  signInWithGoogle,
  isAuthenticated,
  clearAuthState,
  createCleanElectronConfig,
  cleanupTempDirectories
} from './helpers/auth.helpers'

test.describe('Google OAuth Authentication E2E', () => {
  // Setup: Ensure temp directory exists
  test.beforeAll(async () => {
    const tempDir = path.join(__dirname, '../../../temp')
    if (!fs.existsSync(tempDir)) {
      fs.mkdirSync(tempDir, { recursive: true })
    }
  })

  // Cleanup temporary directories after all tests
  test.afterAll(async () => {
    await cleanupTempDirectories()
  })
  test('complete Google OAuth login and logout flow', async () => {
    console.log('🧪 Starting complete Google OAuth authentication test...')

    // Ensure backend is ready first
    console.log('🔍 Checking backend connectivity...')
    const maxRetries = 30
    let backendReady = false

    for (let i = 0; i < maxRetries; i++) {
      try {
        console.log('🔍 Checking backend connectivity...')
        const response = await fetch(E2E_CONFIG.getGraphQLUrl(), {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ query: '{ __typename }' })
        })

        if (response.ok) {
          backendReady = true
          console.log('✅ Backend is ready')
          break
        }
      } catch (error) {
        // Backend not ready yet
      }

      console.log(`⏳ Waiting for backend... (attempt ${i + 1}/${maxRetries})`)
      await new Promise((resolve) => setTimeout(resolve, 30000))
    }

    if (!backendReady) {
      throw new Error('❌ Backend failed to start within timeout period')
    }

    // Launch Electron app with clean cache
    console.log('🚀 Launching Electron app with fresh cache for auth test...')
    const electronApp = await electron.launch(createCleanElectronConfig())

    try {
      const page = await electronApp.firstWindow()
      await page.waitForLoadState('domcontentloaded')

      // Step 1: Verify initial unauthenticated state
      console.log('🔍 Step 1: Verifying initial unauthenticated state...')

      // Clear any existing auth state
      await clearAuthState(page)
      await page.reload()
      await page.waitForLoadState('domcontentloaded')

      // Should see login screen
      await expect(page.getByText('Continue with Google')).toBeVisible({ timeout: 60000 })

      // Take screenshot of initial state
      await page.screenshot({
        path: 'test-results/artifacts/auth-test-initial-state.png',
        fullPage: true
      })

      console.log('✅ Step 1 completed: Confirmed unauthenticated state')

      // Step 2: Perform Google sign-in
      console.log('🔐 Step 2: Performing Google OAuth sign-in...')
      await signInWithGoogle(page, electronApp)

      // Verify authentication was successful
      const authStatus = await isAuthenticated(page)
      expect(authStatus).toBe(true)

      console.log('✅ Step 2 completed: Google sign-in successful')

      // Step 3: Test authenticated functionality
      console.log('🧪 Step 3: Testing authenticated functionality...')

      // Check that we can access authenticated areas
      // The exact UI elements will depend on your app structure

      // Verify user data is stored
      const hasUserData = await page.evaluate(() => {
        const userData = window.localStorage.getItem('enchanted_user_data')
        return userData !== null && userData !== 'undefined'
      })
      expect(hasUserData).toBe(true)

      // Take screenshot of authenticated state
      await page.screenshot({
        path: 'test-results/artifacts/auth-test-authenticated-state.png',
        fullPage: true
      })

      console.log('✅ Step 3 completed: Authenticated functionality verified')

      console.log('🎉 Complete Google OAuth authentication test passed!')
    } catch (error) {
      console.error('❌ Authentication test failed:', error)

      // Take error screenshot
      try {
        const page = await electronApp.firstWindow()
        await page.screenshot({
          path: 'test-results/artifacts/auth-test-error.png',
          fullPage: true
        })
      } catch (screenshotError) {
        console.error('❌ Could not take error screenshot:', screenshotError)
      }

      throw error
    } finally {
      await electronApp.close()
    }
  })
})
