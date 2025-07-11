'use client'
import { Button } from '@renderer/components/ui/button'
import { LucideIcon, Settings } from 'lucide-react'
import { cn } from '@renderer/lib/utils'
import IconContainer from '@renderer/assets/icons/IconContainer'

interface StatusInfo {
  icon: LucideIcon
  label: string
  color: string
}

interface DetailCardProps {
  title: string
  IconComponent: LucideIcon
  statusInfo: StatusInfo
  buttonLabel: string
  onButtonClick: () => void
  isLoading: boolean
  explanation?: string
  grantedIcon?: React.ReactNode
}

export function DetailCard({
  title,
  IconComponent,
  statusInfo,
  buttonLabel,
  onButtonClick,
  isLoading,
  explanation,
  grantedIcon = <Settings className="h-4 w-4" />
}: DetailCardProps) {
  return (
    <div className="p-2 hover:bg-muted transition-colors duration-100 rounded-md flex-col gap-3 flex items-center justify-between w-full bg-transparent border-none">
      <div className="flex items-start gap-4 w-full">
        <IconContainer className="bg-transparent">
          <IconComponent className={cn('h-7 w-7', statusInfo.color)} />
        </IconContainer>
        <div className="flex flex-col gap-1 w-full">
          <div className="flex items-center justify-between gap-1">
            <span className="font-semibold capitalize text-lg">{title}</span>
            <Button
              variant="outline"
              onClick={onButtonClick}
              disabled={isLoading}
              className="py-1 h-fit w-fit px-2"
            >
              {statusInfo.label === 'Granted' || statusInfo.label === 'Enabled'
                ? grantedIcon
                : buttonLabel}
            </Button>
          </div>
          {explanation && (
            <p className="text-sm text-muted-foreground mt-1 text-balance">{explanation}</p>
          )}
        </div>
      </div>
    </div>
  )
}
