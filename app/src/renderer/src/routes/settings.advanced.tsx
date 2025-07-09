import { createFileRoute } from '@tanstack/react-router'
import AdminPanel from '@renderer/components/admin/AdminPanel'
import InstallationStatus from '@renderer/components/InstallationStatus'
import GoLogsViewer from '@renderer/components/admin/GoLogsViewer'
import { SettingsContent } from '@renderer/components/settings/SettingsContent'

export const Route = createFileRoute('/settings/advanced')({
  component: AdvancedSettings
})

function AdvancedSettings() {
  return (
    <SettingsContent className="max-w-full w-fit">
      <div className="flex flex-col gap-4 w-full">
        <h3 className="text-xl font-semibold">Advanced Settings</h3>
        <p className="text-sm text-muted-foreground">
          Configure advanced application settings and preferences.
        </p>
        <div className="flex flex-wrap gap-4 w-full">
          <div className="mt-4 flex flex-col gap-4 max-w-md">
            <AdminPanel />
            <InstallationStatus />
          </div>
          <GoLogsViewer />
        </div>
      </div>
    </SettingsContent>
  )
}
