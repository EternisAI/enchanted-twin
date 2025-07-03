import { createFileRoute } from '@tanstack/react-router'
import { DataSourcesPanel } from '@renderer/components/data-sources/DataSourcesPanel'

export const Route = createFileRoute('/settings/import-data')({
  component: ImportDataSettings
})

function ImportDataSettings() {
  return (
    <div className="p-8 max-w-4xl mx-auto">
      <div className="flex flex-col gap-4 w-full">
        <DataSourcesPanel showStatus={true} />
      </div>
    </div>
  )
}
