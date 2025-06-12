import { createFileRoute } from '@tanstack/react-router'
import AdminPanel from '@renderer/components/admin/AdminPanel'
import InstallationStatus from '@renderer/components/InstallationStatus'
import GoLogsViewer from '@renderer/components/admin/GoLogsViewer'

export const Route = createFileRoute('/settings/advanced')({
  component: AdvancedSettings
})

function AdvancedSettings() {
  return (
    <div className="p-8">
      <div className="flex flex-col gap-4 w-full">
        <h3 className="text-xl font-semibold">Advanced Settings</h3>
        <p className="text-sm text-muted-foreground">
          Configure advanced application settings and preferences.
        </p>
        <div className="flex gap-4 w-full">
          <div className="mt-4 flex flex-col gap-4 max-w-md">
            <AdminPanel />
            <InstallationStatus />
          </div>
          <GoLogsViewer />
        </div>
      </div>
    </div>
  )
}
