'use client'
import { Card } from '@renderer/components/ui/card'
import { Button } from '@renderer/components/ui/button'
import { LucideIcon, Settings } from 'lucide-react'
import { cn } from '@renderer/lib/utils'

// Helper function to map status text color to background color
const getStatusBgColor = (textColor: string): string => {
  switch (textColor) {
    case 'text-green-500':
      return 'bg-green-100'
    case 'text-red-500':
      return 'bg-red-100'
    case 'text-yellow-500':
      return 'bg-yellow-100'
    case 'text-muted-foreground':
    default:
      return 'bg-muted'
  }
}

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
}

export function DetailCard({
  title,
  IconComponent,
  statusInfo,
  buttonLabel,
  onButtonClick,
  isLoading,
  explanation
}: DetailCardProps) {
  // const StatusIcon = statusInfo.icon // Remove unused variable

  return (
    <Card className="p-4 flex-col gap-3 flex items-start justify-between w-full">
      <div className="flex items-start gap-3 w-full">
        <div
          className={cn(
            'h-8 w-8 rounded-full flex items-center justify-center flex-shrink-0',
            getStatusBgColor(statusInfo.color)
          )}
        >
          <IconComponent className={cn('h-5 w-5', statusInfo.color)} />
        </div>
        <div className="flex flex-col gap-1 w-full">
          <div className="flex items-center justify-between gap-1">
            <span className="font-medium capitalize pt-1">{title}</span>
            <Button
              variant="outline"
              size="sm"
              onClick={onButtonClick}
              disabled={isLoading}
              className="py-1 h-fit w-fit px-2"
            >
              {statusInfo.label === 'Granted' || statusInfo.label === 'Enabled' ? (
                <Settings className="h-4 w-4" />
              ) : (
                buttonLabel
              )}
            </Button>
          </div>
          {explanation && <p className="text-xs text-muted-foreground mt-1">{explanation}</p>}
        </div>
      </div>
    </Card>
  )
}
