/**
 * Formats keyboard shortcut strings for display in the UI
 * Converts keys like "CommandOrControl+S" to "⌘ S" on macOS or "Ctrl S" on other platforms
 */
export function formatShortcutForDisplay(shortcut: string): string {
  if (!shortcut) return ''
  
  const isMac = navigator.userAgent.toUpperCase().indexOf('MAC') >= 0
  const parts = shortcut.split('+')
  
  const symbols: Record<string, string> = {
    CommandOrControl: isMac ? '⌘' : 'Ctrl',
    Command: '⌘',
    Cmd: '⌘',
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
    Right: '→'
  }
  
  return parts.map((key) => symbols[key] || key.toUpperCase()).join(' ')
}