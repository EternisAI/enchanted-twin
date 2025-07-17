import { Eye, EyeOff } from 'lucide-react'
import { Button } from '../ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '../ui/tooltip'
import { memo } from 'react'
import { cn } from '@renderer/lib/utils'

interface AnonToggleButtonProps {
  isAnonymized: boolean
  setIsAnonymized: (isAnonymized: boolean) => void
}

export const AnonToggleButton = memo(function AnonToggleButton({
  isAnonymized,
  setIsAnonymized
}: AnonToggleButtonProps) {
  return (
    <div className="absolute top-4 right-4 flex justify-end mb-2 z-40">
      <Tooltip>
        <TooltipTrigger asChild>
          <Button
            onClick={() => setIsAnonymized(!isAnonymized)}
            className={cn(
              'p-2 no-drag backdrop-blur-sm',
              isAnonymized &&
                '!bg-muted-foreground text-white hover:!bg-muted-foreground/80 hover:!text-white'
            )}
            variant="outline"
            size="icon"
          >
            {isAnonymized ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
          </Button>
        </TooltipTrigger>
        <TooltipContent>
          <p>{isAnonymized ? 'Show original messages' : 'Show anonymized messages'}</p>
        </TooltipContent>
      </Tooltip>
    </div>
  )
})
