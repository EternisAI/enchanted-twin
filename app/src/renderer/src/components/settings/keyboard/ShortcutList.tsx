import { useState, useEffect } from 'react'
import { Button } from '@renderer/components/ui/button'
import { ShortcutRecorder } from './ShortcutRecorder'
import { CircleAlertIcon, CircleCheck, KeyboardOff, RotateCcw } from 'lucide-react'
import { toast } from 'sonner'
import { formatShortcutForDisplay } from '@renderer/lib/utils/shortcuts'

interface Shortcut {
  keys: string
  default: string
  global?: boolean
}

interface ShortcutItem {
  action: string
  label: string
  description?: string
}

const SHORTCUT_ITEMS: ShortcutItem[] = [
  {
    action: 'toggleOmnibar',
    label: 'Global Omnibar',
    description: 'Access Enchanted from anywhere'
  },
  {
    action: 'newChat',
    label: 'New Chat'
  },
  {
    action: 'toggleSidebar',
    label: 'Show/Hide Sidebar'
  },
  {
    action: 'openSettings',
    label: 'Open Settings'
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
      if (data && typeof data === 'object') {
        setShortcuts(data)
      } else {
        setShortcuts({})
      }
    } catch {
      toast.error('Failed to load keyboard shortcuts')
    }
  }

  const handleShortcutChange = async (item: ShortcutItem, keys: string) => {
    try {
      const result = await window.api.keyboardShortcuts.set(item.action, keys)
      const isRemoved = keys === ''
      if (result.success) {
        toast(`${item.label} shortcut ${isRemoved ? 'removed' : 'updated'}`, {
          icon: isRemoved ? (
            <KeyboardOff className="h-4 w-4" />
          ) : (
            <CircleCheck className="h-4 w-4 text-green-500" />
          ),
          description: isRemoved ? undefined : (
            <kbd className="text-sm font-medium">{formatShortcutForDisplay(keys)}</kbd>
          )
        })
        loadShortcuts()
      } else {
        toast('Invalid shortcut', {
          description: (
            <kbd className="text-sm text-red-500 font-medium">{formatShortcutForDisplay(keys)}</kbd>
          )
        })
      }
    } catch {
      toast('Failed to update shortcut', {
        icon: <CircleAlertIcon className="h-4 w-4 text-red-500" />
      })
    }
  }

  const handleReset = async (item: ShortcutItem) => {
    try {
      const result = await window.api.keyboardShortcuts.reset(item.action)
      if (result.success) {
        toast(`${item.label} shortcut reset`, {
          icon: <CircleCheck className="h-4 w-4 text-green-500" />,
          description: (
            <kbd className="text-sm font-medium">
              {formatShortcutForDisplay(shortcuts[item.action].default)}
            </kbd>
          )
        })
        loadShortcuts()
      } else {
        toast.error(`Failed to reset shortcut`)
      }
    } catch {
      toast.error(`Failed to reset shortcut`)
    }
  }

  const handleResetAll = async () => {
    try {
      const result = await window.api.keyboardShortcuts.resetAll()
      if (result.success) {
        toast.success('All shortcuts have been reset')
        loadShortcuts()
      } else {
        toast.error(`Failed to reset shortcuts: ${result.error}`)
      }
    } catch (error) {
      toast.error(`Failed to reset shortcuts: ${error}`)
    }
  }

  return (
    <div className="space-y-2">
      <div className="space-y-2">
        {SHORTCUT_ITEMS.map((item) => {
          const shortcut = shortcuts[item.action]
          const isRecording = recordingAction === item.action
          const isModified = shortcut && shortcut.keys !== shortcut.default

          return (
            <div
              key={item.action}
              className="flex items-center justify-between p-3 rounded-lg hover:bg-muted focus-within:bg-muted"
            >
              <div className="flex-1">
                <div className="flex items-center gap-2">
                  <div className="font-medium text-base">{item.label}</div>
                  {shortcut?.global && (
                    <span className="text-sm px-1.5 py-0.5 rounded bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300">
                      Global
                    </span>
                  )}
                </div>
                {item.description && (
                  <div className="text-xs text-muted-foreground">{item.description}</div>
                )}
              </div>
              <div className="flex items-center gap-2">
                <ShortcutRecorder
                  value={shortcut?.keys || ''}
                  onChange={(keys) => handleShortcutChange(item, keys)}
                  onCancel={() => setRecordingAction(null)}
                  isRecording={isRecording}
                  onStartRecording={() => setRecordingAction(item.action)}
                  onStopRecording={() => setRecordingAction(null)}
                />
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-8 w-8"
                  onClick={() => handleReset(item)}
                  disabled={!isModified}
                >
                  <RotateCcw className="h-3.5 w-3.5" />
                </Button>
              </div>
            </div>
          )
        })}
      </div>

      <div className="pt-4 flex justify-end">
        <Button className="text-destructive" variant="outline" size="sm" onClick={handleResetAll}>
          <RotateCcw className="h-3.5 w-3.5 mr-2" />
          Reset All Shortcuts
        </Button>
      </div>
    </div>
  )
}
