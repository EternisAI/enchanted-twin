import { ReactNode } from 'react'
import {
  IndexingStatus,
  DataSource as GraphQLDataSource
} from '@renderer/graphql/generated/graphql'

export interface DataSource {
  name: string
  label: string
  description: string
  selectType: 'directory' | 'files'
  fileRequirement: string
  icon: ReactNode
  fileFilters?: { name: string; extensions: string[] }[]
  customView?: {
    name: string
    component: ReactNode
  }
}

export interface DataSourcesPanelProps {
  onDataSourceSelected?: (source: DataSource) => void
  onDataSourceRemoved?: (name: string) => void
  showStatus?: boolean
  indexingStatus?: IndexingStatus
  header?: boolean
}

export interface PendingDataSource {
  name: string
  path: string
  fileSize?: number
}

export interface ExportInstructions {
  timeEstimate: string
  steps: string[]
  link?: string
}

export type IndexedDataSource = Omit<GraphQLDataSource, 'indexProgress'> & {
  indexProgress?: number
}
