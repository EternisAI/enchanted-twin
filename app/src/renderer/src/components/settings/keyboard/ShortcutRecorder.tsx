import { useState, useEffect, useCallback } from 'react'
import { Button } from '@renderer/components/ui/button'
import { cn } from '@renderer/lib/utils'
import { XCircle } from 'lucide-react'
import { toast } from 'sonner'

interface ShortcutRecorderProps {
  value: string
  onChange: (keys: string) => void
  onCancel?: () => void
  isRecording: boolean
  onStartRecording: () => void
  onStopRecording: () => void
}

export function ShortcutRecorder({
  value,
  onChange,
  onCancel,
  isRecording,
  onStartRecording,
  onStopRecording
}: ShortcutRecorderProps) {
  const [pressedKeys, setPressedKeys] = useState<Set<string>>(new Set())
  const [displayKeys, setDisplayKeys] = useState('')

  useEffect(() => {
    const keys = Array.from(pressedKeys)
    if (keys.length > 0) {
      setDisplayKeys(formatShortcut(keys))
    }
  }, [pressedKeys])

  const normalizeKey = useCallback((e: KeyboardEvent): string | null => {
    // Handle modifier keys
    if (e.key === 'Meta' || e.key === 'OS') return 'Meta'
    if (e.key === 'Control') return 'Control'
    if (e.key === 'Alt') return 'Alt'
    if (e.key === 'Shift') return 'Shift'

    // For dead keys or unidentified
    if (e.key === 'Dead' || e.key === 'Process' || e.key === 'Unidentified') {
      if (e.code && e.code.startsWith('Key')) {
        return e.code.replace('Key', '').toUpperCase()
      }
      return null
    }

    // Prefer e.code for letters and digits to get base key
    if (e.code.startsWith('Key')) {
      return e.code.replace('Key', '').toUpperCase()
    }
    if (e.code.startsWith('Digit')) {
      return e.code.replace('Digit', '')
    }

    // Handle special keys via code
    const codeMap: Record<string, string> = {
      Space: 'Space',
      ArrowUp: 'Up',
      ArrowDown: 'Down',
      ArrowLeft: 'Left',
      ArrowRight: 'Right',
      Escape: 'Esc',
      Tab: 'Tab',
      Enter: 'Enter',
      Backspace: 'Backspace',
      Delete: 'Delete'
    }

    if (codeMap[e.code]) {
      return codeMap[e.code]
    }

    // Fallback to e.key for other keys
    let key = e.key
    if (key.length === 1) {
      key = key.toUpperCase()
    }
    return key
  }, [])

  const buildShortcut = useCallback((keys: string[]): string => {
    const modifiers: string[] = []
    const regularKeys: string[] = []

    // On macOS, use CommandOrControl for Cmd key
    const isMac = navigator.userAgent.toUpperCase().indexOf('MAC') >= 0

    keys.forEach((key) => {
      if (isMac && (key === 'Cmd' || key === 'Command' || key === 'Meta')) {
        modifiers.push('CommandOrControl')
      } else if (!isMac && (key === 'Ctrl' || key === 'Control')) {
        modifiers.push('CommandOrControl')
      } else if (!isMac && key === 'Meta') {
        modifiers.push('Super')
      } else if (key === 'Alt' || key === 'Option') {
        modifiers.push('Alt')
      } else if (key === 'Shift') {
        modifiers.push('Shift')
      } else {
        regularKeys.push(key)
      }
    })

    // Build shortcut in correct order: CommandOrControl+Alt+Shift+Key
    const orderedModifiers: string[] = []
    if (modifiers.includes('CommandOrControl')) orderedModifiers.push('CommandOrControl')
    if (modifiers.includes('Alt')) orderedModifiers.push('Alt')
    if (modifiers.includes('Shift')) orderedModifiers.push('Shift')

    return [...orderedModifiers, ...regularKeys].join(' ')
  }, [])

  useEffect(() => {
    if (!isRecording) {
      setPressedKeys(new Set())
      setDisplayKeys('')
      return
    }

    const handleKeyDown = (e: KeyboardEvent) => {
      e.preventDefault()
      e.stopPropagation()

      // Handle Escape key to cancel recording
      if (e.key === 'Escape') {
        onCancel?.()
        onStopRecording()
        return
      }

      const key = normalizeKey(e)
      if (key) {
        setPressedKeys((prev) => {
          const newKeys = new Set([...prev])
          // Avoid duplicate modifiers
          if (!['Meta', 'Control', 'Alt', 'Shift'].includes(key) || !prev.has(key)) {
            newKeys.add(key)
          }
          return newKeys
        })
      }
    }

    const handleKeyUp = (e: KeyboardEvent) => {
      e.preventDefault()
      e.stopPropagation()

      // Don't process key up for Escape
      if (e.key === 'Escape') {
        return
      }

      // Use callback to access current state value
      setPressedKeys((currentPressedKeys) => {
        const currentKeys = Array.from(currentPressedKeys)
        if (currentKeys.length > 0) {
          const shortcut = buildShortcut(currentKeys)

          // Validate shortcut: must have at least one non-modifier key
          const parts = shortcut.split('+')
          const modifiersSet = new Set(['CommandOrControl', 'Alt', 'Shift', 'Super'])
          const hasNonModifier = parts.some((p) => !modifiersSet.has(p))

          if (!hasNonModifier || parts.length === 0) {
            onStopRecording()
            toast.error('Invalid shortcut: Must include at least one non-modifier key')
            return currentPressedKeys
          }

          onChange(shortcut)
          onStopRecording()
        }
        return currentPressedKeys
      })
    }

    window.addEventListener('keydown', handleKeyDown)
    window.addEventListener('keyup', handleKeyUp)

    return () => {
      window.removeEventListener('keydown', handleKeyDown)
      window.removeEventListener('keyup', handleKeyUp)
    }
  }, [isRecording, onChange, onStopRecording, buildShortcut, normalizeKey, onCancel])

  const formatShortcut = (keys: string[]): string => {
    const isMac = navigator.userAgent.toUpperCase().indexOf('MAC') >= 0
    const symbols: Record<string, string> = {
      CommandOrControl: isMac ? '⌘' : 'Ctrl',
      Command: '⌘',
      Cmd: '⌘',
      Meta: isMac ? '⌘' : 'Ctrl',
      Control: 'Ctrl',
      Ctrl: 'Ctrl',
      Alt: isMac ? '⌥' : 'Alt',
      Option: '⌥',
      Shift: '⇧',
      Space: 'Space',
      Enter: '↵',
      Backspace: '⌫',
      Delete: '⌦',
      Tab: '⇥',
      Escape: 'Esc',
      Esc: 'Esc',
      Up: '↑',
      Down: '↓',
      Left: '←',
      Right: '→',
      Meta: isMac ? '⌘' : 'Super',
      Super: 'Super'
    }

    return keys.map((key) => symbols[key] || key).join(' ')
  }

  const formatDisplayValue = (shortcut: string): string => {
    // Handle both '+' and space separators for backward compatibility
    const parts = shortcut.includes('+') ? shortcut.split('+') : shortcut.split(' ')
    return formatShortcut(parts)
  }

  const handleClick = () => {
    if (isRecording) {
      onCancel?.()
      onStopRecording()
    } else {
      onStartRecording()
    }
  }

  return (
    <div className="flex items-center gap-1">
      <Button
        variant={isRecording ? 'default' : 'outline'}
        size="sm"
        className={cn('min-w-[120px] font-mono text-sm', isRecording && 'animate-pulse')}
        onClick={handleClick}
      >
        {isRecording ? (
          displayKeys || 'Press keys...'
        ) : value ? (
          <kbd>{formatDisplayValue(value)}</kbd>
        ) : (
          'Not set'
        )}
      </Button>
      {value && !isRecording && (
        <Button
          variant="ghost"
          size="icon"
          className="h-8 w-8 hover:bg-destructive/10"
          onClick={() => onChange('')}
          title="Remove shortcut"
        >
          <XCircle className="h-4 w-4 text-muted-foreground hover:text-destructive" />
        </Button>
      )}
    </div>
  )
}
