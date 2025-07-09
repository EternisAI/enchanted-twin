import { createFileRoute } from '@tanstack/react-router'
import { ShortcutList } from '@renderer/components/settings/keyboard/ShortcutList'
import { SettingsContent } from '@renderer/components/settings/SettingsContent'
import SystemTheme from '@renderer/components/settings/appearance/system-theme'
import { Card } from '@renderer/components/ui/card'

export const Route = createFileRoute('/settings/customize')({
  component: KeyboardSettings
})

function KeyboardSettings() {
  return (
    <SettingsContent>
      <h1 className="text-4xl font-semibold">Customize</h1>
      <Card className="flex flex-col md:flex-row gap-4 justify-between p-4">
        <h2 className="text-2xl font-semibold p-2">Appearance</h2>
        <SystemTheme />
      </Card>
      <Card className="flex flex-col md:flex-row gap-4 justify-between p-4">
        <h2 className="text-2xl font-semibold p-2">Keyboard Shortcuts</h2>
        <ShortcutList />
      </Card>
    </SettingsContent>
  )
}
