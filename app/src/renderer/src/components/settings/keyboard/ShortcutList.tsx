import { useState, useEffect } from 'react'
import { Button } from '@renderer/components/ui/button'
import { ShortcutRecorder } from './ShortcutRecorder'
import { RotateCcw } from 'lucide-react'
import { toast } from 'sonner'

interface Shortcut {
  keys: string
  default: string
}

interface ShortcutItem {
  action: string
  label: string
  description: string
}

const SHORTCUT_ITEMS: ShortcutItem[] = [
  {
    action: 'toggleOmnibar',
    label: 'Global Omnibar',
    description: 'Show or hide the global omnibar overlay'
  }
]

export function ShortcutList() {
  const [shortcuts, setShortcuts] = useState<Record<string, Shortcut>>({})
  const [recordingAction, setRecordingAction] = useState<string | null>(null)

  useEffect(() => {
    loadShortcuts()
  }, [])

  const loadShortcuts = async () => {
    try {
      const data = await window.api.keyboardShortcuts.get()
      console.log('loadShortcuts', data)
      if (data && typeof data === 'object') {
        setShortcuts(data)
      } else {
        console.error('Invalid shortcuts data received:', data)
        // Set default shortcuts if data is invalid
        setShortcuts({
          toggleOmnibar: {
            keys: 'CommandOrControl+Alt+O',
            default: 'CommandOrControl+Alt+O'
          }
        })
      }
    } catch (error) {
      console.error('Failed to load shortcuts:', error)
      toast.error('Failed to load keyboard shortcuts')
    }
  }

  const handleShortcutChange = async (action: string, keys: string) => {
    try {
      const result = await window.api.keyboardShortcuts.set(action, keys)
      if (result.success) {
        toast.success(`Shortcut updated: ${action} has been set to ${keys}`)
        loadShortcuts()
      } else {
        toast.error(`Failed to update shortcut: ${result.error}`)
      }
    } catch (error) {
      toast.error(`Failed to update shortcut: ${error}`)
    }
  }

  const handleReset = async (action: string) => {
    try {
      const result = await window.api.keyboardShortcuts.reset(action)
      if (result.success) {
        toast.success(`Shortcut reset: ${action} has been reset to default`)
        loadShortcuts()
      } else {
        toast.error(`Failed to reset shortcut: ${result.error}`)
      }
    } catch (error) {
      toast.error(`Failed to reset shortcut: ${error}`)
    }
  }

  const handleResetAll = async () => {
    try {
      const result = await window.api.keyboardShortcuts.resetAll()
      if (result.success) {
        toast.success('All shortcuts reset')
        loadShortcuts()
      } else {
        toast.error(`Failed to reset shortcuts: ${result.error}`)
      }
    } catch (error) {
      toast.error(`Failed to reset shortcuts: ${error}`)
    }
  }

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        {SHORTCUT_ITEMS.map((item) => {
          const shortcut = shortcuts[item.action]
          const isRecording = recordingAction === item.action
          const isModified = shortcut && shortcut.keys !== shortcut.default

          return (
            <div
              key={item.action}
              className="flex items-center justify-between py-3 px-4 rounded-lg border bg-card"
            >
              <div className="flex-1">
                <div className="font-medium text-sm">{item.label}</div>
                <div className="text-xs text-muted-foreground">{item.description}</div>
              </div>
              <div className="flex items-center gap-2">
                <ShortcutRecorder
                  value={shortcut?.keys || ''}
                  onChange={(keys) => handleShortcutChange(item.action, keys)}
                  onCancel={() => setRecordingAction(null)}
                  isRecording={isRecording}
                  onStartRecording={() => setRecordingAction(item.action)}
                  onStopRecording={() => setRecordingAction(null)}
                />
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  onClick={() => handleReset(item.action)}
                  disabled={!isModified}
                >
                  <RotateCcw className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
          )
        })}
      </div>

      <div className="pt-4 border-t">
        <Button variant="outline" size="sm" onClick={handleResetAll} className="w-full">
          <RotateCcw className="h-3.5 w-3.5 mr-2" />
          Reset All to Defaults
        </Button>
      </div>
    </div>
  )
}
