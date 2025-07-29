import { ChildProcess } from 'child_process'

async function globalTeardown() {
  console.log('🧹 Cleaning up E2E test environment...')

  try {
    // Stop backend server
    const backendProcess = global.backendProcess as ChildProcess | null
    if (backendProcess && !backendProcess.killed) {
      console.log('🛑 Stopping backend server...')

      // Try graceful shutdown first
      backendProcess.kill('SIGTERM')

      // Wait a bit for graceful shutdown
      await new Promise((resolve) => setTimeout(resolve, 3000))

      // Force kill if still running
      if (!backendProcess.killed) {
        console.log('🔪 Force killing backend server...')
        backendProcess.kill('SIGKILL')
      }

      console.log('✅ Backend server stopped')
    } else {
      console.log('ℹ️  No backend server process to stop')
    }

    console.log('✅ E2E test cleanup completed')
  } catch (error) {
    console.error('❌ Error during E2E test cleanup:', error)
    // Don't throw error in teardown to avoid masking test failures
  }
}

export default globalTeardown
