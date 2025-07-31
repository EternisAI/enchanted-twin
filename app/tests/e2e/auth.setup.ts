import { test as setup, expect } from '@playwright/test'
import { _electron as electron } from '@playwright/test'
import path from 'path'
import { signInWithGoogle, clearAuthState, createCleanElectronConfig } from './auth.helpers'
import { E2E_CONFIG, AUTH_CONFIG, FIREBASE_TEST_CONFIG } from './config'

// Path where we'll store the authentication state
const authFile = AUTH_CONFIG.AUTH_STATE_PATH

setup('authenticate with Google and save session', async () => {
  console.log('🚀 Starting authentication setup...')

  // Ensure backend is ready first
  console.log('🔍 Checking backend connectivity...')
  let backendReady = false
  const maxRetries = 30

  for (let i = 0; i < maxRetries; i++) {
    try {
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
    await new Promise((resolve) => setTimeout(resolve, 2000))
  }

  if (!backendReady) {
    throw new Error('❌ Backend failed to start within timeout period')
  }

  // Start the Electron app for authentication
  console.log('🚀 Launching Electron app for authentication setup...')
  const electronApp = await electron.launch({
    args: [path.join(__dirname, '../../out/main/index.js')],
    env: {
      ...process.env,
      NODE_ENV: 'test',
      // Pass Firebase config with correct VITE_ prefix for Electron renderer
      VITE_FIREBASE_API_KEY: FIREBASE_TEST_CONFIG.FIREBASE_API_KEY,
      VITE_FIREBASE_AUTH_DOMAIN: FIREBASE_TEST_CONFIG.FIREBASE_AUTH_DOMAIN,
      VITE_FIREBASE_PROJECT_ID: FIREBASE_TEST_CONFIG.FIREBASE_PROJECT_ID
    }
  })

  try {
    const page = await electronApp.firstWindow()
    await page.waitForLoadState('domcontentloaded')

    // Clear any existing auth state
    await clearAuthState(page)

    // Perform Google authentication
    console.log('🔐 Starting Google authentication flow...')
    await signInWithGoogle(page, electronApp)

    // Verify authentication was successful
    const userDataExists = await page.evaluate(() => {
      return window.localStorage.getItem('enchanted_user_data') !== null
    })

    if (!userDataExists) {
      throw new Error('❌ Authentication failed - no user data found in localStorage')
    }

    console.log('✅ Authentication successful')

    // Save authentication state to file
    console.log(`💾 Saving authentication state to ${authFile}...`)
    await page.context().storageState({ path: authFile })

    console.log('✅ Authentication setup completed successfully!')

    // Take a final screenshot of authenticated state
    await page.screenshot({
      path: 'test-results/artifacts/auth-setup-final-state.png',
      fullPage: true
    })
  } catch (error) {
    console.error('❌ Authentication setup failed:', error)

    // Try to take screenshot for debugging
    try {
      const page = await electronApp.firstWindow()
      await page.screenshot({
        path: 'test-results/artifacts/auth-setup-error.png',
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

// Optional: Add a cleanup setup that runs after tests
setup('cleanup authentication artifacts', async () => {
  console.log('🧹 Cleaning up authentication artifacts...')

  // This setup runs after tests to clean up any test artifacts
  // For now, we'll just log that cleanup is complete
  console.log('✅ Cleanup completed')
})
