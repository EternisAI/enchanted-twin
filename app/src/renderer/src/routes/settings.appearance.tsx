import { createFileRoute } from '@tanstack/react-router'
import SystemTheme from '@renderer/components/settings/appearance/system-theme'

export const Route = createFileRoute('/settings/appearance')({
  component: AppearanceSettings
})

function AppearanceSettings() {
  return (
    <div className="p-8 max-w-3xl mx-auto">
      <div className="flex flex-col gap-4 w-full">
        <h3 className="text-xl font-semibold">Appearance</h3>
        <p className="text-sm text-muted-foreground">Customize how the app looks on your device.</p>
        <SystemTheme />
      </div>
    </div>
  )
}
