import * as React from 'react'

import { cn } from '@renderer/lib/utils'

function Input({ className, type, ...props }: React.ComponentProps<'input'>) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        'file:text-foreground placeholder:text-muted-foreground selection:bg-primary selection:text-primary-foreground dark:bg-input/30 flex h-9 w-full min-w-0 rounded-md bg-transparent px-3 py-1 text-base shadow-xs transition-colors outline-none file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-sm file:font-medium disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50 md:text-sm',
        'hover:bg-accent/50 bg-white dark:bg-black focus:bg-white dark:focus:bg-black',
        'aria-invalid:bg-destructive/20 dark:aria-invalid:bg-destructive/40',
        'backdrop-blur-sm border-[0.5px] border-border',
        className
      )}
      {...props}
    />
  )
}

export { Input }
