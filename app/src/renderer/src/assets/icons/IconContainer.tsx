import { cn } from '@renderer/lib/utils'
import { ReactNode } from 'react'

export default function IconContainer({
  children,
  className
}: {
  children: ReactNode
  className?: string
}) {
  return (
    <div
      className={cn('flex items-center justify-center rounded-sm w-10 h-10 bg-muted/50', className)}
    >
      {children}
    </div>
  )
}
