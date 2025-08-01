import { ChildProcess, spawn } from 'child_process'

async function globalTeardown() {
  console.log('🧹 Cleaning up E2E test environment...')

  try {
    // Stop backend server
    const backendProcess = global.backendProcess as ChildProcess | null
    if (backendProcess && backendProcess.pid && !backendProcess.killed) {
      console.log('🛑 Stopping backend server...')

      // Try graceful shutdown first
      backendProcess.kill('SIGTERM')

      // Wait a bit for graceful shutdown
      await new Promise((resolve) => setTimeout(resolve, 3000))

      // Force kill if still running
      if (backendProcess.pid && !backendProcess.killed) {
        console.log('🔪 Force killing backend server...')
        backendProcess.kill('SIGKILL')

        // Wait a bit for the force kill to take effect
        await new Promise((resolve) => setTimeout(resolve, 1000))
      }

      console.log('✅ Backend server stopped')
    } else {
      console.log('ℹ️  No backend server process to stop')
    }

    // Also try to kill any remaining enchanted-twin processes
    try {
      const killProcess = spawn('pkill', ['-f', 'enchanted-twin'], { stdio: 'ignore' })
      await new Promise((resolve) => {
        killProcess.on('close', () => resolve(undefined))
        setTimeout(() => resolve(undefined), 2000) // Timeout after 2 seconds
      })
      console.log('🧹 Cleaned up any remaining enchanted-twin processes')
    } catch (error) {
      // pkill might fail if no processes found, which is fine
    }

    // Clean up any orphaned livekit-agent processes from test runs
    try {
      const killLivekitProcess = spawn('pkill', ['-f', 'electron-test.*livekit-agent'], {
        stdio: 'ignore'
      })
      await new Promise((resolve) => {
        killLivekitProcess.on('close', () => resolve(undefined))
        setTimeout(() => resolve(undefined), 2000) // Timeout after 2 seconds
      })
      console.log('🧹 Cleaned up any remaining livekit-agent test processes')
    } catch (error) {
      // pkill might fail if no processes found, which is fine
    }

    console.log('✅ E2E test cleanup completed')
  } catch (error) {
    console.error('❌ Error during E2E test cleanup:', error)
    // Don't throw error in teardown to avoid masking test failures
  }
}

export default globalTeardown
