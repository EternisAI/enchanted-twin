import { Button, buttonVariants } from '@renderer/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@renderer/components/ui/tooltip'
import { VariantProps } from 'class-variance-authority'

export function ActionButton({
  children,
  onClick,
  tooltipLabel,
  ...props
}: {
  children: React.ReactNode
  onClick: () => void
  tooltipLabel: string
} & VariantProps<typeof buttonVariants>) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button variant="ghost" size="icon" onClick={onClick} {...props}>
          {children}
        </Button>
      </TooltipTrigger>
      <TooltipContent side="bottom" align="center">
        {tooltipLabel}
      </TooltipContent>
    </Tooltip>
  )
}
