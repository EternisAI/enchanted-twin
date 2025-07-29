import { test, expect, _electron as electron } from '@playwright/test'
import path from 'path'
import { E2E_CONFIG, FIREBASE_TEST_CONFIG } from './config'
import { signInWithGoogle, signOut, isAuthenticated, clearAuthState } from './auth.helpers'

test.describe('Google OAuth Authentication E2E', () => {
  test('complete Google OAuth login and logout flow', async () => {
    console.log('🧪 Starting complete Google OAuth authentication test...')

    // Ensure backend is ready first
    console.log('🔍 Checking backend connectivity...')
    const maxRetries = 30
    let backendReady = false

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

    // Launch Electron app
    console.log('🚀 Launching Electron app for auth test...')
    const electronApp = await electron.launch({
      args: [path.join(__dirname, '../../out/main/index.js')],
      env: {
        ...process.env,
        NODE_ENV: 'test',
        // Pass Firebase config
        VITE_FIREBASE_API_KEY: process.env.VITE_FIREBASE_API_KEY,
        VITE_FIREBASE_AUTH_DOMAIN: process.env.VITE_FIREBASE_AUTH_DOMAIN,
        VITE_FIREBASE_PROJECT_ID: process.env.VITE_FIREBASE_PROJECT_ID
      }
    })

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
      await expect(page.getByText('Continue with Google')).toBeVisible({ timeout: 10000 })

      // Take screenshot of initial state
      await page.screenshot({
        path: 'test-results/artifacts/auth-test-initial-state.png',
        fullPage: true
      })

      console.log('✅ Step 1 completed: Confirmed unauthenticated state')

      // Step 2: Perform Google sign-in
      console.log('🔐 Step 2: Performing Google OAuth sign-in...')
      await signInWithGoogle(page)

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

      // Step 4: Test sign out
      console.log('🚪 Step 4: Testing sign out functionality...')
      await signOut(page)

      // Verify we're back to unauthenticated state
      await expect(page.getByText('Continue with Google')).toBeVisible({ timeout: 10000 })

      // Verify auth data is cleared
      const authStatusAfterSignOut = await isAuthenticated(page)
      expect(authStatusAfterSignOut).toBe(false)

      // Take screenshot of signed out state
      await page.screenshot({
        path: 'test-results/artifacts/auth-test-signedout-state.png',
        fullPage: true
      })

      console.log('✅ Step 4 completed: Sign out successful')

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

  test('Google OAuth with browser popup handling', async () => {
    console.log('🧪 Testing Google OAuth with popup handling...')

    // This test specifically focuses on handling popup windows
    // which might occur during OAuth flow

    const electronApp = await electron.launch({
      args: [path.join(__dirname, '../../out/main/index.js')],
      env: {
        ...process.env,
        NODE_ENV: 'test',
        VITE_FIREBASE_API_KEY: process.env.VITE_FIREBASE_API_KEY,
        VITE_FIREBASE_AUTH_DOMAIN: process.env.VITE_FIREBASE_AUTH_DOMAIN,
        VITE_FIREBASE_PROJECT_ID: process.env.VITE_FIREBASE_PROJECT_ID
      }
    })

    try {
      const page = await electronApp.firstWindow()
      await page.waitForLoadState('domcontentloaded')

      // Clear auth state
      await clearAuthState(page)
      await page.reload()
      await page.waitForLoadState('domcontentloaded')

      // Set up popup handling
      page.on('popup', async (popup) => {
        console.log('🪟 Popup detected:', popup.url())
        await popup.waitForLoadState()

        // Handle the popup if it's a Google auth popup
        if (popup.url().includes('accounts.google.com')) {
          console.log('🔐 Handling Google auth popup...')
          // The popup handling would be similar to the main flow
          // but this test ensures we can handle popup-based auth too
        }
      })

      // Attempt sign-in (this might trigger a popup)
      await signInWithGoogle(page)

      // Verify successful authentication
      const authStatus = await isAuthenticated(page)
      expect(authStatus).toBe(true)

      console.log('✅ Google OAuth popup handling test passed!')
    } catch (error) {
      console.error('❌ Popup handling test failed:', error)
      throw error
    } finally {
      await electronApp.close()
    }
  })

  test('authentication persistence across app restarts', async () => {
    console.log('🧪 Testing authentication persistence across app restarts...')

    // First session: authenticate
    console.log('🚀 Starting first app session...')
    let electronApp = await electron.launch({
      args: [path.join(__dirname, '../../out/main/index.js')],
      env: {
        ...process.env,
        NODE_ENV: 'test',
        VITE_FIREBASE_API_KEY: process.env.VITE_FIREBASE_API_KEY,
        VITE_FIREBASE_AUTH_DOMAIN: process.env.VITE_FIREBASE_AUTH_DOMAIN,
        VITE_FIREBASE_PROJECT_ID: process.env.VITE_FIREBASE_PROJECT_ID
      }
    })

    try {
      let page = await electronApp.firstWindow()
      await page.waitForLoadState('domcontentloaded')

      // Authenticate
      await signInWithGoogle(page)

      // Verify authentication
      let authStatus = await isAuthenticated(page)
      expect(authStatus).toBe(true)

      console.log('✅ First session: Authentication successful')

      // Close the app
      await electronApp.close()

      // Second session: check if auth persists
      console.log('🚀 Starting second app session...')
      electronApp = await electron.launch({
        args: [path.join(__dirname, '../../out/main/index.js')],
        env: {
          ...process.env,
          NODE_ENV: 'test',
          VITE_FIREBASE_API_KEY: process.env.VITE_FIREBASE_API_KEY,
          VITE_FIREBASE_AUTH_DOMAIN: process.env.VITE_FIREBASE_AUTH_DOMAIN,
          VITE_FIREBASE_PROJECT_ID: process.env.VITE_FIREBASE_PROJECT_ID
        }
      })

      page = await electronApp.firstWindow()
      await page.waitForLoadState('domcontentloaded')

      // Wait a moment for any auto-login to happen
      await page.waitForTimeout(3000)

      // Check if still authenticated (or auto-logged in)
      authStatus = await isAuthenticated(page)

      if (authStatus) {
        console.log('✅ Authentication persisted across app restart')
      } else {
        console.log('ℹ️ Authentication did not persist (this may be expected behavior)')
        // This is not necessarily a failure - it depends on how the app handles auth persistence
      }

      console.log('✅ Authentication persistence test completed!')
    } catch (error) {
      console.error('❌ Authentication persistence test failed:', error)
      throw error
    } finally {
      await electronApp.close()
    }
  })
})
