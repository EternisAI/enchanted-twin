import { ReactNode } from 'react'
import { X } from 'lucide-react'
import { cn } from '@renderer/lib/utils'
import { Button } from '../ui/button'

interface OnboardingLayoutProps {
  children: ReactNode
  title: string
  subtitle?: string
  className?: string
  onClose?: () => void
}

function OnboardingTitle({ title, subtitle }: { title: string; subtitle?: string }) {
  return (
    <div className="flex flex-col gap-2 text-center items-center">
      <h1 className="text-4xl tracking-normal text-white">{title}</h1>
      {subtitle && (
        <p className="text-lg text-center text-white/80 text-balance max-w-xl">{subtitle}</p>
      )}
    </div>
  )
}

export function OnboardingLayout({
  children,
  title,
  subtitle,
  className,
  onClose
}: OnboardingLayoutProps) {
  return (
    <div className="min-h-screen flex flex-col items-center justify-center p-8">
      {onClose && (
        <Button variant="ghost" size="icon" className="absolute top-4 right-4" onClick={onClose}>
          <X className="h-4 w-4" />
        </Button>
      )}
      <div
        className={cn(
          'w-full max-w-3xl flex flex-col gap-12 z-10 relative bg-transparent',
          className
        )}
      >
        <div className="flex flex-col gap-8">
          <OnboardingTitle title={title} subtitle={subtitle} />
          {children}
        </div>
      </div>
    </div>
  )
}
