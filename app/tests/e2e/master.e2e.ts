import {
  test,
  expect,
  _electron as electron,
  type Page,
  type ElectronApplication
} from '@playwright/test'
import path from 'path'
import fs from 'fs'
import {
  signInWithGoogle,
  isAuthenticated,
  clearAuthState,
  createCleanElectronConfig,
  cleanupTempDirectories
} from './helpers/auth.helpers'
import { waitForChatInterface, runAllChatTests } from './helpers/chat.helpers'
import { startCompleteBackend, stopBackendServer } from './helpers/backend.helpers'

// Constants
const TIMEOUTS = {
  SHORT: 5000,
  MEDIUM: 10000,
  LONG: 60000
} as const

const SCREENSHOT_PATHS = {
  INITIAL: 'test-results/artifacts/master-test-initial-state.png',
  AUTHENTICATED: 'test-results/artifacts/master-test-authenticated-state.png',
  CHAT_NOT_READY: 'test-results/artifacts/master-test-chat-not-ready.png',
  FINAL_SUCCESS: 'test-results/artifacts/master-test-final-success.png',
  ERROR: 'test-results/artifacts/master-test-error.png'
} as const

// Test logger utility
class TestLogger {
  static phase(phase: number, title: string) {
    console.log(`\n🔧 PHASE ${phase}: ${title}...`)
  }

  static step(phase: number, step: number, title: string) {
    console.log(`🔍 Step ${phase}.${step}: ${title}...`)
  }

  static success(message: string) {
    console.log(`✅ ${message}`)
  }

  static warning(message: string) {
    console.log(`⚠️ ${message}`)
  }

  static error(message: string) {
    console.error(`❌ ${message}`)
  }

  static info(message: string) {
    console.log(`📋 ${message}`)
  }
}

// Screenshot utility
class ScreenshotHelper {
  static async capture(page: Page, path: string, description?: string) {
    await page.screenshot({ path, fullPage: true })
    if (description) {
      TestLogger.info(`Screenshot saved: ${description}`)
    }
  }
}

test.describe('Master E2E Test Suite - Auth + Chat', () => {
  test.beforeAll(async () => {
    const tempDir = path.join(__dirname, '../../../temp')
    if (!fs.existsSync(tempDir)) {
      fs.mkdirSync(tempDir, { recursive: true })
    }
  })

  test.afterAll(async () => {
    await cleanupTempDirectories()
  })

  test('complete flow: authentication then chat interaction', async () => {
    TestLogger.info('🎯 Starting MASTER E2E test: Backend → Auth → Chat flow...')
    printTestOverview()

    let electronApp: ElectronApplication | undefined

    try {
      await startBackendPhase()
      electronApp = await launchElectronPhase()
      const page = await electronApp.firstWindow()

      await authenticationPhase(page, electronApp)
      await chatTestingPhase(page)
      await finalVerificationPhase(page)

      TestLogger.success('🎉 MASTER E2E TEST COMPLETED SUCCESSFULLY!')
    } catch (error) {
      await handleTestError(error, electronApp)
      throw error
    } finally {
      await cleanupPhase(electronApp)
    }
  })
})

// Phase functions
function printTestOverview() {
  TestLogger.info('📋 This test will:')
  TestLogger.info('   1. Build and start backend server')
  TestLogger.info('   2. Launch Electron app')
  TestLogger.info('   3. Perform Google OAuth authentication')
  TestLogger.info('   4. Test chat functionality (same instance)')
  TestLogger.info('   5. Clean up backend and Electron')
}

async function startBackendPhase() {
  TestLogger.phase(1, 'Starting backend server')
  await startCompleteBackend()
}

async function launchElectronPhase() {
  TestLogger.phase(2, 'Launching Electron app')
  const electronApp = await electron.launch(createCleanElectronConfig())
  return electronApp
}

async function authenticationPhase(page: Page, electronApp: ElectronApplication) {
  TestLogger.phase(3, 'Performing authentication')

  await verifyUnauthenticatedState(page)
  await performGoogleSignIn(page, electronApp)
  await verifyAuthenticationSuccess(page)

  TestLogger.success('PHASE 3 completed: Authentication successful!')
}

async function verifyUnauthenticatedState(page: Page) {
  TestLogger.step(3, 1, 'Verifying initial unauthenticated state')

  await clearAuthState(page)
  await page.reload()
  await page.waitForLoadState('domcontentloaded')
  await expect(page.getByText('Continue with Google')).toBeVisible({
    timeout: TIMEOUTS.LONG
  })

  await ScreenshotHelper.capture(page, SCREENSHOT_PATHS.INITIAL, 'Initial state')
  TestLogger.success('Step 3.1 completed: Confirmed unauthenticated state')
}

async function performGoogleSignIn(page: Page, electronApp: ElectronApplication) {
  TestLogger.step(3, 2, 'Performing Google OAuth sign-in')
  await signInWithGoogle(page, electronApp)
}

async function verifyAuthenticationSuccess(page: Page) {
  TestLogger.step(3, 3, 'Verifying authentication success')

  const authStatus = await isAuthenticated(page)
  expect(authStatus).toBe(true)

  const hasUserData = await checkUserDataInLocalStorage(page)
  expect(hasUserData).toBe(true)

  await ScreenshotHelper.capture(page, SCREENSHOT_PATHS.AUTHENTICATED, 'Authenticated state')
}

async function checkUserDataInLocalStorage(page: Page): Promise<boolean> {
  return await page.evaluate(() => {
    const userData = window.localStorage.getItem('enchanted_user_data')
    return userData !== null && userData !== 'undefined'
  })
}

async function chatTestingPhase(page: Page) {
  TestLogger.phase(4, 'Testing chat functionality (same instance)')

  await waitForChatInterfaceReady(page)
  await runAllChatTests(page)

  TestLogger.success('PHASE 4 completed: Chat functionality testing finished!')
}

async function waitForChatInterfaceReady(page: Page) {
  TestLogger.step(4, 1, 'Waiting for chat interface to be ready')

  await page.waitForTimeout(TIMEOUTS.SHORT)

  try {
    await waitForChatInterface(page)
    TestLogger.success('Step 4.1 completed: Chat interface is ready')
  } catch (chatInterfaceError) {
    TestLogger.warning('Chat interface not immediately ready, taking debug screenshot...')
    await ScreenshotHelper.capture(page, SCREENSHOT_PATHS.CHAT_NOT_READY, 'Chat not ready')

    await page.waitForTimeout(TIMEOUTS.MEDIUM)
    await waitForChatInterface(page)
    TestLogger.success('Step 4.1 completed: Chat interface is ready (after additional wait)')
  }
}

async function finalVerificationPhase(page: Page) {
  TestLogger.phase(5, 'Final verification')

  const finalAuthStatus = await isAuthenticated(page)
  expect(finalAuthStatus).toBe(true)

  await ScreenshotHelper.capture(page, SCREENSHOT_PATHS.FINAL_SUCCESS, 'Final success')
  TestLogger.success('PHASE 5 completed: Final verification passed')
}

async function handleTestError(error: unknown, electronApp?: ElectronApplication) {
  TestLogger.error('MASTER E2E TEST FAILED: ' + error)

  if (electronApp) {
    try {
      const page = await electronApp.firstWindow()
      await ScreenshotHelper.capture(page, SCREENSHOT_PATHS.ERROR, 'Error state')
      await logErrorDetails(page)
    } catch (screenshotError) {
      TestLogger.error('Could not take error screenshot: ' + screenshotError)
    }
  }
}

async function logErrorDetails(page: Page) {
  TestLogger.info('Current page URL: ' + page.url())

  const bodyText = await page.locator('body').textContent()
  TestLogger.info('Page body text (first 500 chars): ' + bodyText?.substring(0, 500))

  const userDataExists = await checkUserDataInLocalStorage(page)
  TestLogger.info('User data in localStorage: ' + userDataExists)
}

async function cleanupPhase(electronApp?: ElectronApplication) {
  TestLogger.info('\n🧹 Cleaning up...')

  if (electronApp) {
    await closeElectronApp(electronApp)
  }
  await stopBackend()

  TestLogger.success('Cleanup completed')
}

async function closeElectronApp(electronApp: ElectronApplication) {
  try {
    await electronApp.close()
    TestLogger.success('Electron app closed')
  } catch (error) {
    TestLogger.warning('Error closing Electron app: ' + error)
  }
}

async function stopBackend() {
  try {
    await stopBackendServer()
    TestLogger.success('Backend server stopped')
  } catch (error) {
    TestLogger.warning('Error stopping backend server: ' + error)
  }
}
