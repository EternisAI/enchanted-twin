import { test, expect, _electron as electron } from '@playwright/test'
import path from 'path'
import fs from 'fs'
import {
  signInWithGoogle,
  isAuthenticated,
  clearAuthState,
  createCleanElectronConfig,
  cleanupTempDirectories
} from './helpers/auth.helpers'
import { waitForChatInterface, runAllChatTests } from './chat.helpers'
import { startCompleteBackend, stopBackendServer } from './helpers/backend.helpers'

test.describe('Master E2E Test Suite - Auth + Chat', () => {
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

  test('complete flow: authentication then chat interaction', async () => {
    console.log('🎯 Starting MASTER E2E test: Backend → Auth → Chat flow...')
    console.log('📋 This test will:')
    console.log('   1. Build and start backend server')
    console.log('   2. Launch Electron app')
    console.log('   3. Perform Google OAuth authentication')
    console.log('   4. Test chat functionality (same instance)')
    console.log('   5. Clean up backend and Electron')

    // ========================================
    // PHASE 1: BACKEND STARTUP
    // ========================================
    console.log('\n🔧 PHASE 1: Starting backend server...')
    console.log('📋 This will:')
    console.log('   - Build the backend server (make build)')
    console.log('   - Start the backend process')
    console.log('   - Wait for GraphQL server to be ready')

    await startCompleteBackend()

    // ========================================
    // PHASE 2: ELECTRON APP LAUNCH
    // ========================================
    console.log('\n🚀 PHASE 2: Launching Electron app...')
    const electronApp = await electron.launch(createCleanElectronConfig())

    try {
      const page = await electronApp.firstWindow()
      await page.waitForLoadState('domcontentloaded')

      // ========================================
      // PHASE 3: AUTHENTICATION
      // ========================================
      console.log('\n🔐 PHASE 3: Performing authentication...')

      // Step 3.1: Verify initial unauthenticated state
      console.log('🔍 Step 3.1: Verifying initial unauthenticated state...')
      await clearAuthState(page)
      await page.reload()
      await page.waitForLoadState('domcontentloaded')

      // Should see login screen
      await expect(page.getByText('Continue with Google')).toBeVisible({ timeout: 60000 })

      await page.screenshot({
        path: 'test-results/artifacts/master-test-initial-state.png',
        fullPage: true
      })

      console.log('✅ Step 3.1 completed: Confirmed unauthenticated state')

      // Step 3.2: Perform Google sign-in
      console.log('🔐 Step 3.2: Performing Google OAuth sign-in...')
      await signInWithGoogle(page, electronApp)

      // Step 3.3: Verify authentication was successful
      console.log('🔍 Step 3.3: Verifying authentication success...')
      const authStatus = await isAuthenticated(page)
      expect(authStatus).toBe(true)

      // Verify user data is stored
      const hasUserData = await page.evaluate(() => {
        const userData = window.localStorage.getItem('enchanted_user_data')
        return userData !== null && userData !== 'undefined'
      })
      expect(hasUserData).toBe(true)

      await page.screenshot({
        path: 'test-results/artifacts/master-test-authenticated-state.png',
        fullPage: true
      })

      console.log('✅ PHASE 3 completed: Authentication successful!')

      // ========================================
      // PHASE 4: CHAT FUNCTIONALITY TESTING
      // ========================================
      console.log('\n💬 PHASE 4: Testing chat functionality (same instance)...')

      // Step 4.1: Wait for chat interface to be ready
      console.log('🔍 Step 4.1: Waiting for chat interface to be ready...')

      // Give the app some time to fully load after authentication
      await page.waitForTimeout(5000)

      try {
        await waitForChatInterface(page)
        console.log('✅ Step 4.1 completed: Chat interface is ready')
      } catch (chatInterfaceError) {
        console.log('⚠️ Chat interface not immediately ready, taking debug screenshot...')
        await page.screenshot({
          path: 'test-results/artifacts/master-test-chat-not-ready.png',
          fullPage: true
        })

        // Try waiting a bit more
        await page.waitForTimeout(10000)
        await waitForChatInterface(page)
        console.log('✅ Step 4.1 completed: Chat interface is ready (after additional wait)')
      }

      // Step 4.2: Run comprehensive chat test suite
      console.log('💬 Step 4.2: Running comprehensive chat test suite...')
      await runAllChatTests(page)
      console.log('✅ Step 4.2 completed: Comprehensive chat test suite passed')

      console.log('✅ PHASE 4 completed: Chat functionality testing finished!')

      // ========================================
      // PHASE 5: FINAL VERIFICATION
      // ========================================
      console.log('\n🎯 PHASE 5: Final verification...')

      // Verify we're still authenticated
      const finalAuthStatus = await isAuthenticated(page)
      expect(finalAuthStatus).toBe(true)

      // Take final success screenshot
      await page.screenshot({
        path: 'test-results/artifacts/master-test-final-success.png',
        fullPage: true
      })

      console.log('✅ PHASE 5 completed: Final verification passed')

      // ========================================
      // TEST COMPLETION
      // ========================================
      console.log('\n🎉 MASTER E2E TEST COMPLETED SUCCESSFULLY!')
      console.log('📊 Test Summary:')
      console.log('   ✅ Backend build and startup')
      console.log('   ✅ Electron app launch')
      console.log('   ✅ Google OAuth authentication')
      console.log('   ✅ Chat interface detection')
      console.log('   ✅ Comprehensive chat test suite')
      console.log('   📸 Screenshots saved to test-results/artifacts/')
    } catch (error) {
      console.error('\n❌ MASTER E2E TEST FAILED:', error)

      // Take comprehensive error screenshot
      try {
        const page = await electronApp.firstWindow()
        await page.screenshot({
          path: 'test-results/artifacts/master-test-error.png',
          fullPage: true
        })

        // Log current page state for debugging
        console.log('🔍 Current page URL:', page.url())
        const bodyText = await page.locator('body').textContent()
        console.log('📄 Page body text (first 500 chars):', bodyText?.substring(0, 500))

        const userDataExists = await page.evaluate(() => {
          return window.localStorage.getItem('enchanted_user_data') !== null
        })
        console.log('💾 User data in localStorage:', userDataExists)
      } catch (screenshotError) {
        console.error('❌ Could not take error screenshot:', screenshotError)
      }

      throw error
    } finally {
      // ========================================
      // CLEANUP
      // ========================================
      console.log('\n🧹 Cleaning up...')

      try {
        console.log('🖥️ Stopping Electron app...')
        await electronApp.close()
        console.log('✅ Electron app closed')
      } catch (error) {
        console.error('⚠️ Error closing Electron app:', error)
      }

      try {
        console.log('🛑 Stopping backend server...')
        await stopBackendServer()
        console.log('✅ Backend server stopped')
      } catch (error) {
        console.error('⚠️ Error stopping backend server:', error)
      }

      console.log('✅ Cleanup completed')
    }
  })
})
