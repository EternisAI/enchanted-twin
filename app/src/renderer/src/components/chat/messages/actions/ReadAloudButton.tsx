import { useTTS } from '@renderer/hooks/useTTS'
import { Volume2, VolumeOff } from 'lucide-react'
import { ActionButton } from './ActionButton'

export function ReadAloudButton({ text }: { text: string }) {
  const { speak, stop, isSpeaking } = useTTS()
  return (
    <ActionButton
      onClick={isSpeaking ? stop : () => speak(text || '')}
      aria-label={isSpeaking ? 'Stop reading' : 'Read aloud'}
      tooltipLabel={isSpeaking ? 'Stop reading' : 'Read aloud'}
    >
      {isSpeaking ? (
        <VolumeOff className="h-4 w-4 text-primary" />
      ) : (
        <Volume2 className="h-4 w-4 text-primary" />
      )}
    </ActionButton>
  )
}
