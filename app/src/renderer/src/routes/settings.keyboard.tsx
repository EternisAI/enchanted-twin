import { createFileRoute } from '@tanstack/react-router'
import { ShortcutList } from '@renderer/components/settings/keyboard/ShortcutList'

export const Route = createFileRoute('/settings/keyboard')({
  component: KeyboardSettings
})

function KeyboardSettings() {
  return (
    <div className="p-8 max-w-3xl mx-auto">
      <div className="flex flex-col gap-4 w-full">
        <h3 className="text-xl font-semibold">Keyboard Shortcuts</h3>
        <p className="text-sm text-muted-foreground">Customize keyboard shortcuts to your preference.</p>
        <ShortcutList />
      </div>
    </div>
  )
}