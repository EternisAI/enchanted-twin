'use client'
import { Card } from '@renderer/components/ui/card'
import { Button } from '@renderer/components/ui/button'
import { LucideIcon } from 'lucide-react'
import { cn } from '@renderer/lib/utils'

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
}

export function DetailCard({
  title,
  IconComponent,
  statusInfo,
  buttonLabel,
  onButtonClick,
  isLoading
}: DetailCardProps) {
  const StatusIcon = statusInfo.icon

  return (
    <Card className="p-4 min-w-[200px] flex flex-col items-center gap-3">
      <div className="flex flex-col items-center gap-2">
        <IconComponent className="h-5 w-5 text-muted-foreground" />
        <span className="font-medium capitalize">{title}</span>
      </div>

      <div className="flex items-center gap-2">
        <StatusIcon className={cn('h-5 w-5', statusInfo.color)} />
        <span className={cn('text-sm', statusInfo.color)}>{statusInfo.label}</span>
      </div>

      <Button
        variant="outline"
        className="w-full"
        size="sm"
        onClick={onButtonClick}
        disabled={isLoading}
      >
        {buttonLabel}
      </Button>
    </Card>
  )
}
