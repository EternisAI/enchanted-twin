import { Button } from '@renderer/components/ui/button'
import { LockIcon } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@renderer/components/ui/tooltip'
import { cn } from '@renderer/lib/utils'

export function PrivacyButton({
  className,
  hideLabel = false,
  label = 'Protected Data'
}: {
  className?: string
  hideLabel?: boolean
  label?: string
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="ghost"
          size={hideLabel ? 'icon' : 'sm'}
          className={cn('px-2 gap-1 text-xs z-50 no-drag cursor-help text-primary/50', className)}
        >
          <LockIcon className="w-4 h-4" />
          {!hideLabel && <span className="font-medium leading-none">{label}</span>}
        </Button>
      </TooltipTrigger>
      <TooltipContent>
        <div className="max-w-xs text-wrap">
          <p className="text-xs">Privacy Enabled</p>
          <p className="text-xs opacity-70">Your messages are routed through our privacy network</p>
        </div>
      </TooltipContent>
    </Tooltip>
  )
}
