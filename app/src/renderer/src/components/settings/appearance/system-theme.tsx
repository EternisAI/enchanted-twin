import { Button } from '@renderer/components/ui/button'
import { useTheme } from '@renderer/lib/theme'
import { Sun, Moon, Wand2Icon } from 'lucide-react'

export default function SystemTheme() {
  const { theme, setTheme } = useTheme()

  return (
    <div className="flex flex-row items-center gap-2">
      <Button
        variant={theme === 'light' ? 'default' : 'outline'}
        className="flex-1"
        onClick={() => setTheme('light')}
      >
        <Sun />
        Light
      </Button>
      <Button
        variant={theme === 'dark' ? 'default' : 'outline'}
        className="flex-1"
        onClick={() => setTheme('dark')}
      >
        <Moon />
        Dark
      </Button>
      <Button
        variant={theme === 'system' ? 'default' : 'outline'}
        className="flex-1"
        onClick={() => setTheme('system')}
      >
        <Wand2Icon />
        Auto
      </Button>
    </div>
  )
}
