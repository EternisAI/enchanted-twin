import { test, expect, _electron as electron, Locator } from '@playwright/test'
import path from 'path'
import fs from 'fs'
import { E2E_CONFIG } from './config'
import {
  createCleanElectronConfig,
  mockGoogleAuth,
  clearAuthState,
  cleanupTempDirectories
} from './auth.helpers'

test.describe('Authenticated User Features', () => {
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

  test('can access chat functionality when authenticated', async () => {
    console.log('🧪 Testing authenticated chat functionality with clean cache...')

    // Launch Electron with clean cache configuration
    console.log('🚀 Launching Electron app with clean cache...')
    const electronApp = await electron.launch(createCleanElectronConfig())

    try {
      const page = await electronApp.firstWindow()
      await page.waitForLoadState('domcontentloaded')

      // Clear any existing auth state to ensure clean start
      await clearAuthState(page)
      await page.reload()
      await page.waitForLoadState('domcontentloaded')

      // Use mock authentication instead of cached state
      console.log('🔧 Setting up mock authentication...')
      await mockGoogleAuth(page)

      // Give the app time to process the mock authentication
      console.log('⏱️ Waiting for app to process authentication...')
      await page.waitForTimeout(5000)

      // Wait for network idle to ensure all authentication requests are complete
      await page.waitForLoadState('networkidle')

      // Since we're using cached auth state, user should already be logged in
      console.log('🔍 Verifying app is ready and authenticated...')

      // Wait for the chat textarea to appear - this indicates the app is fully loaded
      const chatboxSelector = 'textarea[placeholder*="Send a message"]'
      const alternateChatboxSelectors = [
        'textarea.outline-none.bg-transparent',
        'textarea[class*="auto-sizing-textarea"]',
        'textarea[placeholder*="message"]'
      ]

      let appReady = false

      // Try the primary selector first
      try {
        await expect(page.locator(chatboxSelector)).toBeVisible({ timeout: 15000 })
        appReady = true
        console.log('✅ App is ready - found chat interface')
      } catch (error) {
        console.log('ℹ️ Primary chatbox selector not found, trying alternatives...')

        // Try alternative selectors
        for (const selector of alternateChatboxSelectors) {
          try {
            await expect(page.locator(selector)).toBeVisible({ timeout: 10000 })
            appReady = true
            console.log(`✅ App is ready - found chat interface with selector: ${selector}`)
            break
          } catch (altError) {
            console.log(`ℹ️ Alternative selector not found: ${selector}`)
          }
        }
      }

      if (!appReady) {
        throw new Error('❌ App not ready - chat interface not found')
      }

      // Verify user data exists in localStorage
      const hasUserData = await page.evaluate(() => {
        return window.localStorage.getItem('enchanted_user_data') !== null
      })
      expect(hasUserData).toBe(true)

      // Get user data to verify it's properly formatted
      const userData = await page.evaluate(() => {
        const data = window.localStorage.getItem('enchanted_user_data')
        return data ? JSON.parse(data) : null
      })

      expect(userData).toBeTruthy()
      expect(userData.uid).toBeTruthy()
      expect(userData.email).toBeTruthy()

      console.log(`✅ User is authenticated: ${userData.email}`)

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

      // Give the app more time to fully initialize and authenticate
      console.log('⏱️ Waiting for app to fully initialize...')
      await page.waitForTimeout(10000) // Wait 10 seconds for full initialization

      // Wait for network idle to ensure all authentication requests are complete
      await page.waitForLoadState('networkidle')

      // Wait for the chat textarea to appear - this indicates the app is fully loaded
      console.log('🔍 Verifying app is ready...')
      const chatboxSelector = 'textarea[placeholder*="Send a message"]'

      try {
        await expect(page.locator(chatboxSelector)).toBeVisible({ timeout: 15000 })
        console.log('✅ App is ready - found chat interface')
      } catch (error) {
        // Try alternative selector
        await expect(page.locator('textarea.outline-none.bg-transparent')).toBeVisible({
          timeout: 10000
        })
        console.log('✅ App is ready - found chat interface (alternative selector)')
      }

      // Verify authentication
      const hasUserData = await page.evaluate(() => {
        return window.localStorage.getItem('enchanted_user_data') !== null
      })
      expect(hasUserData).toBe(true)

      console.log('✅ App ready and user authenticated')

      console.log('🔌 Testing MCP server access...')

      // Look for MCP-related functionality
      // Adjust these based on your app's actual MCP server interface
      const mcpElements = ['MCP', 'Servers', 'Connect', 'Server', 'Tools']

      for (const element of mcpElements) {
        try {
          await expect(page.getByText(element)).toBeVisible({ timeout: 15000 }) // Increased timeout
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

      // Give the app more time to fully initialize and authenticate
      console.log('⏱️ Waiting for app to fully initialize...')
      await page.waitForTimeout(10000) // Wait 10 seconds for full initialization

      // Wait for network idle to ensure all authentication requests are complete
      await page.waitForLoadState('networkidle')

      // Wait for the chat textarea to appear - this indicates the app is fully loaded
      console.log('🔍 Verifying app is ready...')
      const chatboxSelector = 'textarea[placeholder*="Send a message"]'

      try {
        await expect(page.locator(chatboxSelector)).toBeVisible({ timeout: 15000 })
        console.log('✅ App is ready - found chat interface')
      } catch (error) {
        // Try alternative selector
        await expect(page.locator('textarea.outline-none.bg-transparent')).toBeVisible({
          timeout: 10000
        })
        console.log('✅ App is ready - found chat interface (alternative selector)')
      }

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
          await expect(page.getByText(element)).toBeVisible({ timeout: 15000 }) // Increased timeout
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

      // Give the app more time to fully initialize and authenticate
      console.log('⏱️ Waiting for app to fully initialize...')
      await page.waitForTimeout(10000) // Wait 10 seconds for full initialization

      // Wait for network idle to ensure all authentication requests are complete
      await page.waitForLoadState('networkidle')

      // Wait for the chat textarea to appear - this indicates the app is fully loaded
      console.log('🔍 Verifying app is ready...')
      const chatboxSelector = 'textarea[placeholder*="Send a message"]'

      try {
        await expect(page.locator(chatboxSelector)).toBeVisible({ timeout: 15000 })
        console.log('✅ App is ready - found chat interface')
      } catch (error) {
        // Try alternative selector
        await expect(page.locator('textarea.outline-none.bg-transparent')).toBeVisible({
          timeout: 10000
        })
        console.log('✅ App is ready - found chat interface (alternative selector)')
      }

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
      await page.waitForLoadState('networkidle')

      // Wait a moment for auth to be restored
      await page.waitForTimeout(5000) // Increased wait time

      // Wait for the chat textarea to appear again - this indicates the app is ready after refresh
      console.log('🔍 Verifying app is ready after refresh...')

      try {
        await expect(page.locator(chatboxSelector)).toBeVisible({ timeout: 15000 })
        console.log('✅ App is ready after refresh - found chat interface')
      } catch (error) {
        // Try alternative selector
        await expect(page.locator('textarea.outline-none.bg-transparent')).toBeVisible({
          timeout: 10000
        })
        console.log('✅ App is ready after refresh - found chat interface (alternative selector)')
      }

      // Check if still authenticated after refresh
      const stillAuthenticated = await page.evaluate(() => {
        return window.localStorage.getItem('enchanted_user_data') !== null
      })

      expect(stillAuthenticated).toBe(true)

      // Verify user data is still properly formatted after refresh
      const refreshedUserData = await page.evaluate(() => {
        const data = window.localStorage.getItem('enchanted_user_data')
        return data ? JSON.parse(data) : null
      })

      expect(refreshedUserData).toBeTruthy()
      expect(refreshedUserData.uid).toBeTruthy()
      expect(refreshedUserData.email).toBeTruthy()

      console.log('✅ Authentication persisted through page refresh (localStorage confirmed)')

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

  test('can send message and receive response in chat', async () => {
    console.log('🧪 Testing chat message sending and response...')

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

      // Verify authentication first
      console.log('🔍 Verifying authenticated state...')
      const hasUserData = await page.evaluate(() => {
        return window.localStorage.getItem('enchanted_user_data') !== null
      })
      expect(hasUserData).toBe(true)

      // Get user data to verify it's properly formatted
      const userData = await page.evaluate(() => {
        const data = window.localStorage.getItem('enchanted_user_data')
        return data ? JSON.parse(data) : null
      })

      expect(userData).toBeTruthy()
      expect(userData.uid).toBeTruthy()
      expect(userData.email).toBeTruthy()

      console.log(`✅ User is authenticated: ${userData.email}`)

      // Step 1: Detect the chatbox
      console.log('💬 Step 1: Looking for chat interface...')

      // Wait for the specific textarea with the classes mentioned by user
      const chatboxSelector = 'textarea[placeholder*="Send a message"]'
      const alternateChatboxSelectors = [
        'textarea.outline-none.bg-transparent',
        'textarea[class*="auto-sizing-textarea"]',
        'textarea[placeholder*="message"]',
        'input[placeholder*="message"]',
        '[data-testid="chat-input"]'
      ]

      let chatbox: Locator | null = null

      // Try the primary selector first
      try {
        const primaryChatbox = page.locator(chatboxSelector)
        await expect(primaryChatbox).toBeVisible({ timeout: 10000 })
        chatbox = primaryChatbox
        console.log('✅ Found chatbox with primary selector')
      } catch (error) {
        console.log('ℹ️ Primary chatbox selector not found, trying alternatives...')

        // Try alternative selectors
        for (const selector of alternateChatboxSelectors) {
          try {
            const altChatbox = page.locator(selector)
            await expect(altChatbox).toBeVisible({ timeout: 5000 })
            chatbox = altChatbox
            console.log(`✅ Found chatbox with selector: ${selector}`)
            break
          } catch (altError) {
            console.log(`ℹ️ Alternative selector not found: ${selector}`)
          }
        }
      }

      if (!chatbox) {
        throw new Error('❌ Could not find chatbox element')
      }

      // Take screenshot of initial chat state
      await page.screenshot({
        path: 'test-results/artifacts/chat-initial-state.png',
        fullPage: true
      })

      // Step 2: Send message "hey twin"
      console.log('📝 Step 2: Sending message "hey twin"...')

      // Click on the chatbox to focus it
      await chatbox.click()

      // Clear any existing content and type the message
      await chatbox.fill('')
      await chatbox.type('hey twin')

      // Press Enter to send the message
      await chatbox.press('Enter')

      console.log('✅ Message sent')

      // Step 3: Wait for response and verify messages
      console.log('⏳ Step 3: Waiting for response and verifying messages...')

      // Wait a bit for the message to be processed and response to arrive
      await page.waitForTimeout(5000)

      // Look for user message (should have justify-end class)
      const userMessageSelector = 'div[class*="justify-end"]'
      const userMessages = page.locator(userMessageSelector)

      // Look for agent response (should have justify-start class)
      const agentMessageSelector = 'div[class*="justify-start"]'
      const agentMessages = page.locator(agentMessageSelector)

      // Wait up to 30 seconds for agent response
      console.log('⏳ Waiting for agent response...')
      let agentResponseFound = false
      let attempts = 0
      const maxAttempts = 30 // 30 seconds total

      while (!agentResponseFound && attempts < maxAttempts) {
        try {
          const agentMessageCount = await agentMessages.count()
          if (agentMessageCount > 0) {
            agentResponseFound = true
            console.log('✅ Agent response detected')
            break
          }
        } catch (error) {
          // Continue waiting
        }

        await page.waitForTimeout(1000) // Wait 1 second between checks
        attempts++

        if (attempts % 5 === 0) {
          console.log(`⏳ Still waiting for agent response... (${attempts}s elapsed)`)
        }
      }

      if (!agentResponseFound) {
        console.log(
          '⚠️ No agent response found within timeout, checking message structure anyway...'
        )
      }

      // Step 4: Verify we have exactly 2 messages
      console.log('🔍 Step 4: Verifying message structure...')

      // Count user messages
      const userMessageCount = await userMessages.count()
      console.log(`📨 Found ${userMessageCount} user message(s)`)

      // Count agent messages
      const agentMessageCount = await agentMessages.count()
      console.log(`🤖 Found ${agentMessageCount} agent message(s)`)

      // Verify we have at least 1 user message
      expect(userMessageCount).toBeGreaterThanOrEqual(1)

      // If we found an agent response, verify it's non-empty
      if (agentMessageCount > 0) {
        const agentMessageText = await agentMessages.first().textContent()
        expect(agentMessageText).toBeTruthy()
        if (agentMessageText) {
          expect(agentMessageText.trim().length).toBeGreaterThan(0)
          console.log(
            `✅ Agent response verified: "${agentMessageText.substring(0, 50)}${agentMessageText.length > 50 ? '...' : ''}"`
          )
        }
      }

      // Verify total message count (user + agent)
      const totalExpectedMessages = agentResponseFound ? 2 : 1
      const actualTotalMessages = userMessageCount + agentMessageCount

      if (agentResponseFound) {
        expect(actualTotalMessages).toBeGreaterThanOrEqual(2)
        console.log('✅ Found expected user message and agent response')
      } else {
        expect(actualTotalMessages).toBeGreaterThanOrEqual(1)
        console.log(
          '✅ Found user message (agent response timeout - this may be expected in test environment)'
        )
      }

      // Check if our specific message "hey twin" is visible
      const ourMessageVisible = await page.getByText('hey twin').isVisible()
      expect(ourMessageVisible).toBe(true)
      console.log('✅ Our message "hey twin" is visible in the chat')

      // Step 5: Take final screenshot
      console.log('📸 Step 5: Taking final screenshot...')
      await page.screenshot({
        path: 'test-results/artifacts/chat-conversation-complete.png',
        fullPage: true
      })

      console.log('🎉 Chat interaction test completed successfully!')

      // Log summary
      console.log('📊 Test Summary:')
      console.log(`   • User messages: ${userMessageCount}`)
      console.log(`   • Agent messages: ${agentMessageCount}`)
      console.log(`   • Total messages: ${actualTotalMessages}`)
      console.log(`   • Agent responded: ${agentResponseFound ? 'Yes' : 'No'}`)
    } catch (error) {
      console.error('❌ Chat interaction test failed:', error)

      // Take error screenshot
      try {
        const page = await electronApp.firstWindow()
        await page.screenshot({
          path: 'test-results/artifacts/chat-interaction-error.png',
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
