#!/usr/bin/env node

import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

// Function to resolve $ref references in configuration
function resolveReferences(obj, rootConfig) {
  if (typeof obj !== 'object' || obj === null) {
    return obj
  }

  if (Array.isArray(obj)) {
    return obj.map(item => resolveReferences(item, rootConfig))
  }

  const result = {}
  for (const [key, value] of Object.entries(obj)) {
    if (key === '$ref' && typeof value === 'string') {
      // Parse JSON Pointer reference (e.g., "#/shared_configs/darwin_postgres")
      const refPath = value.replace(/^#\//, '').split('/')
      let referencedValue = rootConfig
      
      for (const pathSegment of refPath) {
        if (referencedValue && typeof referencedValue === 'object' && pathSegment in referencedValue) {
          referencedValue = referencedValue[pathSegment]
        } else {
          throw new Error(`Cannot resolve reference: ${value}`)
        }
      }
      
      // Return the resolved reference, also resolving any nested references
      return resolveReferences(referencedValue, rootConfig)
    } else {
      result[key] = resolveReferences(value, rootConfig)
    }
  }
  
  return result
}

// Read the runtime dependencies config
const configPath = path.join(__dirname, '..', '..', 'runtime-dependencies.json')
let config
try {
  const configData = fs.readFileSync(configPath, 'utf8')
  const rawConfig = JSON.parse(configData)
  
  // Resolve all $ref references
  config = resolveReferences(rawConfig, rawConfig)
  
  // Remove the shared_configs section from the final output as it's no longer needed
  if (config.shared_configs) {
    delete config.shared_configs
  }
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
