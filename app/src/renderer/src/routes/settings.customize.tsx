import { createFileRoute } from '@tanstack/react-router'
import { ShortcutList } from '@renderer/components/settings/keyboard/ShortcutList'
import { SettingsContent } from '@renderer/components/settings/SettingsContent'
import SystemTheme from '@renderer/components/settings/appearance/system-theme'

export const Route = createFileRoute('/settings/customize')({
  component: KeyboardSettings
})

function KeyboardSettings() {
  return (
    <SettingsContent>
      <div className="flex flex-col gap-10">
        <div className="flex flex-col md:flex-row gap-4 justify-between">
          <h2 className="text-xl font-semibold p-2">Appearance</h2>
          <SystemTheme />
        </div>
        <div className="flex flex-col gap-4 justify-between">
          <h2 className="text-xl font-semibold p-2">Keyboard Shortcuts</h2>
          <ShortcutList />
        </div>
      </div>
    </SettingsContent>
  )
}
