import { ReactNode } from 'react'

export interface DataSource {
  name: string
  label: string
  description: string
  selectType: 'directory' | 'files'
  fileRequirement: string
  icon: ReactNode
  fileFilters?: { name: string; extensions: string[] }[]
}

export interface DataSourcesPanelProps {
  onDataSourceSelected?: (source: DataSource) => void
  onDataSourceRemoved?: (name: string) => void
}

export interface PendingDataSource {
  name: string
  path: string
}

export interface ExportInstructions {
  timeEstimate: string
  steps: string[]
  link?: string
}
