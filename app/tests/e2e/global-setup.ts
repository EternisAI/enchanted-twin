import { FullConfig } from '@playwright/test'
import { spawn, ChildProcess } from 'child_process'
import path from 'path'
import { promises as fs } from 'fs'
import { E2E_CONFIG, BACKEND_ENV } from './config'

let backendProcess: ChildProcess | null = null

async function globalSetup(config: FullConfig) {
  console.log('🚀 Starting E2E test setup...')

  try {
    // Build the backend server
    await buildBackendServer()

    // Start the backend server
    await startBackendServer()

    // Wait for backend to be ready
    await waitForBackendReady()

    console.log('✅ E2E test setup completed successfully')
  } catch (error) {
    console.error('❌ E2E test setup failed:', error)
    throw error
  }
}

async function buildBackendServer(): Promise<void> {
  return new Promise((resolve, reject) => {
    const backendPath = path.join(__dirname, '../../../backend/golang')
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

async function startBackendServer(): Promise<void> {
  return new Promise((resolve, reject) => {
    const backendPath = path.join(__dirname, '../../../backend/golang')
    console.log('🖥️  Starting backend server for E2E tests...')

    backendProcess = spawn('./bin/enchanted-twin', [], {
      cwd: backendPath,
      env: {
        ...process.env,
        ...BACKEND_ENV
      },
      stdio: 'pipe'
    })

    let hasStarted = false

    if (backendProcess.stdout) {
      backendProcess.stdout.on('data', (data) => {
        const output = data.toString()
        console.log(`Backend: ${output.trim()}`)

        // Look for GraphQL server startup message
        if (output.includes('Starting GraphQL HTTP server') && !hasStarted) {
          hasStarted = true
          setTimeout(() => resolve(), 2000) // Give it a moment to fully start
        }
      })
    }

    if (backendProcess.stderr) {
      backendProcess.stderr.on('data', (data) => {
        const error = data.toString()
        console.log(`Backend Log: ${error.trim()}`)

        // Don't treat warnings as errors, but still log them
        if (error.includes('Starting GraphQL HTTP server') && !hasStarted) {
          hasStarted = true
          setTimeout(() => resolve(), 2000)
        }
      })
    }

    backendProcess.on('error', (error) => {
      console.error('❌ Failed to start backend server:', error)
      reject(error)
    })

    // Timeout after configured time
    setTimeout(() => {
      if (!hasStarted) {
        reject(
          new Error(
            `Backend server failed to start within ${E2E_CONFIG.BACKEND_STARTUP_TIMEOUT / 1000} seconds`
          )
        )
      }
    }, E2E_CONFIG.BACKEND_STARTUP_TIMEOUT)
  })
}

async function waitForBackendReady(): Promise<void> {
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

// Store the process globally so teardown can access it
global.backendProcess = backendProcess

export default globalSetup
