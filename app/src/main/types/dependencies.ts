export type DependencyName = 'embeddings' | 'anonymizer' | 'onnx'

export interface DependencyStatus {
  embeddings: boolean
  anonymizer: boolean
  onnx: boolean
}
