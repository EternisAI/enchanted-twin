import { spawn, ChildProcess } from 'child_process'
import path from 'path'
import { promises as fs, mkdirSync, openSync, closeSync } from 'fs'
import { E2E_CONFIG, BACKEND_ENV } from '../config'

let backendProcess: ChildProcess | null = null

const backendPath = path.join(__dirname, '../../../../backend/golang')
export async function buildBackendServer(): Promise<void> {
  return new Promise((resolve, reject) => {
    console.log('🔨 Building backend server...')

    const buildProcess = spawn('make', ['build'], {
      cwd: backendPath,
      stdio: 'pipe'
    })

    let buildOutput = ''
    if (buildProcess.stdout) {
      buildProcess.stdout.on('data', (data) => {
        buildOutput += data.toString()
      })
    }

    if (buildProcess.stderr) {
      buildProcess.stderr.on('data', (data) => {
        buildOutput += data.toString()
      })
    }

    buildProcess.on('close', (code) => {
      if (code === 0) {
        console.log('✅ Backend server built successfully')
        resolve()
      } else {
        console.error('❌ Backend build failed:', buildOutput)
        reject(new Error(`Backend build failed with code ${code}`))
      }
    })

    buildProcess.on('error', (error) => {
      console.error('❌ Failed to start backend build:', error)
      reject(error)
    })
  })
}

export async function startBackendServer(): Promise<void> {
  return new Promise((resolve, reject) => {
    const logsPath = path.join(backendPath, 'output')

    console.log('🖥️  Starting backend server for E2E tests...')

    try {
      mkdirSync(logsPath, { recursive: true })
    } catch (error) {
      // Directory might already exist
    }

    const stdoutFd = openSync(path.join(logsPath, 'e2e-backend-stdout.log'), 'w')
    const stderrFd = openSync(path.join(logsPath, 'e2e-backend-stderr.log'), 'w')

    console.log('🔍 Starting backend server with env:', {
      ...BACKEND_ENV,
      NODE_ENV: 'production'
    })

    backendProcess = spawn('./bin/enchanted-twin', [], {
      cwd: backendPath,
      env: {
        ...process.env,
        ...BACKEND_ENV
      },
      stdio: ['ignore', stdoutFd, stderrFd]
    })

    let hasStarted = false

    // Monitor the log files for startup messages
    const startupChecker = setInterval(async () => {
      try {
        const stdoutContent = await fs.readFile(
          path.join(logsPath, 'e2e-backend-stdout.log'),
          'utf8'
        )
        const stderrContent = await fs.readFile(
          path.join(logsPath, 'e2e-backend-stderr.log'),
          'utf8'
        )

        const combinedOutput = stdoutContent + stderrContent

        if (combinedOutput.includes('Starting GraphQL HTTP server') && !hasStarted) {
          hasStarted = true
          clearInterval(startupChecker)
          console.log('✅ Backend server started (logs written to files)')
          setTimeout(() => resolve(), 2000)
        }
      } catch (error) {
        // File might not exist yet, continue checking
      }
    }, 500)

    backendProcess.on('error', (error) => {
      clearInterval(startupChecker)
      closeSync(stdoutFd)
      closeSync(stderrFd)
      console.error('❌ Failed to start backend server:', error)
      reject(error)
    })

    // Timeout after configured time
    setTimeout(() => {
      if (!hasStarted) {
        clearInterval(startupChecker)
        closeSync(stdoutFd)
        closeSync(stderrFd)
        reject(
          new Error(
            `Backend server failed to start within ${E2E_CONFIG.BACKEND_STARTUP_TIMEOUT / 1000} seconds`
          )
        )
      }
    }, E2E_CONFIG.BACKEND_STARTUP_TIMEOUT)

    // Clean up file descriptors when process exits
    backendProcess.on('exit', () => {
      clearInterval(startupChecker)
      closeSync(stdoutFd)
      closeSync(stderrFd)
    })
  })
}

export async function waitForBackendReady(): Promise<void> {
  const maxAttempts = 30
  const delay = 1000

  for (let i = 0; i < maxAttempts; i++) {
    try {
      // Use the shared config for the GraphQL URL
      const response = await fetch(E2E_CONFIG.getGraphQLUrl(E2E_CONFIG.BACKEND_GRAPHQL_PORT), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ query: '{ __typename }' })
      })

      if (response.ok) {
        console.log('✅ Backend server is ready and responding')
        return
      }
    } catch (error) {
      // Ignore connection errors and keep trying
    }

    console.log(`⏳ Waiting for backend server... (${i + 1}/${maxAttempts})`)
    await new Promise((resolve) => setTimeout(resolve, delay))
  }

  throw new Error('Backend server did not become ready within expected time')
}

export async function startCompleteBackend(): Promise<void> {
  console.log('🚀 Starting complete backend setup...')

  try {
    // Build the backend server
    await buildBackendServer()

    // Start the backend server
    await startBackendServer()

    // Wait for backend to be ready
    await waitForBackendReady()

    console.log('✅ Complete backend setup completed successfully')
  } catch (error) {
    console.error('❌ Complete backend setup failed:', error)
    await stopBackendServer()
    throw error
  }
}

export async function stopBackendServer(): Promise<void> {
  if (backendProcess) {
    console.log('🛑 Stopping backend server...')

    return new Promise((resolve) => {
      const process = backendProcess!

      process.on('exit', () => {
        console.log('✅ Backend server stopped')
        backendProcess = null
        resolve()
      })

      // Try graceful shutdown first
      process.kill('SIGTERM')

      // Force kill after timeout
      setTimeout(() => {
        if (backendProcess) {
          console.log('⚠️ Force stopping backend server...')
          process.kill('SIGKILL')
        }
      }, 5000)
    })
  } else {
    console.log('ℹ️ No backend process to stop')
  }
}

export function getBackendProcess(): ChildProcess | null {
  return backendProcess
}
