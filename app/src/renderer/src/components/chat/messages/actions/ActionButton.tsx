import { Button, buttonVariants } from '@renderer/components/ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '@renderer/components/ui/tooltip'
import { VariantProps } from 'class-variance-authority'

export function ActionButton({
  children,
  onClick,
  tooltipLabel,
  disabled,
  ...props
}: {
  children: React.ReactNode
  onClick: () => void
  tooltipLabel: string
  disabled?: boolean
} & VariantProps<typeof buttonVariants>) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          onClick={onClick}
          disabled={disabled}
          aria-label={tooltipLabel}
          {...props}
        >
          {children}
        </Button>
      </TooltipTrigger>
      <TooltipContent side="bottom" align="center">
        {tooltipLabel}
      </TooltipContent>
    </Tooltip>
  )
}
