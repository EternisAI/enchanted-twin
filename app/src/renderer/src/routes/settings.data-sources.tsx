import { createFileRoute } from '@tanstack/react-router'
import { DataSourcesPanel } from '@renderer/components/data-sources/DataSourcesPanel'
import MCPPanel from '@renderer/components/oauth/MCPPanel'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@renderer/components/ui/tabs'
import ConnectMCPServerButton from '@renderer/components/oauth/MCPConnectServerButton'
import LocalFilesTab from '@renderer/components/data-sources/LocalFilesTab'

export const Route = createFileRoute('/settings/data-sources')({
  component: ImportDataSettings
})

function ImportDataSettings() {
  return (
    <div className="p-8 max-w-4xl mx-auto">
      <div className="flex flex-col gap-4 w-full">
        <Header />
      </div>
    </div>
  )
}

// Tabs: Available, Local Files, Connected

function Header() {
  return (
    <div className="flex flex-col gap-4">
      <h2 className="text-4xl font-semibold">Data Sources</h2>
      <Tabs defaultValue="available">
        <div className="flex flex-row gap-2 w-full justify-between">
          <TabsList>
            <TabsTrigger value="available">Available</TabsTrigger>
            <TabsTrigger value="local-files">Local Files</TabsTrigger>
            <TabsTrigger value="connected">Connected</TabsTrigger>
          </TabsList>
          <ConnectMCPServerButton onSuccess={() => {}} />
        </div>
        <TabsContent value="available">
          <MCPPanel header={false} />
          <DataSourcesPanel header={false} />
        </TabsContent>
        <TabsContent value="local-files">
          <LocalFilesTab />
        </TabsContent>
        <TabsContent value="connected">
          <DataSourcesPanel header={false} />
        </TabsContent>
      </Tabs>
    </div>
  )
}
