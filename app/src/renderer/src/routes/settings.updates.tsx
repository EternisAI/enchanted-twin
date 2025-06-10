import { createFileRoute } from '@tanstack/react-router'
import Versions from '@renderer/components/Versions'

export const Route = createFileRoute('/settings/updates')({
  component: UpdatesSettings
})

function UpdatesSettings() {
  return (
    <div className="p-8 max-w-3xl mx-auto">
      <div className="flex flex-col gap-4 w-full">
        <h3 className="text-xl font-semibold">Updates</h3>
        <p className="text-sm text-muted-foreground mb-4">
          Check for updates and manage your app&apos;s version.
        </p>
        <Versions />
      </div>
    </div>
  )
}
