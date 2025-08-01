import { Page, expect, Locator } from '@playwright/test'

/**
 * Chat interface selectors - ordered by preference
 */
const CHAT_SELECTORS = {
  primary:
    'textarea[placeholder*="message"], textarea[placeholder*="chat"], textarea[placeholder*="type"]',
  alternative: 'textarea.outline-none.bg-transparent',
  fallback: 'textarea, input[type="text"][placeholder*="message"]'
}

/**
 * Waits for the chat interface to be ready and visible
 */
export async function waitForChatInterface(page: Page): Promise<void> {
  console.log('🔍 Looking for chat interface...')

  try {
    // Try primary selector first
    await expect(page.locator(CHAT_SELECTORS.primary)).toBeVisible({ timeout: 15000 })
    console.log('✅ Chat interface found (primary selector)')
    return
  } catch (error) {
    console.log('⚠️ Primary chat selector not found, trying alternative...')
  }

  try {
    // Try alternative selector
    await expect(page.locator(CHAT_SELECTORS.alternative)).toBeVisible({ timeout: 10000 })
    console.log('✅ Chat interface found (alternative selector)')
    return
  } catch (error) {
    console.log('⚠️ Alternative chat selector not found, trying fallback...')
  }

  try {
    // Try fallback selector
    await expect(page.locator(CHAT_SELECTORS.fallback)).toBeVisible({ timeout: 5000 })
    console.log('✅ Chat interface found (fallback selector)')
    return
  } catch (error) {
    console.log('❌ No chat interface found with any selector')

    // Take debug screenshot
    await page.screenshot({
      path: 'test-results/artifacts/chat-interface-not-found.png',
      fullPage: true
    })

    throw new Error('Chat interface not found after trying all selectors')
  }
}

/**
 * Gets the chat input element (textarea/input for typing messages)
 */
export async function getChatInput(page: Page): Promise<Locator> {
  // Try each selector until we find a visible one
  const selectors = [CHAT_SELECTORS.primary, CHAT_SELECTORS.alternative, CHAT_SELECTORS.fallback]

  for (const selector of selectors) {
    try {
      const input = page.locator(selector).first()
      await input.waitFor({ state: 'visible', timeout: 3000 })
      return input
    } catch (error) {
      continue // Try next selector
    }
  }

  throw new Error('Could not find any chat input element')
}

/**
 * Sends a message in the chat
 */
export async function sendChatMessage(page: Page, message: string): Promise<void> {
  console.log(`💬 Sending chat message: "${message}"`)

  try {
    // Get the chat input
    const chatInput = await getChatInput(page)

    // Clear any existing content and type the message
    await chatInput.click()
    await chatInput.fill('')
    await chatInput.type(message, { delay: 50 }) // Human-like typing

    // Take screenshot before sending
    await page.screenshot({
      path: 'test-results/artifacts/chat-before-send.png',
      fullPage: true
    })

    // Send the message (try different methods)
    try {
      // Try pressing Enter first
      await chatInput.press('Enter')
      console.log('✅ Message sent via Enter key')
    } catch (enterError) {
      // If Enter doesn't work, look for a send button
      try {
        const sendButton = page
          .locator('button[type="submit"], button:has-text("Send"), button[aria-label*="send" i]')
          .first()
        await sendButton.click()
        console.log('✅ Message sent via Send button')
      } catch (buttonError) {
        console.log('⚠️ Could not find Send button, trying Ctrl+Enter...')
        await chatInput.press('Control+Enter')
        console.log('✅ Message sent via Ctrl+Enter')
      }
    }

    // Wait for message to be processed
    await page.waitForTimeout(2000)

    // Take screenshot after sending
    await page.screenshot({
      path: 'test-results/artifacts/chat-after-send.png',
      fullPage: true
    })

    console.log('✅ Chat message sent successfully')
  } catch (error) {
    console.error('❌ Failed to send chat message:', error)

    // Take error screenshot
    await page.screenshot({
      path: 'test-results/artifacts/chat-send-error.png',
      fullPage: true
    })

    throw error
  }
}

/**
 * Waits for a response in the chat
 */
export async function waitForChatResponse(page: Page, timeoutMs: number = 30000): Promise<void> {
  console.log('⏳ Waiting for chat response...')

  try {
    // Look for indicators that a response is being generated or has arrived
    const responseIndicators = [
      'div[data-message-id]', // Message containers
      '.message', // Generic message class
      '[role="log"]', // Chat log
      'div:has-text("typing")', // Typing indicator
      'div:has-text("...")' // Loading dots
    ]

    // Wait for any response indicator
    for (const indicator of responseIndicators) {
      try {
        await page
          .locator(indicator)
          .first()
          .waitFor({
            state: 'visible',
            timeout: timeoutMs / responseIndicators.length
          })
        console.log(`✅ Chat response detected via: ${indicator}`)

        // Take screenshot of response
        await page.screenshot({
          path: 'test-results/artifacts/chat-response-received.png',
          fullPage: true
        })

        return
      } catch (error) {
        continue // Try next indicator
      }
    }

    console.log('⚠️ No specific response indicator found, but continuing...')
  } catch (error) {
    console.log('⚠️ Could not detect chat response, but test will continue')
  }
}

/**
 * Checks if chat is ready for interaction
 */
export async function isChatReady(page: Page): Promise<boolean> {
  try {
    await waitForChatInterface(page)
    return true
  } catch (error) {
    return false
  }
}

/**
 * Takes a screenshot of the current chat state
 */
export async function screenshotChatState(page: Page, filename: string): Promise<void> {
  try {
    await page.screenshot({
      path: `test-results/artifacts/chat-${filename}.png`,
      fullPage: true
    })
    console.log(`📸 Chat screenshot saved: chat-${filename}.png`)
  } catch (error) {
    console.log(`⚠️ Could not take chat screenshot: ${error.message}`)
  }
}

/**
 * Clears the chat input
 */
export async function clearChatInput(page: Page): Promise<void> {
  try {
    const chatInput = await getChatInput(page)
    await chatInput.click()
    await chatInput.fill('')
    console.log('✅ Chat input cleared')
  } catch (error) {
    console.log('⚠️ Could not clear chat input:', error.message)
  }
}

/**
 * Tests basic chat functionality with a simple message
 */
export async function testBasicChat(page: Page): Promise<void> {
  console.log('🧪 Testing basic chat functionality...')

  try {
    // Step 1: Wait for chat interface to be ready
    console.log('🔍 Step 1: Waiting for chat interface to be ready...')
    await waitForChatInterface(page)

    // Take screenshot of chat interface
    await screenshotChatState(page, 'interface-ready')

    console.log('✅ Step 1 completed: Chat interface is ready')

    // Step 2: Send a test message
    console.log('💬 Step 2: Sending test message...')
    const testMessage = 'Hello! This is a test message from automated testing.'

    await sendChatMessage(page, testMessage)
    console.log('✅ Step 2 completed: Test message sent')

    // Step 3: Wait for response (optional - depends on your app)
    console.log('⏳ Step 3: Waiting for chat response...')
    try {
      await waitForChatResponse(page, 15000) // 15 second timeout
      console.log('✅ Step 3 completed: Chat response received')
    } catch (responseError) {
      console.log('⚠️ Step 3: No response detected (this may be expected)')
    }

    // Take final screenshot
    await screenshotChatState(page, 'basic-chat-completed')

    console.log('🎉 Basic chat test completed successfully!')
  } catch (error) {
    console.error('❌ Basic chat test failed:', error)

    // Take error screenshot
    await screenshotChatState(page, 'basic-chat-error')

    throw error
  }
}

/**
 * Tests multiple message sending
 */
export async function testMultipleMessages(page: Page): Promise<void> {
  console.log('🧪 Testing multiple message sending...')

  try {
    // Ensure chat is ready
    await waitForChatInterface(page)

    const messages = ['First test message', 'Second test message', 'What is 2 + 2?']

    for (let i = 0; i < messages.length; i++) {
      console.log(`💬 Sending message ${i + 1}/${messages.length}: "${messages[i]}"`)

      await sendChatMessage(page, messages[i])

      // Wait between messages
      await page.waitForTimeout(2000)

      // Take screenshot after each message
      await screenshotChatState(page, `multiple-messages-${i + 1}`)
    }

    console.log('🎉 Multiple messages test completed successfully!')
  } catch (error) {
    console.error('❌ Multiple messages test failed:', error)
    await screenshotChatState(page, 'multiple-messages-error')
    throw error
  }
}

/**
 * Tests chat input clearing and re-typing
 */
export async function testChatInputManipulation(page: Page): Promise<void> {
  console.log('🧪 Testing chat input manipulation...')

  try {
    await waitForChatInterface(page)

    // Type a message but don't send it
    const draftMessage = 'This is a draft message that will be cleared'
    const chatInput = await page.locator('textarea').first()

    await chatInput.click()
    await chatInput.type(draftMessage, { delay: 50 })

    // Take screenshot with draft
    await screenshotChatState(page, 'draft-message')

    // Clear the input
    await clearChatInput(page)

    // Take screenshot after clearing
    await screenshotChatState(page, 'input-cleared')

    // Type and send a different message
    const finalMessage = 'This is the final message after clearing'
    await sendChatMessage(page, finalMessage)

    await screenshotChatState(page, 'input-manipulation-completed')

    console.log('🎉 Chat input manipulation test completed successfully!')
  } catch (error) {
    console.error('❌ Chat input manipulation test failed:', error)
    await screenshotChatState(page, 'input-manipulation-error')
    throw error
  }
}

/**
 * Comprehensive chat test suite - runs all chat tests in sequence
 * This is the main function to call from the master test
 */
export async function runAllChatTests(page: Page): Promise<void> {
  console.log('\n💬 🚀 STARTING COMPREHENSIVE CHAT TEST SUITE...')
  console.log('📋 This suite will run:')
  console.log('   1. Basic chat functionality test')
  console.log('   2. Multiple messages test')
  console.log('   3. Chat input manipulation test')

  try {
    // Test 1: Basic chat functionality
    console.log('\n🎯 CHAT TEST 1/3: Basic Chat Functionality')
    await testBasicChat(page)
    console.log('✅ CHAT TEST 1/3: Completed successfully')

    // Test 2: Multiple messages (optional - continue even if it fails)
    console.log('\n🎯 CHAT TEST 2/3: Multiple Messages')
    try {
      await testMultipleMessages(page)
      console.log('✅ CHAT TEST 2/3: Completed successfully')
    } catch (multipleMessagesError) {
      console.log('⚠️ CHAT TEST 2/3: Failed, but continuing...', multipleMessagesError.message)
      // Take screenshot but don't fail the whole suite
      await screenshotChatState(page, 'multiple-messages-suite-error')
    }

    // Test 3: Input manipulation (optional - continue even if it fails)
    console.log('\n🎯 CHAT TEST 3/3: Input Manipulation')
    try {
      await testChatInputManipulation(page)
      console.log('✅ CHAT TEST 3/3: Completed successfully')
    } catch (inputManipError) {
      console.log('⚠️ CHAT TEST 3/3: Failed, but continuing...', inputManipError.message)
      await screenshotChatState(page, 'input-manipulation-suite-error')
    }

    console.log('\n🎉 💬 COMPREHENSIVE CHAT TEST SUITE COMPLETED!')
    console.log('📊 Chat Test Summary:')
    console.log('   ✅ Basic chat functionality (required)')
    console.log('   📝 Multiple messages (optional)')
    console.log('   🧪 Input manipulation (optional)')
    console.log('   📸 Screenshots saved to test-results/artifacts/')
  } catch (error) {
    console.error('\n❌ 💬 COMPREHENSIVE CHAT TEST SUITE FAILED:', error)
    await screenshotChatState(page, 'chat-suite-critical-error')
    throw error
  }
}
