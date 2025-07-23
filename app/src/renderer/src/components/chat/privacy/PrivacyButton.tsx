import { LockIcon } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@renderer/components/ui/tooltip'
import { cn } from '@renderer/lib/utils'

export function PrivacyButton({
  className,
  label = 'Privacy Enabled'
}: {
  className?: string
  label?: string | boolean
}) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div
          className={cn(
            'px-2 gap-1 h-8 text-xs z-50 no-drag cursor-help text-primary/50 flex items-center justify-center',
            className
          )}
        >
          <LockIcon className="w-4 h-4" />
          {label && <span className="font-medium leading-none">{label}</span>}
        </div>
      </TooltipTrigger>
      <TooltipContent>
        <p className="text-xs">Your messages are routed through our privacy mixing network</p>
      </TooltipContent>
    </Tooltip>
  )
}
