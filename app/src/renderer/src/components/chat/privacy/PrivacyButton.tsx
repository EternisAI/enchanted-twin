import { Button } from '@renderer/components/ui/button'
import { LockIcon } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@renderer/components/ui/tooltip'
import { cn } from '@renderer/lib/utils'

export function PrivacyButton({ className }: { className?: string }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className={cn(
            'h-7 flex items-center justify-center px-2 gap-1 text-xs z-50 no-drag cursor-help',
            className
          )}
        >
          <LockIcon className="w-4 h-4" />
          <span className="text-muted-foreground tracking-tight font-medium">Protected Data</span>
        </Button>
      </TooltipTrigger>
      <TooltipContent>
        <div className="max-w-xs text-wrap">
          <p className="text-xs">Your data is private</p>
          <p className="text-xs opacity-70">
            Messages are anonymized before being sent through a TEE proxy. More details soon.
          </p>
        </div>
      </TooltipContent>
    </Tooltip>
  )
}
