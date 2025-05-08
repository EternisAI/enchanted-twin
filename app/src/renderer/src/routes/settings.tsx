import { createFileRoute } from '@tanstack/react-router'
import { useTheme } from '@renderer/lib/theme'
import { Button } from '@renderer/components/ui/button'
import { Monitor, Moon, Sun } from 'lucide-react'
import { Card } from '@renderer/components/ui/card'
import PermissionsCard from '@renderer/components/settings/permissions/PermissionsCard'
import useAppVersion from '@renderer/hooks/useAppVersion'
import ScreenpipePanel from '@renderer/components/settings/permissions/ScreenpipeCard'

export const Route = createFileRoute('/settings')({
  component: Settings
})

function Settings() {
  const { theme, setTheme } = useTheme()
  const { version } = useAppVersion()

  return (
    <div className="p-6 flex flex-col gap-6 w-full mx-auto">
      <h2 className="text-4xl mb-6">Settings</h2>

      <div className="grid grid-cols-2 gap-6">
        <div className="flex flex-col gap-8">
          <Card className="p-6 w-full">
            <h3 className="text-xl font-semibold">Appearance</h3>
            <p className="text-sm text-muted-foreground">
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
          </Card>

          <Card className="p-6 w-full">
            <h3 className="text-xl font-semibold">Updates</h3>
            <p className="text-sm text-muted-foreground">Version {version}</p>
          </Card>

          <ScreenpipePanel />
        </div>

        <div>
          <PermissionsCard />
        </div>
      </div>
    </div>
  )
}
