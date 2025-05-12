import { Dialog, DialogContent } from '@renderer/components/ui/dialog'
import { Monitor, Database, Settings2, Plug, Shield, RefreshCcw } from 'lucide-react'
import { DataSourcesPanel } from '@renderer/components/data-sources/DataSourcesPanel'
import MCPPanel from '@renderer/components/oauth/MCPPanel'
import { ScrollArea } from '@renderer/components/ui/scroll-area'
import { useSettingsStore } from '@renderer/lib/stores/settings'
import * as Tabs from '@radix-ui/react-tabs'
import PermissionsCard from './permissions/PermissionsCard'
import Versions from '../Versions'
import SystemTheme from './appearance/system-theme'
import AdminPanel from '../admin/AdminPanel'

export function SettingsDialog() {
  const { isOpen, close, activeTab, setActiveTab } = useSettingsStore()

  return (
    <Dialog open={isOpen} onOpenChange={close}>
      <DialogContent className="!max-w-[95vw] w-full h-[90vh] p-0 z-[100]">
        <div className="flex h-full w-full">
          <Tabs.Root
            value={activeTab}
            onValueChange={setActiveTab}
            className="flex h-full w-full"
            orientation="vertical"
          >
            <Tabs.List className="w-[240px] bg-muted/50 p-4">
              <Tabs.Trigger
                value="connections"
                className="flex items-center gap-2 w-full p-2 data-[state=active]:bg-accent rounded-md"
              >
                <Plug className="h-4 w-4" />
                Connections
              </Tabs.Trigger>
              <Tabs.Trigger
                value="import-data"
                className="flex items-center gap-2 w-full p-2 data-[state=active]:bg-accent rounded-md"
              >
                <Database className="h-4 w-4" />
                Import Data
              </Tabs.Trigger>
              <Tabs.Trigger
                value="permissions"
                className="flex items-center gap-2 w-full p-2 data-[state=active]:bg-accent rounded-md"
              >
                <Shield className="h-4 w-4" />
                Permissions
              </Tabs.Trigger>
              <Tabs.Trigger
                value="updates"
                className="flex items-center gap-2 w-full p-2 data-[state=active]:bg-accent rounded-md"
              >
                <RefreshCcw className="h-4 w-4" />
                Updates
              </Tabs.Trigger>
              <Tabs.Trigger
                value="appearance"
                className="flex items-center gap-2 w-full p-2 data-[state=active]:bg-accent rounded-md"
              >
                <Monitor className="h-4 w-4" />
                Appearance
              </Tabs.Trigger>
              <Tabs.Trigger
                value="advanced"
                className="flex items-center gap-2 w-full p-2 data-[state=active]:bg-accent rounded-md"
              >
                <Settings2 className="h-4 w-4" />
                Advanced
              </Tabs.Trigger>
            </Tabs.List>

            <div className="flex-1 relative">
              <Tabs.Content value="appearance" className="absolute inset-0">
                <ScrollArea className="h-full">
                  <div className="p-8 flex flex-col gap-4">
                    <h3 className="text-xl font-semibold">Appearance</h3>
                    <p className="text-sm text-muted-foreground">
                      Customize how the app looks on your device.
                    </p>
                    <SystemTheme />
                  </div>
                </ScrollArea>
              </Tabs.Content>

              <Tabs.Content value="connections" className="absolute inset-0">
                <ScrollArea className="h-full">
                  <div className="p-8 flex flex-col gap-4">
                    <h3 className="text-xl font-semibold">Connections</h3>
                    <p className="text-sm text-muted-foreground mb-4">
                      Connect your accounts to continually update your data.
                    </p>
                    <MCPPanel header={false} />
                  </div>
                </ScrollArea>
              </Tabs.Content>

              <Tabs.Content value="permissions" className="absolute inset-0">
                <ScrollArea className="h-full">
                  <div className="p-8 flex flex-col gap-4">
                    <h3 className="text-xl font-semibold">Permissions</h3>
                    <p className="text-sm text-muted-foreground mb-4">
                      Manage your app&apos;s permissions to access your device&apos;s features.
                    </p>
                    <PermissionsCard />
                  </div>
                </ScrollArea>
              </Tabs.Content>
              <Tabs.Content value="import-data" className="absolute inset-0">
                <ScrollArea className="h-full w-full items-center p-8 flex flex-col gap-4">
                  <DataSourcesPanel showStatus={true} />
                </ScrollArea>
              </Tabs.Content>
              <Tabs.Content value="updates" className="absolute inset-0 p-4">
                <ScrollArea className="h-full">
                  <Versions />
                </ScrollArea>
              </Tabs.Content>
              <Tabs.Content value="advanced" className="absolute inset-0">
                <ScrollArea className="h-full">
                  <div className="p-8 flex flex-col gap-4">
                    <h3 className="text-xl font-semibold">Advanced Settings</h3>
                    <p className="text-sm text-muted-foreground">
                      Configure advanced application settings and preferences.
                    </p>
                    <div className="mt-4 space-y-4 max-w-md">
                      <AdminPanel />
                    </div>
                  </div>
                </ScrollArea>
              </Tabs.Content>
            </div>
          </Tabs.Root>
        </div>
      </DialogContent>
    </Dialog>
  )
}
