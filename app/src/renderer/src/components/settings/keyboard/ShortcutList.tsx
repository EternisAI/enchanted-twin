import { useState, useEffect } from 'react'
import { Button } from '@renderer/components/ui/button'
import { ShortcutRecorder } from './ShortcutRecorder'
import { RotateCcw } from 'lucide-react'
import { toast } from 'sonner'

interface Shortcut {
  keys: string
  default: string
  global?: boolean
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
  },
  {
    action: 'newChat',
    label: 'New Chat',
    description: 'Create a new chat'
  },
  {
    action: 'toggleSidebar',
    label: 'Toggle Sidebar',
    description: 'Show or hide the sidebar'
  },
  {
    action: 'openSettings',
    label: 'Open Settings',
    description: 'Open the settings window'
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
        setShortcuts({})
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
            <div key={item.action} className="flex items-center justify-between py-3 rounded-lg">
              <div className="flex-1">
                <div className="flex items-center gap-2">
                  <div className="font-medium text-sm">{item.label}</div>
                  {shortcut?.global && (
                    <span className="text-xs px-1.5 py-0.5 rounded bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300">
                      Global
                    </span>
                  )}
                </div>
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

      <div className="pt-4">
        <Button variant="outline" size="sm" onClick={handleResetAll} className="w-full">
          <RotateCcw className="h-3.5 w-3.5 mr-2" />
          Reset All to Defaults
        </Button>
      </div>
    </div>
  )
}
