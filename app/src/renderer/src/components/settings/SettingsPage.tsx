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

const settingsTabs = [
  {
    value: 'connections',
    label: 'Connections',
    icon: Plug,
    content: (
      <>
        <h3 className="text-xl font-semibold">Connections</h3>
        <p className="text-sm text-muted-foreground mb-4">
          Connect your accounts to continually update your data.
        </p>
        <MCPPanel header={false} />
      </>
    ),
    fullWidth: false
  },
  {
    value: 'import-data',
    label: 'Import Data',
    icon: Database,
    content: <DataSourcesPanel showStatus={true} />,
    fullWidth: false
  },
  {
    value: 'permissions',
    label: 'Permissions',
    icon: Shield,
    content: (
      <>
        <h3 className="text-xl font-semibold">Permissions</h3>
        <p className="text-sm text-muted-foreground mb-4">
          Manage your app&apos;s permissions to access your device&apos;s features.
        </p>
        <PermissionsCard />
      </>
    ),
    fullWidth: false
  },
  {
    value: 'updates',
    label: 'Updates',
    icon: RefreshCcw,
    content: (
      <>
        <h3 className="text-xl font-semibold">Updates</h3>
        <p className="text-sm text-muted-foreground mb-4">
          Check for updates and manage your app&apos;s version.
        </p>
        <Versions />
      </>
    ),
    fullWidth: false
  },
  {
    value: 'appearance',
    label: 'Appearance',
    icon: Monitor,
    content: (
      <>
        <h3 className="text-xl font-semibold">Appearance</h3>
        <p className="text-sm text-muted-foreground">Customize how the app looks on your device.</p>
        <SystemTheme />
      </>
    ),
    fullWidth: false
  },
  {
    value: 'advanced',
    label: 'Advanced',
    icon: Settings2,
    content: (
      <>
        <h3 className="text-xl font-semibold">Advanced Settings</h3>
        <p className="text-sm text-muted-foreground">
          Configure advanced application settings and preferences.
        </p>
        <div className="mt-4 space-y-4 max-w-md">
          <AdminPanel />
        </div>
      </>
    ),
    fullWidth: false
  }
]

// Renamed from SettingsDialog to SettingsPage
// Removed Dialog, DialogTitle, DialogContent wrappers
export function SettingsPage() {
  const { activeTab, setActiveTab } = useSettingsStore() // Removed isOpen and close

  return (
    // Removed Dialog and DialogContent, using a simple div for page structure.
    // The overall page structure (like padding, max-width) will now be controlled
    // by the route component (SettingsRouteComponent) and this component.
    <div className="flex h-full w-full">
      {' '}
      {/* Was previously DialogContent className="!max-w-[95vw] w-full h-[90vh] p-0" */}
      <Tabs.Root
        value={activeTab}
        onValueChange={setActiveTab}
        className="flex h-full w-full" // Ensure this takes full height/width within its container
        orientation="vertical"
      >
        {/* Tabs.List no longer needs a back button here, as it's in SettingsRouteComponent */}
        <Tabs.List className="w-[240px] bg-muted/50 p-4 flex flex-col gap-1 border-r">
          {' '}
          {/* Added border-r for separation */}
          {settingsTabs.map((tab) => (
            <Tabs.Trigger
              key={tab.value}
              value={tab.value}
              className="flex items-center gap-2 w-full p-2 data-[state=active]:bg-accent rounded-md justify-start text-sm"
            >
              <tab.icon className="h-4 w-4" />
              {tab.label}
            </Tabs.Trigger>
          ))}
        </Tabs.List>

        <div className="flex-1 relative w-full">
          {' '}
          {/* Ensure this takes remaining width */}
          {settingsTabs.map((tab) => (
            <Tabs.Content
              key={tab.value}
              value={tab.value}
              className="absolute inset-0 outline-none focus:ring-0 transition-opacity duration-300 ease-in-out data-[state=active]:opacity-100 data-[state=inactive]:opacity-0 data-[state=inactive]:pointer-events-none"
            >
              <ScrollArea className="h-full">
                {' '}
                {/* Ensure ScrollArea takes full height of its container */}
                <div
                  className={`p-8 ${tab.fullWidth ? '' : 'max-w-3xl mx-auto'}`} // Simplified layout, removed flex justify-center and min-h-full as ScrollArea handles height
                >
                  <div className={`flex flex-col gap-4 ${tab.fullWidth ? 'w-full' : 'w-full'}`}>
                    {' '}
                    {/* max-w-3xl is handled by parent now */}
                    {tab.content}
                  </div>
                </div>
              </ScrollArea>
            </Tabs.Content>
          ))}
        </div>
      </Tabs.Root>
    </div>
  )
}
