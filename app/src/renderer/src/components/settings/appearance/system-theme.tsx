import { Button } from '@renderer/components/ui/button'
import { Card } from '@renderer/components/ui/card'
import { useTheme } from '@renderer/lib/theme'
import { Sun, Moon, Monitor } from 'lucide-react'

export default function SystemTheme() {
  const { theme, setTheme } = useTheme()

  return (
    <Card className="flex items-center gap-2 mt-4 max-w-4xl">
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
    </Card>
  )
}
