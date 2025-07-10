import * as React from 'react'
import { Copy, Check } from 'lucide-react'
import { Button } from './button'
import { useClipboard } from '@renderer/hooks/useClipboard'
import { cn } from '@renderer/lib/utils'
import { motion, AnimatePresence } from 'framer-motion'
import { useEffect } from 'react'

interface CopyButtonProps extends Omit<React.ComponentProps<typeof Button>, 'onClick'> {
  /**
   * Text to copy to clipboard
   */
  text: string
  /**
   * Custom icon for the button
   */
  icon?: React.ReactNode
  /**
   * Custom icon for the success state
   */
  successIcon?: React.ReactNode
  /**
   * Show text label next to icon
   */
  showLabel?: boolean
  /**
   * Custom label text
   */
  label?: string
  /**
   * Custom success label text
   */
  successLabel?: string
}

/**
 * Reusable copy button component
 *
 * @example
 * ```tsx
 * <CopyButton text="Hello World" />
 * <CopyButton text={user.email} showLabel label="Copy email" />
 * ```
 */
export function CopyButton({
  text,
  icon,
  successIcon,
  showLabel = false,
  label = 'Copy',
  successLabel = 'Copied!',
  className,
  variant = 'ghost',
  size = 'sm',
  ...props
}: CopyButtonProps) {
  const { copy, copied } = useClipboard()
  const [isAnimating, setIsAnimating] = React.useState(false)

  const handleClick = async (e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    await copy(text)
  }

  // useEffect(() => {
  //   if (copied) {
  //     setIsAnimating(true)
  //   } else if (isAnimating) {
  //     const timer = setTimeout(() => {
  //       setIsAnimating(false)
  //     }, 200)
  //     return () => clearTimeout(timer)
  //   }
  // }, [copied, isAnimating])

  const showSuccess = copied || isAnimating

  return (
    <Button
      variant={variant}
      size={size}
      onClick={handleClick}
      className={cn('relative overflow-hidden', className)}
      {...props}
    >
      <AnimatePresence mode="wait" initial={false}>
        {showSuccess ? (
          <motion.div
            key="success"
            initial={{ y: 20, opacity: 0 }}
            animate={{ y: 0, opacity: 1 }}
            exit={{ y: -20, opacity: 0 }}
            transition={{ duration: 0.2, ease: 'easeInOut' }}
            className="flex items-center text-emerald-600 dark:text-emerald-500"
          >
            {successIcon || <Check className="h-4 w-4" />}
            {showLabel && (
              <motion.span
                initial={{ opacity: 0 }}
                animate={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.15 }}
                className="ml-1"
              >
                {successLabel}
              </motion.span>
            )}
          </motion.div>
        ) : (
          <motion.div
            key="default"
            initial={{ y: 0, opacity: 1 }}
            exit={{ y: -20, opacity: 0 }}
            transition={{ duration: 0.2, ease: 'easeInOut' }}
            className="flex items-center"
          >
            {icon || <Copy className="h-4 w-4" />}
            {showLabel && (
              <motion.span
                initial={{ opacity: 1 }}
                exit={{ opacity: 0 }}
                transition={{ duration: 0.15 }}
                className="ml-1"
              >
                {label}
              </motion.span>
            )}
          </motion.div>
        )}
      </AnimatePresence>
    </Button>
  )
}
