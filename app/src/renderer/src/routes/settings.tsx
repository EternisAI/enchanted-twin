import { createFileRoute } from '@tanstack/react-router'
import { Button } from '@renderer/components/ui/button'
import { ArrowLeft } from 'lucide-react'
import { SettingsPage } from '@renderer/components/settings/SettingsPage'

export const Route = createFileRoute('/settings')({
  component: SettingsRouteComponent
})

function SettingsRouteComponent() {
  return (
    <div className="flex flex-col h-screen w-screen text-foreground pt-8 relative bg-background">
      <div className="titlebar text-center fixed top-0 left-0 right-0 text-muted-foreground text-xs h-8 z-20 flex items-center justify-center backdrop-blur-sm drag">
        Settings
      </div>
      <div className="flex-1 flex flex-col mt-8 overflow-hidden">
        <div className="p-4 border-b no-drag">
          <Button variant="ghost" onClick={() => window.history.back()} className="h-9 px-2">
            <ArrowLeft className="w-4 h-4 mr-2" />
            Back
          </Button>
        </div>
        <div className="flex-1 overflow-y-auto">
          <SettingsPage />
        </div>
      </div>
    </div>
  )
}
