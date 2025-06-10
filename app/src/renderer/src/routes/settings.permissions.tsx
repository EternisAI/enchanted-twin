import { createFileRoute } from '@tanstack/react-router'
import PermissionsCard from '@renderer/components/settings/permissions/PermissionsCard'

export const Route = createFileRoute('/settings/permissions')({
  component: PermissionsSettings
})

function PermissionsSettings() {
  return (
    <div className="p-8 max-w-3xl mx-auto">
      <div className="flex flex-col gap-4 w-full">
        <h3 className="text-xl font-semibold">Permissions</h3>
        <p className="text-sm text-muted-foreground mb-4">
          Manage your app&apos;s permissions to access your device&apos;s features.
        </p>
        <PermissionsCard />
      </div>
    </div>
  )
}
