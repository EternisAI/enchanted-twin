import { createFileRoute } from '@tanstack/react-router'
import PermissionsCard from '@renderer/components/settings/permissions/PermissionsCard'
import { SettingsContent } from '@renderer/components/settings/SettingsContent'
import { z } from 'zod'

const searchSchema = z.object({
  screenpipe: z.union([z.string(), z.boolean()]).optional()
})

export const Route = createFileRoute('/settings/permissions')({
  component: PermissionsSettings,
  validateSearch: searchSchema
})

function PermissionsSettings() {
  return (
    <SettingsContent>
      {/* <h1 className="text-2xl font-semibold">Permissions</h1> */}
      <PermissionsCard />
    </SettingsContent>
  )
}
