import { createFileRoute } from '@tanstack/react-router'
import Versions from '@renderer/components/Versions'
import { SettingsContent } from '@renderer/components/settings/SettingsContent'

export const Route = createFileRoute('/settings/about')({
  component: AboutSettings
})

function AboutSettings() {
  return (
    <SettingsContent>
      {/* <h1 className="text-4xl font-semibold">About</h1> */}
      <Versions />
    </SettingsContent>
  )
}
