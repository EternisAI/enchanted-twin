import { createFileRoute } from '@tanstack/react-router'
import PermissionsCard from '@renderer/components/settings/permissions/PermissionsCard'
import { SettingsContent } from '@renderer/components/settings/SettingsContent'

export const Route = createFileRoute('/settings/permissions')({
  component: PermissionsSettings
})

function PermissionsSettings() {
  return (
    <SettingsContent>
      <h1 className="text-4xl font-semibold">Permissions</h1>
      <PermissionsCard />
    </SettingsContent>
  )
}
