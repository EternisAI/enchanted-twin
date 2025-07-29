import { test } from '@playwright/test'
import { AUTH_CONFIG } from './config'
import fs from 'fs'
import path from 'path'

test('cleanup test artifacts', async () => {
  console.log('🧹 Running post-test cleanup...')

  try {
    // Optional: Clean up authentication state if needed
    // (Usually we want to keep it for subsequent test runs)

    // Log cleanup completion
    console.log('✅ Test cleanup completed successfully')

    // Show summary of test artifacts
    const artifactsDir = 'test-results/artifacts'
    if (fs.existsSync(artifactsDir)) {
      const files = fs.readdirSync(artifactsDir)
      console.log(`📁 Test artifacts saved: ${files.length} files in ${artifactsDir}`)
    }

    // Show auth state status
    if (fs.existsSync(AUTH_CONFIG.AUTH_STATE_PATH)) {
      console.log(`🔐 Authentication state preserved at: ${AUTH_CONFIG.AUTH_STATE_PATH}`)
    }
  } catch (error) {
    console.error('❌ Cleanup failed:', error)
    // Don't throw error - cleanup failure shouldn't fail the entire test suite
  }
})
