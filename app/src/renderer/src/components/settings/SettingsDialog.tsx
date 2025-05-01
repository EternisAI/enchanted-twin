import { Dialog, DialogContent } from '@renderer/components/ui/dialog'
import { useTheme } from '@renderer/lib/theme'
import { Button } from '@renderer/components/ui/button'
import { Monitor, Moon, Sun, Database, Settings2, Plug } from 'lucide-react'
import { DataSourcesPanel } from '@renderer/components/data-sources/DataSourcesPanel'
import MCPPanel from '@renderer/components/oauth/MCPPanel'
import { ScrollArea } from '@renderer/components/ui/scroll-area'
import { useSettingsStore } from '@renderer/lib/stores/settings'
import * as Tabs from '@radix-ui/react-tabs'

export function SettingsDialog() {
  const { isOpen, close } = useSettingsStore()
  const { theme, setTheme } = useTheme()

  return (
    <Dialog open={isOpen} onOpenChange={close}>
      <DialogContent className="!max-w-[95vw] w-full h-[80vh] p-0 z-[100]">
        <div className="flex h-full w-full">
          <Tabs.Root
            defaultValue="appearance"
            className="flex h-full w-full"
            orientation="vertical"
          >
            <Tabs.List className="w-[240px] border-r bg-muted/50 p-4">
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
                    <div className="flex items-center gap-2 mt-4">
                      <Button
                        variant={theme === 'light' ? 'default' : 'outline'}
                        className="flex-1"
                        onClick={() => setTheme('light')}
                      >
                        <Sun className="mr-2" />
                        Light
                      </Button>
                      <Button
                        variant={theme === 'dark' ? 'default' : 'outline'}
                        className="flex-1"
                        onClick={() => setTheme('dark')}
                      >
                        <Moon className="mr-2" />
                        Dark
                      </Button>
                      <Button
                        variant={theme === 'system' ? 'default' : 'outline'}
                        className="flex-1"
                        onClick={() => setTheme('system')}
                      >
                        <Monitor className="mr-2" />
                        System
                      </Button>
                    </div>
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

              <Tabs.Content value="import-data" className="absolute inset-0">
                <ScrollArea className="h-full">
                  <div className="p-8 flex flex-col gap-4">
                    <h3 className="text-xl font-semibold">Import Data</h3>
                    <p className="text-sm text-muted-foreground mb-4">
                      Manage your data sources and import data.
                    </p>
                    <DataSourcesPanel showStatus={true} />
                  </div>
                </ScrollArea>
              </Tabs.Content>

              <Tabs.Content value="advanced" className="absolute inset-0">
                <ScrollArea className="h-full">
                  <div className="p-8 flex flex-col gap-4">
                    <h3 className="text-xl font-semibold">Advanced Settings</h3>
                    <p className="text-sm text-muted-foreground">
                      Configure advanced application settings and preferences.
                    </p>
                    <div className="mt-4 space-y-4">
                      <Button
                        variant="outline"
                        className="w-full justify-start"
                        onClick={() => window.api.openLogsFolder()}
                      >
                        Open Logs Folder
                      </Button>
                      <Button
                        variant="outline"
                        className="w-full justify-start"
                        onClick={() => window.api.openAppDataFolder()}
                      >
                        Open Application Folder
                      </Button>
                      <Button
                        variant="destructive"
                        className="w-full justify-start"
                        onClick={() => window.api.deleteAppData()}
                      >
                        Delete App Data
                      </Button>
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
