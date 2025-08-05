import { DownloadState } from './DependenciesGate'
import { DependencyName } from '../../types/dependencies'
import { EMBEDDED_RUNTIME_DEPS_CONFIG } from '../../embeddedDepsConfig'

// Type for dependency config from embedded runtime deps
type EmbeddedDependencyConfig = {
  name?: string
  display_name?: string
  description?: string
  category?: string
  [key: string]: unknown
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'

  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))}${sizes[i]}`
}

// Generate dependency config from JSON using embedded display names and descriptions
export const DEPENDENCY_CONFIG: Record<
  DependencyName,
  {
    name: string
    description: string
    disabled?: boolean
  }
> = (() => {
  const config: Record<
    DependencyName,
    {
      name: string
      description: string
      disabled?: boolean
    }
  > = {} as Record<
    DependencyName,
    {
      name: string
      description: string
      disabled?: boolean
    }
  >
  const deps = EMBEDDED_RUNTIME_DEPS_CONFIG?.dependencies || {}

  for (const [depName, depConfig] of Object.entries(deps)) {
    const typedConfig = depConfig as EmbeddedDependencyConfig
    const configEntry = config as Record<
      string,
      { name: string; description: string; disabled?: boolean }
    >
    configEntry[depName] = {
      name: typedConfig.display_name || typedConfig.name || depName,
      description: typedConfig.description || '',
      disabled: false
    }
  }

  return config
})()

export const DEPENDENCY_NAMES: DependencyName[] = (Object.keys(
  EMBEDDED_RUNTIME_DEPS_CONFIG?.dependencies || {}
) as string[]) as DependencyName[]

export const MODEL_NAMES: DependencyName[] = DEPENDENCY_NAMES.filter((name) => {
  const config = EMBEDDED_RUNTIME_DEPS_CONFIG?.dependencies?.[name]
  return config?.category === 'model'
})

export const initialDownloadState: DownloadState = DEPENDENCY_NAMES.reduce((acc, dependency) => {
  acc[dependency] = {
    downloading: false,
    percentage: 0,
    completed: false,
    totalBytes: 0,
    downloadedBytes: 0
  }
  return acc
}, {} as DownloadState)
