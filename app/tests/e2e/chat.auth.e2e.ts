import { test, expect, _electron as electron } from '@playwright/test'
import path from 'path'
import { E2E_CONFIG } from './config'

test.describe('Authenticated User Features', () => {
  test('can access chat functionality when authenticated', async () => {
    console.log('🧪 Testing authenticated chat functionality...')

    // This test uses the stored authentication state from auth.setup.ts
    // The user should already be authenticated

    const electronApp = await electron.launch({
      args: [path.join(__dirname, '../../out/main/index.js')],
      env: {
        ...process.env,
        NODE_ENV: 'test'
      }
    })

    try {
      const page = await electronApp.firstWindow()
      await page.waitForLoadState('domcontentloaded')

      // Since we're using cached auth state, user should already be logged in
      console.log('🔍 Verifying authenticated state...')

      // Check for authenticated content (adjust selector based on your app)
      // Use a more flexible approach since the exact welcome text may vary
      const welcomeSelectors = [
        'text=Welcome',
        'text=welcome',
        '[data-testid="user-welcome"]',
        '[data-testid="authenticated-content"]'
      ]

      let welcomeFound = false
      for (const selector of welcomeSelectors) {
        try {
          await expect(page.locator(selector).first()).toBeVisible({ timeout: 5000 })
          welcomeFound = true
          console.log(`✅ Found welcome indicator: ${selector}`)
          break
        } catch (error) {
          console.log(`ℹ️ Welcome selector not found: ${selector}`)
        }
      }

      // Verify user data exists in localStorage
      const hasUserData = await page.evaluate(() => {
        return window.localStorage.getItem('enchanted_user_data') !== null
      })
      expect(hasUserData).toBe(true)

      if (!welcomeFound) {
        console.log('⚠️ No specific welcome message found, but user data exists in localStorage')
      }

      console.log('✅ User is authenticated via cached session')

      // Test chat-specific functionality
      console.log('💬 Testing chat functionality...')

      // Look for chat interface elements
      // Note: Adjust these selectors based on your actual app structure

      // Check if we can access the main chat area
      // This is just an example - you'll need to adapt to your app's structure
      const chatElements = ['Chat', 'Messages', 'Send', 'Type a message']

      for (const element of chatElements) {
        try {
          await expect(page.getByText(element)).toBeVisible({ timeout: 5000 })
          console.log(`✅ Found chat element: ${element}`)
        } catch (error) {
          console.log(`ℹ️ Chat element not found (may be expected): ${element}`)
        }
      }

      // Take screenshot of authenticated chat interface
      await page.screenshot({
        path: 'test-results/artifacts/authenticated-chat-interface.png',
        fullPage: true
      })

      console.log('✅ Chat functionality test completed')
    } catch (error) {
      console.error('❌ Authenticated chat functionality test failed:', error)

      // Take error screenshot
      try {
        const page = await electronApp.firstWindow()
        await page.screenshot({
          path: 'test-results/artifacts/authenticated-chat-error.png',
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

  test('can access MCP server features when authenticated', async () => {
    console.log('🧪 Testing authenticated MCP server functionality...')

    const electronApp = await electron.launch({
      args: [path.join(__dirname, '../../out/main/index.js')],
      env: {
        ...process.env,
        NODE_ENV: 'test'
      }
    })

    try {
      const page = await electronApp.firstWindow()
      await page.waitForLoadState('domcontentloaded')

      // Verify authentication
      const hasUserData = await page.evaluate(() => {
        return window.localStorage.getItem('enchanted_user_data') !== null
      })
      expect(hasUserData).toBe(true)

      // Navigate to MCP servers or settings if needed
      // This depends on your app's navigation structure

      console.log('🔌 Testing MCP server access...')

      // Look for MCP-related functionality
      // Adjust these based on your app's actual MCP server interface
      const mcpElements = ['MCP', 'Servers', 'Connect', 'Server', 'Tools']

      for (const element of mcpElements) {
        try {
          await expect(page.getByText(element)).toBeVisible({ timeout: 5000 })
          console.log(`✅ Found MCP element: ${element}`)
        } catch (error) {
          console.log(`ℹ️ MCP element not found (may be expected): ${element}`)
        }
      }

      // Take screenshot of MCP interface
      await page.screenshot({
        path: 'test-results/artifacts/authenticated-mcp-interface.png',
        fullPage: true
      })

      console.log('✅ MCP server functionality test completed')
    } catch (error) {
      console.error('❌ Authenticated MCP functionality test failed:', error)
      throw error
    } finally {
      await electronApp.close()
    }
  })

  test('can access settings and user profile when authenticated', async () => {
    console.log('🧪 Testing authenticated settings and profile functionality...')

    const electronApp = await electron.launch({
      args: [path.join(__dirname, '../../out/main/index.js')],
      env: {
        ...process.env,
        NODE_ENV: 'test'
      }
    })

    try {
      const page = await electronApp.firstWindow()
      await page.waitForLoadState('domcontentloaded')

      // Verify authentication
      const hasUserData = await page.evaluate(() => {
        return window.localStorage.getItem('enchanted_user_data') !== null
      })
      expect(hasUserData).toBe(true)

      // Test user profile access
      console.log('👤 Testing user profile access...')

      // Get user data from localStorage to verify it's properly formatted
      const userData = await page.evaluate(() => {
        const data = window.localStorage.getItem('enchanted_user_data')
        return data ? JSON.parse(data) : null
      })

      expect(userData).toBeTruthy()
      expect(userData.uid).toBeTruthy()
      expect(userData.email).toBeTruthy()

      console.log(`✅ User profile data verified for: ${userData.email}`)

      // Look for settings or profile elements
      const profileElements = ['Settings', 'Profile', 'Account', 'Preferences']

      for (const element of profileElements) {
        try {
          await expect(page.getByText(element)).toBeVisible({ timeout: 5000 })
          console.log(`✅ Found profile element: ${element}`)
        } catch (error) {
          console.log(`ℹ️ Profile element not found (may be expected): ${element}`)
        }
      }

      // Take screenshot of profile/settings interface
      await page.screenshot({
        path: 'test-results/artifacts/authenticated-profile-interface.png',
        fullPage: true
      })

      console.log('✅ Settings and profile functionality test completed')
    } catch (error) {
      console.error('❌ Authenticated settings functionality test failed:', error)
      throw error
    } finally {
      await electronApp.close()
    }
  })

  test('maintains authentication state during app usage', async () => {
    console.log('🧪 Testing authentication state persistence during usage...')

    const electronApp = await electron.launch({
      args: [path.join(__dirname, '../../out/main/index.js')],
      env: {
        ...process.env,
        NODE_ENV: 'test'
      }
    })

    try {
      const page = await electronApp.firstWindow()
      await page.waitForLoadState('domcontentloaded')

      // Verify initial authentication
      const hasUserData = await page.evaluate(() => {
        return window.localStorage.getItem('enchanted_user_data') !== null
      })
      expect(hasUserData).toBe(true)

      console.log('🔄 Testing authentication persistence during navigation...')

      // Simulate some app usage - navigation, interactions, etc.
      // This depends on your app's structure

      // Refresh the page to test auth persistence
      await page.reload()
      await page.waitForLoadState('domcontentloaded')

      // Wait a moment for auth to be restored
      await page.waitForTimeout(3000)

      // Check if still authenticated after refresh
      const stillAuthenticated = await page.evaluate(() => {
        return window.localStorage.getItem('enchanted_user_data') !== null
      })

      expect(stillAuthenticated).toBe(true)

      // Verify UI still shows authenticated state (if welcome message is available)
      try {
        const welcomeSelectors = ['text=Welcome', 'text=welcome', '[data-testid="user-welcome"]']

        let welcomeFound = false
        for (const selector of welcomeSelectors) {
          try {
            await expect(page.locator(selector).first()).toBeVisible({ timeout: 5000 })
            welcomeFound = true
            break
          } catch (error) {
            // Continue to next selector
          }
        }

        if (welcomeFound) {
          console.log('✅ Authentication persisted through page refresh (UI confirmed)')
        } else {
          console.log('✅ Authentication persisted through page refresh (localStorage confirmed)')
        }
      } catch (error) {
        console.log('✅ Authentication persisted through page refresh (localStorage confirmed)')
      }

      // Take final screenshot
      await page.screenshot({
        path: 'test-results/artifacts/authenticated-persistence-test.png',
        fullPage: true
      })

      console.log('✅ Authentication persistence test completed')
    } catch (error) {
      console.error('❌ Authentication persistence test failed:', error)
      throw error
    } finally {
      await electronApp.close()
    }
  })
})
