// codegen.ts
import type { CodegenConfig } from '@graphql-codegen/cli'

const config: CodegenConfig = {
  overwrite: true,
  schema: '../backend/golang/graph/schema.graphqls',
  documents: 'src/renderer/src/graphql/operations.gql',
  generates: {
    'src/renderer/src/graphql/generated/': {
      preset: 'client',
      presetConfig: {
        gqlTagName: 'gql'
      }
    }
  },
  ignoreNoDocuments: true // Don't error if no .gql files are found initially
}

export default config
