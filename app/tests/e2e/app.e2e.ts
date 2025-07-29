import { test, expect, _electron as electron } from '@playwright/test'
import path from 'path'
import { E2E_CONFIG } from './config'

test.describe('Enchanted App - Basic E2E', () => {
  test('Backend connectivity and app launch', async () => {
    console.log('🔍 Testing backend connectivity...')

    // Test backend connectivity first
    const maxWaitTime = 10 * 60 * 1000 // 2 minutes
    const failAfterTime = 5 * 60 * 1000 // 1 minute
    const checkInterval = E2E_CONFIG.DEPENDENCY_CHECK_INTERVAL

    let backendReady = false
    let dependenciesLoaded = false
    const startTime = Date.now()

    // Check backend connectivity
    console.log(`⏳ Checking backend on port ${E2E_CONFIG.BACKEND_GRAPHQL_PORT}...`)

    while (Date.now() - startTime < maxWaitTime) {
      try {
        const url = E2E_CONFIG.getGraphQLUrl()
        console.log('Checking backend connectivity...', url)
        const response = await fetch(url, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ query: '{ __typename }' })
        })

        if (response.ok) {
          backendReady = true
          console.log('✅ Backend is responding')
          break
        }
      } catch (error) {
        // Backend not ready yet
      }

      const elapsed = Date.now() - startTime
      if (elapsed > failAfterTime && !backendReady) {
        throw new Error(`❌ Backend failed to start within 1 minute (${elapsed}ms elapsed)`)
      }

      console.log(`⏳ Waiting for backend... (${Math.round(elapsed / 1000)}s elapsed)`)
      await new Promise((resolve) => setTimeout(resolve, checkInterval))
    }

    if (!backendReady) {
      throw new Error(`❌ Backend failed to start within 2 minutes`)
    }

    // Check for dependencies (GraphQL schema, database, etc.)
    console.log('🔍 Checking backend dependencies...')

    while (Date.now() - startTime < maxWaitTime && !dependenciesLoaded) {
      try {
        // Test a more complex GraphQL query to ensure dependencies are loaded
        const response = await fetch(E2E_CONFIG.getGraphQLUrl(), {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            query: `
              query { 
                __schema { 
                  types { 
                    name 
                  } 
                } 
              }
            `
          })
        })

        const result = await response.json()
        if (response.ok && result.data && result.data.__schema) {
          dependenciesLoaded = true
          console.log('✅ Backend dependencies loaded')
          break
        }
      } catch (error) {
        // Dependencies not ready yet
      }

      const elapsed = Date.now() - startTime
      if (elapsed > failAfterTime && !dependenciesLoaded) {
        throw new Error(
          `❌ Backend dependencies failed to load within 1 minute (${elapsed}ms elapsed)`
        )
      }

      console.log(`⏳ Waiting for dependencies... (${Math.round(elapsed / 1000)}s elapsed)`)
      await new Promise((resolve) => setTimeout(resolve, checkInterval))
    }

    if (!dependenciesLoaded) {
      throw new Error(`❌ Backend dependencies failed to load within 2 minutes`)
    }

    const totalElapsed = Date.now() - startTime
    console.log(`✅ Backend fully ready in ${Math.round(totalElapsed / 1000)}s`)

    // Now launch the Electron app
    console.log('🚀 Launching Electron app...')
    const electronApp = await electron.launch({
      args: [path.join(__dirname, '../../out/main/index.js')],
      env: {
        ...process.env,
        NODE_ENV: 'test'
      }
    })

    // Get the first (main) window
    const page = await electronApp.firstWindow()

    // Wait for the page to load
    await page.waitForLoadState('domcontentloaded')

    // Wait a bit for the app to fully initialize
    await page.waitForTimeout(2000)

    // Verify the app window is visible by checking body element
    const body = page.locator('body')
    await expect(body).toBeVisible()

    // Check the page title contains "Enchanted"
    const title = await page.title()
    expect(title).toContain('Enchanted')

    // Take a full page screenshot
    await page.screenshot({
      path: 'test-results/artifacts/app-launch.png',
      fullPage: true
    })

    // Also take a screenshot using Playwright's built-in artifact system
    await page.screenshot({
      fullPage: true,
      path: `test-results/artifacts/app-launch-${Date.now()}.png`
    })

    console.log('✅ App launched successfully and screenshot saved!')

    // Keep the app running for extended testing
    console.log(
      `🕒 Keeping app running for ${E2E_CONFIG.APP_RUNTIME_DURATION / 60000} minutes for extended testing...`
    )

    // Take periodic screenshots to monitor app state
    const screenshotIntervals = Math.floor(
      E2E_CONFIG.APP_RUNTIME_DURATION / E2E_CONFIG.SCREENSHOT_INTERVAL
    )
    for (let i = 1; i <= screenshotIntervals; i++) {
      const elapsedSeconds = (i * E2E_CONFIG.SCREENSHOT_INTERVAL) / 1000
      console.log(`⏱️  App running... ${elapsedSeconds}s elapsed`)
      await page.waitForTimeout(E2E_CONFIG.SCREENSHOT_INTERVAL)

      // Take a screenshot every interval
      await page.screenshot({
        fullPage: true,
        path: `test-results/artifacts/app-runtime-${elapsedSeconds}s-${Date.now()}.png`
      })

      // Verify app is still responsive
      const bodyVisible = await page.locator('body').isVisible()
      expect(bodyVisible).toBe(true)
      console.log(`✅ App still responsive at ${elapsedSeconds}s`)
    }

    console.log(`✅ App successfully ran for ${E2E_CONFIG.APP_RUNTIME_DURATION / 60000} minutes!`)

    // Close the app
    await electronApp.close()
  })

  test('App window properties', async () => {
    const electronApp = await electron.launch({
      args: [path.join(__dirname, '../../out/main/index.js')]
    })

    const page = await electronApp.firstWindow()
    await page.waitForLoadState('domcontentloaded')

    // Check that main content area exists and get dimensions
    const body = page.locator('body')
    await expect(body).toBeVisible()

    // Get the actual page dimensions
    const bodyBox = await body.boundingBox()
    if (bodyBox) {
      expect(bodyBox.width).toBeGreaterThan(400) // More reasonable minimum
      expect(bodyBox.height).toBeGreaterThan(300)
      console.log(`App window size: ${bodyBox.width}x${bodyBox.height}`)
    }

    // Take screenshot of window properties test
    await page.screenshot({
      fullPage: true,
      path: `test-results/artifacts/app-properties-${Date.now()}.png`
    })

    await electronApp.close()
  })
})
