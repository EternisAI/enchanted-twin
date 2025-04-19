import { createFileRoute, useNavigate } from '@tanstack/react-router'
import { useTheme } from '@renderer/lib/theme'
import { Button } from '@renderer/components/ui/button'
import { Monitor, Moon, Sun } from 'lucide-react'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { ConfirmDestructiveAction } from '@renderer/components/ui/confirm-destructive-action-dialog'

export const Route = createFileRoute('/settings')({
  component: Settings
})

function Settings() {
  const { theme, setTheme } = useTheme()
  // reset all zustand stores
  // reset onboarding store
  const resetOnboarding = useOnboardingStore((state) => state.resetOnboarding)
  const navigate = useNavigate()
  return (
    <div className="p-6 max-w-2xl mx-auto" style={{ viewTransitionName: 'page-content' }}>
      <style>
        {`
          :root {
            --direction: 1;
          }
        `}
      </style>
      <h2 className="text-2xl mb-6">Settings</h2>

      <div className="space-y-6">
        <div>
          <h3 className="text-lg font-medium mb-2">Appearance</h3>
          <p className="text-sm text-muted-foreground mb-4">
            Customize how the app looks on your device.
          </p>
          <div className="flex items-center gap-2">
            <Button
              variant={theme === 'light' ? 'default' : 'outline'}
              className="flex-1"
              onClick={() => setTheme('light')}
            >
              <Sun className="mr-2" />
              Light
            </Button>
            <Button
              variant={theme === 'dark' ? 'default' : 'outline'}
              className="flex-1"
              onClick={() => setTheme('dark')}
            >
              <Moon className="mr-2" />
              Dark
            </Button>
            <Button
              variant={theme === 'system' ? 'default' : 'outline'}
              className="flex-1"
              onClick={() => setTheme('system')}
            >
              <Monitor className="mr-2" />
              System
            </Button>
          </div>
        </div>
        <div>
          <h3 className="text-lg font-medium mb-2">Reset</h3>
          <p className="text-sm text-muted-foreground mb-4">
            Resets internal app state. This will remove all data from the app.{' '}
            <strong>THIS ONLY RESETS THE FRONT-END FOR NOW</strong>
          </p>
          <div className="flex items-center gap-2">
            <ConfirmDestructiveAction
              title="Reset"
              description="Resets internal app state. This will remove all data from the app."
              onConfirm={() => {
                resetOnboarding()
                navigate({ to: '/' })
              }}
              confirmText="Reset"
            />
          </div>
        </div>
      </div>
    </div>
  )
}
