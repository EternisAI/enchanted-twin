import { EMBEDDED_RUNTIME_DEPS_CONFIG } from '../embeddedDepsConfig'

// Extract only model dependencies from the config
type ModelEntries = {
  [K in keyof typeof EMBEDDED_RUNTIME_DEPS_CONFIG.dependencies]: (typeof EMBEDDED_RUNTIME_DEPS_CONFIG.dependencies)[K]['category'] extends 'model'
    ? K
    : never
}[keyof typeof EMBEDDED_RUNTIME_DEPS_CONFIG.dependencies & string]

export type ModelName = ModelEntries
