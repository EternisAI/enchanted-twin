import { createFileRoute } from '@tanstack/react-router'
import { DataSourcesPanel } from '@renderer/components/data-sources/DataSourcesPanel'
import MCPPanel from '@renderer/components/oauth/MCPPanel'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@renderer/components/ui/tabs'
import ConnectMCPServerButton from '@renderer/components/oauth/MCPConnectServerButton'
import { FolderSyncIcon, NetworkIcon, PlugIcon } from 'lucide-react'
import LocalFolderSync from '@renderer/components/data-sources/LocalFolderSync'
import { SettingsContent } from '@renderer/components/settings/SettingsContent'

export const Route = createFileRoute('/settings/data-sources')({
  component: ImportDataSettings
})

function ImportDataSettings() {
  return (
    <SettingsContent>
      <Tabs defaultValue="available">
        <div className="flex flex-row gap-2 w-full items-center justify-between">
          <h2 className="text-4xl font-semibold">Data Sources</h2>
          <div className="block md:hidden">
            <ConnectMCPServerButton onSuccess={() => {}} />
          </div>
        </div>
        <div className="flex flex-row gap-2 w-full items-center justify-between pt-5 pb-10">
          <TabsList>
            <TabsTrigger value="available">
              <NetworkIcon className="w-4 h-4" /> Available
            </TabsTrigger>
            <TabsTrigger value="local-files">
              <FolderSyncIcon className="w-4 h-4" /> Synced Folders
            </TabsTrigger>
            <TabsTrigger value="connected">
              <PlugIcon className="w-4 h-4" /> Connected
            </TabsTrigger>
          </TabsList>
          <div className="hidden md:block">
            <ConnectMCPServerButton onSuccess={() => {}} />
          </div>
        </div>
        <TabsContent value="available">
          <div className="flex flex-col gap-15">
            <MCPPanel />
            <DataSourcesPanel />
          </div>
        </TabsContent>
        <TabsContent value="local-files">
          <LocalFolderSync />
        </TabsContent>
        <TabsContent value="connected">
          <DataSourcesPanel header={false} />
        </TabsContent>
      </Tabs>
    </SettingsContent>
  )
}
