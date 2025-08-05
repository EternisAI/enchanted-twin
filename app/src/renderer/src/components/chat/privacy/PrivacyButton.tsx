import { LockIcon } from 'lucide-react'
import { Tooltip, TooltipContent, TooltipTrigger } from '@renderer/components/ui/tooltip'
import { cn } from '@renderer/lib/utils'
import { Button } from '@renderer/components/ui/button'
import { useModal } from '@renderer/hooks/useModal'
import { PrivacyModal } from './PrivacyModal'

export function PrivacyButton({
  className,
  label = 'Privacy Enabled'
}: {
  className?: string
  label?: string | boolean
}) {
  const { openModal } = useModal()

  const handleClick = () => {
    openModal(<PrivacyModal />)
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
      </TooltipContent>
    </Tooltip>
  )
}
