import { cn } from '@renderer/lib/utils'

export function SettingsContent({
  children,
  className
}: {
  children: React.ReactNode
  className?: string
}) {
  return (
    <div className="p-4 sm:p-8 w-full flex flex-col items-center justify-center">
      <div className={cn('flex flex-col gap-10 w-full max-w-3xl', className)}>{children}</div>
    </div>
  )
}
