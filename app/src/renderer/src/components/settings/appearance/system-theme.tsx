import { Button } from '@renderer/components/ui/button'
import { useTheme } from '@renderer/lib/theme'
import { Sun, Moon, Monitor } from 'lucide-react'

export default function SystemTheme() {
  const { theme, setTheme } = useTheme()

  return (
    <div className="flex flex-row items-center gap-2">
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
  )
}
