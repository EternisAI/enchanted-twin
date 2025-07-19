import * as React from 'react'

import { cn } from '@renderer/lib/utils'
import { motion } from 'framer-motion'

function Textarea({ className, ...props }: React.ComponentProps<typeof motion.textarea>) {
  return (
    <motion.textarea
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      transition={{ duration: 0.3, ease: [0.4, 0, 0.2, 1] }}
      data-slot="textarea"
      className={cn(
        'text-foreground placeholder:text-muted-foreground bg-input focus-visible:bg-input aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive dark:bg-input/30 flex field-sizing-content min-h-16 w-full rounded-md border-transparent px-3 py-2 text-base shadow-xs transition-[color,box-shadow,background-color] outline-none focus-visible:ring-0 disabled:cursor-not-allowed disabled:opacity-50 md:text-sm',
        className
      )}
      {...props}
    />
  )
}

export { Textarea }
