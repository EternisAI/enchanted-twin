import { DownloadState, DependencyName } from './DependenciesGate'

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'

  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))

  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))}${sizes[i]}`
}

export const DEPENDENCY_CONFIG: Record<
  DependencyName,
  {
    name: string
    description: string
    disabled?: boolean
  }
> = {
  embeddings: {
    name: 'Embeddings model',
    description: 'Enchanted makes sense of your content'
  },
  anonymizer: {
    name: 'Anonymizer model',
    description: 'Enchanted keeps your data private'
  },
  onnx: {
    name: 'Inference engine',
    description: ''
  },
  LLAMACCP: {
    name: 'LLM engine',
    description: ''
  },
  uv: {
    name: 'Voice mode dependencies',
    description: ''
  },
  postgres: {
    name: 'PostgreSQL database',
    description: 'Vector database for memory storage'
  }
}

export const DEPENDENCY_NAMES: DependencyName[] = Object.keys(DEPENDENCY_CONFIG) as DependencyName[]

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
