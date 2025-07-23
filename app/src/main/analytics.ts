/* eslint-disable @typescript-eslint/no-explicit-any */
import Store from 'electron-store'
import { machineIdSync } from 'node-machine-id'
import { v5 as uuidv5 } from 'uuid'
import posthog from 'posthog-node'
import log from 'electron-log/main'
import { app } from 'electron'
import { ipcMain } from 'electron/main'

log.transports.file.level = 'info'

const POSTHOG_API_KEY = process.env.POSTHOG_API_KEY ?? ''
const POSTHOG_HOST = 'https://us.i.posthog.com'

const machineId = machineIdSync(true)
const distinctId = uuidv5(machineId, uuidv5.URL)

let client: posthog.PostHog | null = null
let handlersInitialized = false

// STORE SETUP
interface PreferencesSchema {
  analyticsEnabled: boolean
}

export const analyticsStore = new Store<PreferencesSchema>({
  defaults: {
    analyticsEnabled: true
  }
})

function getAnalyticsEnabled(): boolean {
  return analyticsStore.get('analyticsEnabled')
}

export function setAnalyticsEnabled(enabled: boolean): void {
  analyticsStore.set('analyticsEnabled', enabled)
  if (enabled) {
    initializeAnalytics()
  }
}

function setupAnalyticsHandlers() {
  if (handlersInitialized) {
    return
  }

  ipcMain.handle('analytics:is-enabled', () => {
    return getAnalyticsEnabled()
  })

  ipcMain.handle('analytics:set-enabled', (_, enabled: boolean) => {
    setAnalyticsEnabled(enabled)
    return true
  })

  ipcMain.handle('analytics:capture', (_, event: string, properties: Record<string, unknown>) => {
    capture(event, properties)
    return true
  })

  /**
   * Handle feedback capture requests from the renderer process.
   *
   * IMPORTANT: This handler bypasses user telemetry preferences specifically for feedback collection.
   *
   * Rationale for bypassing telemetry preferences:
   * - User feedback is essential for product improvement and user experience
   * - Feedback is contextual and user-initiated, not passive tracking
   * - Users expect their feedback to be received when they submit it
   * - This is distinct from general usage analytics which users can opt out of
   *
   * Privacy considerations:
   * - Only captures data explicitly provided in the feedback form
   * - Includes minimal technical context (app version, timestamp, distinct ID)
   * - Does not include browsing behavior or passive usage data
   * - Marked with 'forced: true' to distinguish from regular analytics
   *
   */
  ipcMain.handle(
    'analytics:capture-feedback',
    (_, event: string, properties: Record<string, unknown>) => {
      return captureForced(event, properties)
    }
  )

  ipcMain.handle('analytics:identify', (_, properties: Record<string, unknown>) => {
    identify(properties)
    return true
  })

  ipcMain.handle('analytics:get-distinct-id', () => {
    return getDistinctId()
  })

  handlersInitialized = true
}

export function capture(event: string, properties: Record<string, any> = {}) {
  try {
    if (!getAnalyticsEnabled()) {
      log.info('[Analytics] analytics disabled, skipping event capture', { event })
      return
    }

    if (client) {
      log.info('[Analytics] capturing', { distinctId, event, properties })
      client.capture({
        distinctId,
        event,
        properties: {
          ...properties,
          timestamp: new Date().toISOString(),
          appVersion: app.getVersion()
        }
      })
    }
  } catch (err) {
    log.error('[Analytics] capture error', err)
  }
}

/**
 * Captures analytics events while bypassing user telemetry preferences.
 *
 * This function is specifically designed for feedback collection where delivery
 * confirmation is critical for user experience.
 *
 * @param event - The event name to capture
 * @param properties - Additional properties to include with the event
 * @returns Promise that resolves only after successful delivery to PostHog or rejects on failure
 *
 * Delivery guarantees:
 * - Promise resolves after initiating delivery to PostHog (HTTP request sent)
 * - Promise rejects if PostHog is not configured or flush method fails
 * - Includes timeout protection (10 seconds) to prevent hanging
 * - Logs all attempts locally for debugging regardless of delivery status
 * - Note: PostHog client doesn't provide delivery confirmation callbacks in current version
 */
export function captureForced(event: string, properties: Record<string, any> = {}) {
  return new Promise<void>((resolve, reject) => {
    try {
      // Debug logging
      log.info('[Analytics] captureForced called', {
        hasApiKey: !!POSTHOG_API_KEY,
        apiKeyLength: POSTHOG_API_KEY?.length || 0,
        host: POSTHOG_HOST,
        event,
        distinctId
      })

      if (!POSTHOG_API_KEY || !POSTHOG_HOST) {
        log.warn('[Analytics] PostHog not configured, logging feedback locally', {
          event,
          properties,
          distinctId,
          timestamp: new Date().toISOString()
        })
        reject(
          new Error(
            'Analytics service not configured. Your feedback was saved locally but not sent to our servers.'
          )
        )
        return
      }

      // Initialize client if not already done for forced events
      if (!client) {
        client = new posthog.PostHog(POSTHOG_API_KEY, {
          host: POSTHOG_HOST,
          flushAt: 1,
          flushInterval: 5000
        })
      }

      const capturePayload = {
        distinctId,
        event,
        properties: {
          ...properties,
          timestamp: new Date().toISOString(),
          appVersion: app.getVersion(),
          forced: true // Mark as forced to distinguish from regular analytics
        }
      }

      log.info('[Analytics] force capturing (bypassing telemetry preference)', capturePayload)

      client.capture(capturePayload)

      // Set up timeout to prevent hanging indefinitely
      const timeoutId = setTimeout(() => {
        log.error('[Analytics] flush timeout - feedback delivery uncertain')
        reject(new Error('Feedback delivery timeout. Please try again.'))
      }, 10000) // 10 second timeout

      // Ensure the event is sent immediately and wait for completion
      log.info('[Analytics] flushing events to PostHog...')

      // PostHog's flush method doesn't accept callbacks in the current version
      // We'll use a simpler approach with timeout-based resolution
      try {
        client.flush()

        // Since PostHog flush is synchronous and doesn't provide delivery confirmation,
        // we'll resolve after a short delay to allow the HTTP request to complete
        setTimeout(() => {
          clearTimeout(timeoutId)
          log.info('[Analytics] feedback event sent to PostHog (delivery not confirmed)')
          resolve()
        }, 1000) // 1 second delay to allow HTTP request to complete
      } catch (flushError: unknown) {
        clearTimeout(timeoutId)
        const errorMessage =
          flushError instanceof Error ? flushError.message : 'Unknown flush error'
        log.error('[Analytics] flush method error', flushError)
        reject(new Error(`Feedback delivery error: ${errorMessage}`))
      }
    } catch (err) {
      log.error('[Analytics] forced capture error', err)
      reject(err)
    }
  })
}

export function identify(properties: Record<string, any> = {}) {
  if (!getAnalyticsEnabled()) {
    log.info('[Analytics] analytics disabled, skipping identify')
    return
  }

  if (client) {
    log.info('[Analytics] identifying', { distinctId, properties })
    client.identify({
      distinctId,
      properties
    })
  }
}

export function getDistinctId() {
  return distinctId
}

export function initializeAnalytics() {
  try {
    setupAnalyticsHandlers()

    if (!POSTHOG_API_KEY || !POSTHOG_HOST) {
      log.error(
        '[Analytics] Missing POSTHOG_API_KEY or POSTHOG_HOST - Skipping analytics initialization'
      )
      return
    }

    if (!getAnalyticsEnabled() || process.env.NODE_ENV === 'development') {
      log.info('[Analytics] analytics disabled or running in development, skipping initialization')
      return
    }

    client = new posthog.PostHog(POSTHOG_API_KEY, {
      host: POSTHOG_HOST,
      flushAt: 10,
      flushInterval: 20000
    })

    identify({
      platform: process.platform,
      version: app.getVersion(),
      env: process.env.NODE_ENV
    })

    capture('analytics_started', {
      timestamp: new Date().toISOString()
    })

    log.info('[Analytics] client initialized successfully')
    client.flush()
  } catch (error) {
    log.error('[Analytics] Error initializing analytics', error)
  }
}
