import { createFileRoute } from '@tanstack/react-router'
import { useTheme } from '@renderer/lib/theme'
import { Button } from '@renderer/components/ui/button'
import { Monitor, Moon, Sun } from 'lucide-react'

export const Route = createFileRoute('/settings')({
  component: Settings
})

function Settings() {
  const { theme, setTheme } = useTheme()

  return (
    <div
      className="p-6 flex flex-col gap-6 max-w-2xl mx-auto"
      style={{ viewTransitionName: 'page-content' }}
    >
      <style>
        {`
          :root {
            --direction: 1;
          }
        `}
      </style>
      <h2 className="text-4xl mb-6">Settings</h2>

      <div className="space-y-6">
        <div>
          <h3 className="text-xl font-medium mb-2">Appearance</h3>
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
      </div>
    </div>
  )
}
