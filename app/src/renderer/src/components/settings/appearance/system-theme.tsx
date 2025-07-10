import { Button } from '@renderer/components/ui/button'
import { Card } from '@renderer/components/ui/card'
import { useTheme } from '@renderer/lib/theme'
import { Sun, Moon, Wand2Icon } from 'lucide-react'

export default function SystemTheme() {
  const { theme, setTheme } = useTheme()

  return (
    <Card className="flex flex-row items-center gap-2 mt-4 max-w-4xl p-4">
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
    </Card>
  )
}
