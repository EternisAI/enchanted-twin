#!/usr/bin/env node

const fs = require('fs')
const path = require('path')

// Read the runtime dependencies config
const configPath = path.join(__dirname, '..', '..', 'runtime-dependencies.json')
const configData = fs.readFileSync(configPath, 'utf8')
const config = JSON.parse(configData)

// Generate TypeScript file with embedded config
const tsContent = `// @generated
// This file is auto-generated at build time from runtime-dependencies.json
// Do not edit manually!

export const EMBEDDED_RUNTIME_DEPS_CONFIG = ${JSON.stringify(config, null, 2)} as const;
`

// Write the generated TypeScript file
const outputPath = path.join(__dirname, '..', 'src', 'main', 'embeddedDepsConfig.ts')
fs.writeFileSync(outputPath, tsContent)

console.log('✓ Embedded runtime dependencies config generated at:', outputPath)