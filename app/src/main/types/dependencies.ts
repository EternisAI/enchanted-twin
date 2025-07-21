export type DependencyName = 'embeddings' | 'anonymizer' | 'onnx' | 'LLAMACCP' | 'uv'

export type PostInstallHook = (dependencyDir: string) => Promise<void>
