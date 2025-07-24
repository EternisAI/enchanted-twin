import { ExternalLink, LockIcon } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@renderer/components/ui/tooltip'
import { cn } from '@renderer/lib/utils'

const PRIVACY_URL =
  'https://eternis.notion.site/User-facing-Privacy-preserving-AI-interface-22228664e9b180bb879ed89fbe4ea5be?pvs=74'

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
      <TooltipContent className="flex flex-col gap-2 p-3">
        <p className="text-xs max-w-[180px]">
          Your messages are routed through our privacy network
        </p>
        <a
          href={PRIVACY_URL}
          target="_blank"
          rel="noopener noreferrer"
          className="my-0.5 inline-flex items-center gap-1 relative font-semibold hover:underline text-xs text-primary-foreground/50 before:content-[''] before:absolute before:-inset-2"
        >
          Learn more
          <ExternalLink strokeWidth={2.5} className="w-3 h-3" />
        </a>
      </TooltipContent>
    </Tooltip>
  )
}
