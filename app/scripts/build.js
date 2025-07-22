#!/usr/bin/env node

import { execSync } from 'child_process'

const args = process.argv.slice(2)
const platform = args[0] || 'mac'
const environment = args[1] || 'prod'

console.log(`Building for ${platform} in ${environment} mode...`)

try {
  if (environment === 'dev') {
    console.log('Building dev version...')
    execSync(`pnpm run build:dev:${platform}`, { stdio: 'inherit' })
  } else {
    console.log('Building production version...')
    execSync(`pnpm run build:${platform}`, { stdio: 'inherit' })
  }

  console.log('Build completed successfully!')
} catch (error) {
  console.error('Build failed:', error.message)
  process.exit(1)
}
