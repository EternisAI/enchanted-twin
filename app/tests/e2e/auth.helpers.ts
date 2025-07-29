import { Page, expect } from '@playwright/test'
import { GOOGLE_TEST_CREDENTIALS, AUTH_CONFIG } from './config'

/**
 * Signs in with Google using the test credentials
 * This function handles the complete OAuth flow through Google's login pages
 */
export async function signInWithGoogle(page: Page): Promise<void> {
  console.log('🔐 Starting Google sign-in flow...')

  try {
    // For Electron apps, we don't need to navigate - the page is already loaded
    // Just wait for the page to be ready
    await page.waitForLoadState('domcontentloaded')

    // Wait for the Google sign-in button to appear
    console.log('⏳ Waiting for Google sign-in button...')
    await page.waitForSelector('text=Continue with Google', {
      timeout: AUTH_CONFIG.LOGIN_TIMEOUT
    })

    // Take screenshot before clicking
    await page.screenshot({
      path: 'test-results/artifacts/auth-before-google-click.png',
      fullPage: true
    })

    // Click the Google sign-in button
    console.log('👆 Clicking Google sign-in button...')
    await page.getByText('Continue with Google').click()

    // Wait for either Google's login page OR immediate redirect if already authenticated
    console.log('⏳ Waiting for Google authentication page or redirect...')

    // Wait a moment to see if we're redirected immediately (cached auth)
    await page.waitForTimeout(3000)

    const currentUrl = page.url()
    console.log('📍 Current URL after Google click:', currentUrl)

    // If we're already back at the app (cached authentication), we're done
    if (!currentUrl.includes('accounts.google.com')) {
      console.log(
        '✅ Already authenticated or redirected back to app, checking for success indicators...'
      )
      await waitForSuccessfulAuth(page)
      return
    }

    // Handle Google's login form
    if (currentUrl.includes('accounts.google.com')) {
      console.log('📝 Filling Google login form...')

      // Handle email input
      const emailInput = page.locator('input[type="email"]').first()
      await emailInput.waitFor({ state: 'visible', timeout: AUTH_CONFIG.LOGIN_TIMEOUT })

      await emailInput.fill(GOOGLE_TEST_CREDENTIALS.EMAIL)
      console.log('✏️ Email filled')

      // Click Next button
      await page.getByRole('button', { name: 'Next' }).click()

      // Wait for password field and fill it
      console.log('⏳ Waiting for password field...')
      const passwordInput = page.locator('input[type="password"]').first()
      await passwordInput.waitFor({ state: 'visible', timeout: AUTH_CONFIG.LOGIN_TIMEOUT })

      await passwordInput.fill(GOOGLE_TEST_CREDENTIALS.PASSWORD)
      console.log('✏️ Password filled')

      // Submit password
      await page.getByRole('button', { name: 'Next' }).click()

      // Handle potential 2FA or additional security screens
      await page.waitForTimeout(2000)

      // Check if we need to handle additional screens (like "Continue" or "Allow")
      const continueButton = page.getByRole('button', { name: 'Continue' })
      if (await continueButton.isVisible({ timeout: 5000 })) {
        console.log('👆 Clicking Continue button...')
        await continueButton.click()
      }

      const allowButton = page.getByRole('button', { name: 'Allow' })
      if (await allowButton.isVisible({ timeout: 5000 })) {
        console.log('👆 Clicking Allow button...')
        await allowButton.click()
      }

      // Wait for redirect back to app
      console.log('⏳ Waiting for redirect back to app...')
      await page.waitForTimeout(5000) // Give some time for redirect
    }

    // Wait for successful authentication
    await waitForSuccessfulAuth(page)
  } catch (error) {
    console.error('❌ Google sign-in failed:', error)

    // Take screenshot of error state
    await page.screenshot({
      path: 'test-results/artifacts/auth-error-state.png',
      fullPage: true
    })

    throw error
  }
}

/**
 * Waits for successful authentication by checking for expected UI elements
 */
async function waitForSuccessfulAuth(page: Page): Promise<void> {
  console.log('⏳ Waiting for authentication success indicators...')

  try {
    // Wait for user data to be stored in localStorage
    await page.waitForFunction(
      () => {
        return window.localStorage.getItem('enchanted_user_data') !== null
      },
      { timeout: AUTH_CONFIG.LOGIN_TIMEOUT }
    )

    // Wait for welcome message or user avatar to appear
    // Use a more flexible approach for different possible welcome messages
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
        // Continue to next selector
        console.log(`ℹ️ Welcome selector not found: ${selector}`)
      }
    }

    if (!welcomeFound) {
      console.log('⚠️ No specific welcome message found, but user data exists in localStorage')
    }

    console.log('✅ Google sign-in completed successfully!')

    // Take success screenshot
    await page.screenshot({
      path: 'test-results/artifacts/auth-success-state.png',
      fullPage: true
    })
  } catch (error) {
    console.error('❌ Failed to verify authentication success:', error)
    console.log('🔍 Current page content for debugging:')

    // Log current page state for debugging
    try {
      const bodyText = await page.locator('body').textContent()
      console.log('📄 Page body text (first 500 chars):', bodyText?.substring(0, 500))

      const userDataExists = await page.evaluate(() => {
        return window.localStorage.getItem('enchanted_user_data') !== null
      })
      console.log('💾 User data in localStorage:', userDataExists)
    } catch (debugError) {
      console.error('❌ Could not get debug info:', debugError)
    }

    throw error
  }
}

/**
 * Signs out from the application
 */
export async function signOut(page: Page): Promise<void> {
  console.log('🚪 Starting sign out flow...')

  try {
    // For Electron, we don't need to navigate - just wait for page to be ready
    await page.waitForLoadState('domcontentloaded')

    // Look for sign out button/link - this might be in a menu or direct link
    const signOutButton = page.getByText('Log out').first()
    const isVisible = await signOutButton.isVisible({ timeout: 5000 })

    if (isVisible) {
      await signOutButton.click()
      console.log('👆 Clicked sign out button')
    } else {
      // Try alternative selectors if main one doesn't work
      const altSignOut = page.getByRole('button', { name: 'Sign out' })
      if (await altSignOut.isVisible({ timeout: 5000 })) {
        await altSignOut.click()
        console.log('👆 Clicked alternative sign out button')
      } else {
        console.log('⚠️ No sign out button found, user might already be signed out')
      }
    }

    // Wait for redirect to login screen
    await expect(page.getByText('Continue with Google')).toBeVisible({
      timeout: AUTH_CONFIG.LOGIN_TIMEOUT
    })

    console.log('✅ Sign out completed successfully')

    // Take screenshot of signed out state
    await page.screenshot({
      path: 'test-results/artifacts/auth-signout-success.png',
      fullPage: true
    })
  } catch (error) {
    console.error('❌ Sign out failed:', error)
    await page.screenshot({
      path: 'test-results/artifacts/auth-signout-error.png',
      fullPage: true
    })
    throw error
  }
}

/**
 * Waits for authentication callback to complete
 * Useful when the app processes OAuth callbacks
 */
export async function waitForAuthCallback(page: Page): Promise<void> {
  console.log('⏳ Waiting for authentication callback to complete...')

  // Wait for the authentication data to be stored
  await page.waitForFunction(
    () => {
      const userData = window.localStorage.getItem('enchanted_user_data')
      return userData !== null && userData !== 'undefined'
    },
    { timeout: AUTH_CONFIG.OAUTH_TIMEOUT }
  )

  console.log('✅ Authentication callback completed')
}

/**
 * Checks if user is currently authenticated
 */
export async function isAuthenticated(page: Page): Promise<boolean> {
  try {
    // Check localStorage for user data
    const hasUserData = await page.evaluate(() => {
      return window.localStorage.getItem('enchanted_user_data') !== null
    })

    if (!hasUserData) {
      return false
    }

    // Also check if welcome message is visible (but don't fail if not found)
    try {
      const hasWelcomeMessage = await page.getByText('Welcome').isVisible({ timeout: 2000 })
      return hasUserData && hasWelcomeMessage
    } catch (error) {
      // If welcome message check fails, just return based on localStorage
      return hasUserData
    }
  } catch (error) {
    return false
  }
}

/**
 * Clears authentication state (useful for test cleanup)
 */
export async function clearAuthState(page: Page): Promise<void> {
  console.log('🧹 Clearing authentication state...')

  await page.evaluate(() => {
    window.localStorage.removeItem('enchanted_user_data')
    window.localStorage.removeItem('enchanted_has_auto_connected')
  })

  console.log('✅ Authentication state cleared')
}
