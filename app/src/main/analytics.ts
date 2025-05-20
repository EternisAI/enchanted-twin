/* eslint-disable @typescript-eslint/no-explicit-any */
import Store from 'electron-store'
import { machineIdSync } from 'node-machine-id'
import { v5 as uuidv5 } from 'uuid'
import posthog from 'posthog-node'
import log from 'electron-log/main'
import { app } from 'electron'
import { ipcMain } from 'electron/main'

log.transports.file.level = 'info'

const POSTHOG_API_KEY = 'phc_z8xhkNCHHUClOYiQ79nLsMeY7rxbWqCpI8KQUmmcKd8'
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
          timestamp: new Date().toISOString()
        }
      })
    }
  } catch (err) {
    log.error('[Analytics] capture error', err)
  }
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
      flushAt: 1,
      flushInterval: 5000
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
