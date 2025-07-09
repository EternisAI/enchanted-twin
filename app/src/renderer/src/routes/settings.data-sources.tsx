import { createFileRoute } from '@tanstack/react-router'
import { DataSourcesPanel } from '@renderer/components/data-sources/DataSourcesPanel'
import MCPPanel from '@renderer/components/oauth/MCPPanel'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@renderer/components/ui/tabs'
import ConnectMCPServerButton from '@renderer/components/oauth/MCPConnectServerButton'
import LocalFilesTab from '@renderer/components/data-sources/LocalFilesTab'
import { FolderIcon, NetworkIcon, PlugIcon } from 'lucide-react'
import LocalFolderSync from '@renderer/components/data-sources/LocalFolderSync'

export const Route = createFileRoute('/settings/data-sources')({
  component: ImportDataSettings
})

function ImportDataSettings() {
  return (
    <div className="p-4 sm:p-8 w-full flex flex-col items-center justify-center">
      <div className="flex flex-col w-full max-w-3xl">
        <Content />
      </div>
    </div>
  )
}

// Tabs: Available, Local Files, Connected

function Content() {
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-row gap-2 w-full items-center justify-between">
        <h2 className="text-4xl font-semibold">Data Sources</h2>
        <div className="block sm:hidden">
          <ConnectMCPServerButton onSuccess={() => {}} />
        </div>
      </div>
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
          <div className="hidden sm:block">
            <ConnectMCPServerButton onSuccess={() => {}} />
          </div>
        </div>
        <TabsContent value="available">
          <div className="flex flex-col gap-15">
            <MCPPanel />
            <header className="flex flex-col gap-2 border-b pb-3">
              <h2 className="text-2xl font-bold leading-none">Imports & Takeouts</h2>
            </header>
            <DataSourcesPanel />
          </div>
        </TabsContent>
        <TabsContent value="local-files">
          <div className="flex flex-col gap-15">
            <h2 className="text-2xl font-semibold">Local Files</h2>
            <LocalFilesTab />
            <h2 className="text-2xl font-semibold">Synced Folders</h2>
            <LocalFolderSync />
          </div>
        </TabsContent>
        <TabsContent value="connected">
          <DataSourcesPanel header={false} />
        </TabsContent>
      </Tabs>
    </div>
  )
}
