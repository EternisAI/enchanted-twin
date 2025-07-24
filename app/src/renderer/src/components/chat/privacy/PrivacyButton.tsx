import { ExternalLink, LockIcon } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@renderer/components/ui/tooltip'
import { cn } from '@renderer/lib/utils'
import { Button } from '@renderer/components/ui/button'

const PRIVACY_URL =
  'https://eternis.notion.site/User-facing-Privacy-preserving-AI-interface-22228664e9b180bb879ed89fbe4ea5be?pvs=74'

export function PrivacyButton({
  className,
  label = 'Privacy Enabled'
}: {
  className?: string
  label?: string | boolean
}) {
  const handleClick = () => {
    window.open(PRIVACY_URL, '_blank')
  }
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="ghost"
          size={label ? 'sm' : 'icon'}
          className={cn(
            'px-2 gap-1 h-8 text-xs z-50 no-drag text-primary/50 flex items-center justify-center w-fit self-center active:bg-transparent hover:bg-transparent',
            className
          )}
          onClick={handleClick}
        >
          <LockIcon className="w-4 h-4" />
          {label && <span className="font-medium leading-none">{label}</span>}
        </Button>
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
