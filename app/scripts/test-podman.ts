import { installPodman, isPodmanInstalled, startPodman, stopPodman } from '../src/main/podman'
import { spawn } from 'child_process'

async function testPodman() {
  console.log('Testing Podman integration...')

  console.log('Checking if Podman is installed...')
  const isInstalled = await isPodmanInstalled()
  console.log(`Podman installed: ${isInstalled}`)

  if (!isInstalled) {
    console.log('Installing Podman...')
    const installSuccess = await installPodman()
    console.log(`Podman installation ${installSuccess ? 'succeeded' : 'failed'}`)

    if (!installSuccess) {
      console.error('Failed to install Podman. Exiting test.')
      process.exit(1)
    }
  }

  console.log('Starting Podman service...')
  const startSuccess = await startPodman()
  console.log(`Podman service start ${startSuccess ? 'succeeded' : 'failed'}`)

  if (startSuccess) {
    console.log('Podman is running. Waiting 5 seconds before stopping...')

    // Run a simple podman command to test
    const podmanInfoProcess = spawn('podman', ['info'])

    podmanInfoProcess.stdout.on('data', (data) => {
      console.log(`Podman info output: ${data}`)
    })

    podmanInfoProcess.stderr.on('data', (data) => {
      console.error(`Podman info error: ${data}`)
    })

    await new Promise((resolve) => setTimeout(resolve, 5000))

    console.log('Stopping Podman service...')
    const stopSuccess = await stopPodman()
    console.log(`Podman service stop ${stopSuccess ? 'succeeded' : 'failed'}`)
  }

  console.log('Podman test completed.')
}

testPodman().catch((error) => {
  console.error('Error during Podman test:', error)
  process.exit(1)
})
