import { createFileRoute } from '@tanstack/react-router'
import { DataSourcesPanel } from '@renderer/components/data-sources/DataSourcesPanel'
import MCPPanel from '@renderer/components/oauth/MCPPanel'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@renderer/components/ui/tabs'
import ConnectMCPServerButton from '@renderer/components/oauth/MCPConnectServerButton'
import LocalFilesTab from '@renderer/components/data-sources/LocalFilesTab'
import { FolderIcon, NetworkIcon, PlugIcon } from 'lucide-react'

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
        <div className="flex flex-row gap-2 w-full items-center justify-between pb-10">
          <TabsList>
            <TabsTrigger value="available">
              <NetworkIcon className="w-4 h-4" /> Available
            </TabsTrigger>
            <TabsTrigger value="local-files">
              <FolderIcon className="w-4 h-4" /> Local Files
            </TabsTrigger>
            <TabsTrigger value="connected">
              <PlugIcon className="w-4 h-4" /> Connected
            </TabsTrigger>
          </TabsList>
          <ConnectMCPServerButton onSuccess={() => {}} />
        </div>
        <TabsContent value="available">
          <div className="flex flex-col gap-15">
            <MCPPanel />
            <DataSourcesPanel header={false} />
          </div>
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
