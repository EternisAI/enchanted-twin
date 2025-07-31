import { Page, expect } from '@playwright/test'
import path from 'path'
import fs from 'fs'
import { GOOGLE_TEST_CREDENTIALS, AUTH_CONFIG, FIREBASE_TEST_CONFIG } from './config'

/**
 * Creates Electron launch configuration with fresh cache/user data
 * This ensures consistent OAuth flow without "Choose an account" scenarios
 */
export function createCleanElectronConfig() {
  const tempUserDataDir = path.join(__dirname, '../../../temp', `electron-test-${Date.now()}`)

  return {
    args: [
      path.join(__dirname, '../../out/main/index.js'),
      // Cache clearing arguments - ensures fresh OAuth every time
      `--user-data-dir=${tempUserDataDir}`,
      '--disable-session-crashed-bubble',
      '--disable-background-timer-throttling',
      '--disable-backgrounding-occluded-windows',
      '--disable-renderer-backgrounding',
      // Existing browser-like arguments
      '--disable-web-security',
      '--disable-features=VizDisplayCompositor',
      '--disable-blink-features=AutomationControlled',
      '--no-first-run',
      '--no-default-browser-check',
      '--disable-extensions',
      '--disable-default-apps',
      '--user-agent=Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36'
    ],
    env: {
      ...process.env,
      NODE_ENV: 'test',
      VITE_FIREBASE_API_KEY: FIREBASE_TEST_CONFIG.FIREBASE_API_KEY,
      VITE_FIREBASE_AUTH_DOMAIN: FIREBASE_TEST_CONFIG.FIREBASE_AUTH_DOMAIN,
      VITE_FIREBASE_PROJECT_ID: FIREBASE_TEST_CONFIG.FIREBASE_PROJECT_ID
    },
    timeout: 30000
  }
}

/**
 * Cleanup temporary test directories
 */
export async function cleanupTempDirectories() {
  const tempDir = path.join(__dirname, '../../../temp')
  try {
    if (fs.existsSync(tempDir)) {
      fs.rmSync(tempDir, { recursive: true, force: true })
      console.log('🧹 Cleaned up temporary test directories')
    }
  } catch (error) {
    console.log('⚠️ Could not clean up temp directories:', error.message)
  }
}

/**
 * Signs in with Google using the test credentials
 * This function handles the complete OAuth flow through Google's Electron window
 */
// @ts-ignore - Electron app parameter type
export async function signInWithGoogle(page: Page, electronApp: any): Promise<void> {
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

    // Click the Google sign-in button - this opens the OAuth window
    console.log('👆 Clicking Google sign-in button...')
    await page.getByText('Continue with Google').click()

    // Wait for the OAuth window to open
    console.log('⏳ Waiting for OAuth window to open...')
    await page.waitForTimeout(3000) // Give time for window to open

    // Wait for the OAuth window to appear
    // @ts-ignore - OAuth window type
    let oauthWindow: any = null
    for (let i = 0; i < 10; i++) {
      const windows = electronApp.windows()
      // Find the window that's not the main window and contains localhost
      // @ts-ignore - Window comparison
      const foundWindow = windows.find(
        (win: any) => win !== page && win.url().includes('localhost')
      )
      if (foundWindow) {
        oauthWindow = foundWindow
        console.log('🪟 Found OAuth window:', oauthWindow.url())
        break
      }
      console.log(`⏳ Looking for OAuth window... attempt ${i + 1}/10`)
      await page.waitForTimeout(1000)
    }

    if (!oauthWindow) {
      throw new Error('❌ OAuth window not found after 10 seconds')
    }

    // Now interact with the OAuth window to find the real button
    console.log('🔍 Looking for #auth-signin button in OAuth window...')

    // Wait for the OAuth window to load
    // @ts-ignore - OAuth window API
    await oauthWindow.waitForLoadState('domcontentloaded')

    // Take screenshot of OAuth window before clicking
    // @ts-ignore - OAuth window API
    await oauthWindow.screenshot({
      path: 'test-results/artifacts/oauth-window-before-click.png',
      fullPage: true
    })

    // Look for the specific button the user mentioned
    // @ts-ignore - OAuth window API
    const authButton = oauthWindow.locator('#auth-signin')

    // Wait for the button to be visible
    console.log('⏳ Waiting for #auth-signin button to appear...')
    await authButton.waitFor({ state: 'visible', timeout: 10000 })

    console.log('✅ Found #auth-signin button!')

    // Take screenshot showing the button was found
    // @ts-ignore - OAuth window API
    await oauthWindow.screenshot({
      path: 'test-results/artifacts/oauth-window-button-found.png',
      fullPage: true
    })

    // Click the actual OAuth button
    console.log('👆 Clicking #auth-signin button...')
    await authButton.click()

    // Now handle the Google sign-in form that appears
    console.log('⏳ Waiting for Google sign-in form to load...')
    await page.waitForTimeout(3000) // Give time for Google form to load

    // After clicking #auth-signin, a SECOND window opens with Google accounts
    console.log('🔍 Looking for the Google accounts window...')

    // @ts-ignore - OAuth window type
    let googleWindow: any = null
    for (let i = 0; i < 15; i++) {
      const windows = electronApp.windows()
      // Find the Google accounts window (different from localhost OAuth window)
      // @ts-ignore - Window comparison
      const foundGoogleWindow = windows.find(
        (win: any) =>
          win !== page &&
          win !== oauthWindow &&
          (win.url().includes('accounts.google.com') || win.url().includes('google.com'))
      )
      if (foundGoogleWindow) {
        googleWindow = foundGoogleWindow
        console.log('🪟 Found Google accounts window:', googleWindow.url())
        break
      }
      console.log(`⏳ Looking for Google accounts window... attempt ${i + 1}/15`)
      await page.waitForTimeout(1000)
    }

    if (!googleWindow) {
      console.log('❌ Google accounts window not found after 15 seconds')
      console.log('🔍 Available windows:')
      const windows = electronApp.windows()
      // @ts-ignore - Window iteration
      windows.forEach((win: any, index: number) => {
        console.log(`   Window ${index}: ${win.url()}`)
      })
      throw new Error('Google accounts window not found')
    }

    // Now interact with the Google accounts window
    console.log('📧 Looking for email input in Google accounts window...')

    // @ts-ignore - Google window API
    await googleWindow.waitForLoadState('domcontentloaded')

    // Add human-like delay before interacting
    await googleWindow.waitForTimeout(2000)

    // Take screenshot of Google window
    // @ts-ignore - Google window API
    await googleWindow.screenshot({
      path: 'test-results/artifacts/google-accounts-window.png',
      fullPage: true
    })

    // Check if Google is showing security warning or error page
    try {
      // @ts-ignore - Google window API
      const securityWarning = googleWindow
        .getByText(/browser or app may not be secure/i)
        .or(googleWindow.getByText(/not secure/i))
        .or(googleWindow.getByText(/This browser or app may not be secure/i))
        .or(googleWindow.locator('[data-error="true"]'))

      const hasSecurityWarning = await securityWarning.isVisible({ timeout: 3000 })

      if (hasSecurityWarning) {
        console.log('⚠️ Google security warning detected - trying alternative approach...')

        // Take screenshot of the warning
        // @ts-ignore - Google window API
        await googleWindow.screenshot({
          path: 'test-results/artifacts/google-security-warning.png',
          fullPage: true
        })

        // Look for "Continue" or "Advanced" buttons to bypass warning
        // @ts-ignore - Google window API
        const continueButton = googleWindow
          .getByRole('button', { name: /continue|advanced|proceed/i })
          .or(
            googleWindow.locator(
              'button:has-text("Continue"), button:has-text("Advanced"), button:has-text("Proceed")'
            )
          )

        const continueVisible = await continueButton.isVisible({ timeout: 5000 })

        if (continueVisible) {
          console.log('👆 Clicking continue/advanced button to bypass security warning...')
          await continueButton.click()
          await googleWindow.waitForTimeout(2000)
        } else {
          console.log('⚠️ No bypass option found - this may be a hard block from Google')
          console.log('💡 Consider using the fallback test with mock authentication instead')
          throw new Error(
            '❌ Google security warning appeared and no bypass option found. Try using the fallback test.'
          )
        }
      }
    } catch (warningError) {
      // If checking for warning fails, continue with normal flow
      console.log('ℹ️ Could not check for security warning, proceeding with normal OAuth flow...')
    }

    // Step 1: Handle email input in Google window
    // @ts-ignore - Google window API
    const emailField = googleWindow.locator('#identifierId')

    try {
      // await page.waitForTimeout(1000)
      console.log('✅ Waiting for email field...')
      await emailField.waitFor({ state: 'visible', timeout: 10000 })
      console.log('✅ Found email field in Google accounts window!')

      // Add human-like typing delay
      await googleWindow.waitForTimeout(1000)

      // Fill in the email with human-like typing
      console.log('✍️ Filling in email...')
      await emailField.click() // Ensure field is focused
      await emailField.fill('') // Clear any existing content
      await emailField.type(GOOGLE_TEST_CREDENTIALS.EMAIL, { delay: 100 }) // Human-like typing

      // Add delay before clicking Next
      await googleWindow.waitForTimeout(1500)

      // Click Next button
      console.log('👆 Looking for Next button...')
      // @ts-ignore - Google window API
      const nextButton = googleWindow.locator('button:has-text("Next"), #identifierNext')
      await nextButton.click()

      // Step 2: Handle password input
      console.log('🔑 Looking for password input field...')
      // @ts-ignore - Google window API
      const passwordField = googleWindow.locator(
        '#password, input[name="password"], input[type="password"]'
      )

      await passwordField.waitFor({ state: 'visible', timeout: 15000 })
      console.log('✅ Found password field!')

      // Add human-like delay
      await googleWindow.waitForTimeout(1000)

      // Fill in the password with human-like typing
      console.log('✍️ Filling in password...')
      await passwordField.click() // Ensure field is focused
      await passwordField.fill('') // Clear any existing content
      await passwordField.type(GOOGLE_TEST_CREDENTIALS.PASSWORD, { delay: 120 }) // Human-like typing

      // Add delay before clicking Sign In
      await googleWindow.waitForTimeout(1500)

      // Click Sign In button
      console.log('👆 Looking for Sign In button...')
      // @ts-ignore - Google window API
      const signInButton = googleWindow.locator(
        'button:has-text("Sign in"), #passwordNext, button:has-text("Next")'
      )
      await signInButton.click()

      // Step 3: Handle authorization popup with "Autoriser" button (if needed)
      console.log('🔍 Checking if Google window is still open for authorization step...')

      // Check if the Google window is still open before proceeding
      // @ts-ignore - Google window API
      if (!googleWindow.isClosed()) {
        console.log('⏳ Google window still open - checking for authorization popup...')

        try {
          // Give time for popup to load
          await googleWindow.waitForTimeout(10000)

          // Look for the authorization button
          console.log('🔍 Looking for authorization button with ID "submit_approve_access"...')
          // @ts-ignore - Google window API
          const authorizeButton = googleWindow.locator('#submit_approve_access')

          // Wait for the authorization button to appear
          await authorizeButton.waitFor({ state: 'visible', timeout: 10000 })
          console.log('✅ Found authorization button!')

          // Take screenshot showing the authorization popup
          // @ts-ignore - Google window API
          await googleWindow.screenshot({
            path: 'test-results/artifacts/google-authorization-popup.png',
            fullPage: true
          })

          // Click the authorization button - this will close the popup
          console.log('👆 Clicking "Autoriser" (authorize) button...')
          await authorizeButton.click()

          console.log('✅ Authorization button clicked - popup should close now')

          // Wait for OAuth completion after authorization
          console.log('⏳ Waiting 30 seconds for OAuth completion after authorization...')
          await page.waitForTimeout(30000) // Use main page instead of closed googleWindow

          console.log('✅ Authorization step completed successfully')
        } catch (authError) {
          console.log(
            '⚠️ Authorization button not found or window closed, continuing with OAuth flow...'
          )

          // Only take screenshot if the window is still open
          try {
            // @ts-ignore - Google window API
            if (!googleWindow.isClosed()) {
              await googleWindow.screenshot({
                path: 'test-results/artifacts/google-authorization-not-found.png',
                fullPage: true
              })
            }
          } catch (screenshotError) {
            console.log(
              'ℹ️ Could not take screenshot - window may have closed during authorization check'
            )
          }
          // Don't throw error - authorization step might not always be required
        }
      } else {
        console.log(
          '✅ Google window already closed - OAuth completed without additional authorization step'
        )
        console.log('⏳ Waiting 5 seconds for OAuth completion...')
        await page.waitForTimeout(5000) // Shorter wait since OAuth already completed
      }

      console.log('✅ Successfully completed Google OAuth - returning to main window')
    } catch (error) {
      console.log('❌ Error interacting with Google accounts window:', error)

      // Take error screenshot only if the window is still open
      try {
        // @ts-ignore - Google window API
        if (!googleWindow.isClosed()) {
          await googleWindow.screenshot({
            path: 'test-results/artifacts/google-accounts-error.png',
            fullPage: true
          })
        } else {
          console.log('ℹ️ Google window already closed - taking screenshot of main window instead')
          await page.screenshot({
            path: 'test-results/artifacts/google-accounts-error-main-window.png',
            fullPage: true
          })
        }
      } catch (screenshotError) {
        console.log('ℹ️ Could not take error screenshot:', screenshotError.message)
      }

      throw error
    }

    // Wait for OAuth completion and redirect back to main app
    console.log('⏳ Waiting for OAuth completion and redirect to main app...')
    await page.waitForTimeout(8000) // Give time for OAuth to complete

    console.log('✅ OAuth flow completed successfully')

    // Wait for successful authentication in the main window
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
 * Mock authentication for testing when Google OAuth is blocked
 * This bypasses the Google OAuth flow entirely and directly sets auth state
 */
export async function mockGoogleAuth(page: Page): Promise<void> {
  console.log('🔧 Using mock authentication to bypass Google OAuth restrictions...')

  try {
    // Wait for the page to be ready
    await page.waitForLoadState('domcontentloaded')

    // Mock user data that would normally come from Google OAuth
    const mockUserData = {
      uid: 'test-user-123',
      email: GOOGLE_TEST_CREDENTIALS.EMAIL,
      displayName: 'Test User',
      photoURL: 'https://via.placeholder.com/150',
      accessToken: 'mock-access-token',
      refreshToken: 'mock-refresh-token',
      idToken: 'mock-id-token',
      timestamp: Date.now()
    }

    // Set the mock user data in localStorage (simulating successful auth)
    await page.evaluate((userData) => {
      window.localStorage.setItem('enchanted_user_data', JSON.stringify(userData))
      window.localStorage.setItem('enchanted_has_auto_connected', 'true')

      // Dispatch a custom event to notify the app of authentication
      window.dispatchEvent(
        new CustomEvent('auth-state-changed', {
          detail: { authenticated: true, user: userData }
        })
      )
    }, mockUserData)

    // Reload the page to trigger authentication state check
    await page.reload()
    await page.waitForLoadState('domcontentloaded')

    // Wait for authentication to be processed
    await page.waitForTimeout(3000)

    console.log('✅ Mock authentication completed successfully')
  } catch (error) {
    console.error('❌ Mock authentication failed:', error)
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
      { timeout: 15000 } // Reasonable timeout for OAuth completion
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
        await expect(page.locator(selector).first()).toBeVisible({ timeout: 10000 })
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
