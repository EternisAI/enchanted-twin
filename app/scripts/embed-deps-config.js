#!/usr/bin/env node

import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

// Read the runtime dependencies config
const configPath = path.join(__dirname, '..', '..', 'runtime-dependencies.json')
let config
try {
  const configData = fs.readFileSync(configPath, 'utf8')
  config = JSON.parse(configData)
} catch (error) {
  if (error.code === 'ENOENT') {
    console.error(`❌ Failed to read runtime-dependencies.json: File not found at ${configPath}`)
  } else if (error instanceof SyntaxError) {
    console.error(`❌ Failed to parse runtime-dependencies.json: Invalid JSON syntax`)
    console.error(`   Error: ${error.message}`)
  } else {
    console.error(`❌ Failed to read runtime-dependencies.json: ${error.message}`)
  }
  process.exit(1)
}

// Generate TypeScript file with embedded config
const tsContent = `// @generated
// This file is auto-generated at build time from runtime-dependencies.json
// Do not edit manually!

export const EMBEDDED_RUNTIME_DEPS_CONFIG = ${JSON.stringify(config, null, 2)} as const;
`

// Write the generated TypeScript file for main process
const mainOutputPath = path.join(__dirname, '..', 'src', 'main', 'embeddedDepsConfig.ts')
fs.writeFileSync(mainOutputPath, tsContent)

// Also create a renderer-accessible version
const rendererOutputPath = path.join(
  __dirname,
  '..',
  'src',
  'renderer',
  'src',
  'embeddedDepsConfig.ts'
)
fs.writeFileSync(rendererOutputPath, tsContent)

console.log('✓ Embedded runtime dependencies config generated at:', mainOutputPath)
console.log('✓ Embedded runtime dependencies config generated at:', rendererOutputPath)
