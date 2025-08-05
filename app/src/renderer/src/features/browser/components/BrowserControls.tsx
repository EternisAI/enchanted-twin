import { useState, useRef, FormEvent, useEffect } from 'react'
import { ArrowLeft, ArrowRight, RotateCw, X, Search, Shield, ShieldOff } from 'lucide-react'
import { Button } from '@renderer/components/ui/button'
import { Input } from '@renderer/components/ui/input'
import { cn } from '@renderer/lib/utils'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger
} from '@renderer/components/ui/tooltip'

interface BrowserControlsProps {
  url: string
  canGoBack: boolean
  canGoForward: boolean
  isLoading: boolean
  isSecure: boolean
  onNavigate: (url: string) => void
  onGoBack: () => void
  onGoForward: () => void
  onRefresh: () => void
  onStop: () => void
  className?: string
}

export function BrowserControls({
  url,
  canGoBack,
  canGoForward,
  isLoading,
  isSecure,
  onNavigate,
  onGoBack,
  onGoForward,
  onRefresh,
  onStop,
  className
}: BrowserControlsProps) {
  const [inputUrl, setInputUrl] = useState(url)
  const [isFocused, setIsFocused] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (!isFocused) {
      setInputUrl(url)
    }
  }, [url, isFocused])

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault()

    // Ensure URL has protocol
    let finalUrl = inputUrl.trim()
    if (!finalUrl.startsWith('http://') && !finalUrl.startsWith('https://')) {
      // Check if it looks like a URL
      if (finalUrl.includes('.') && !finalUrl.includes(' ')) {
        finalUrl = 'https://' + finalUrl
      } else {
        // Treat as search query
        finalUrl = `https://www.google.com/search?q=${encodeURIComponent(finalUrl)}`
      }
    }

    onNavigate(finalUrl)
    setInputUrl(finalUrl)
    inputRef.current?.blur()
  }

  return (
    <TooltipProvider>
      <div className={cn('flex items-center gap-2 p-2 border-b bg-background', className)}>
        {/* Navigation buttons */}
        <div className="flex items-center gap-1">
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={onGoBack}
                disabled={!canGoBack}
                className="h-8 w-8"
              >
                <ArrowLeft className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Go back</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={onGoForward}
                disabled={!canGoForward}
                className="h-8 w-8"
              >
                <ArrowRight className="h-4 w-4" />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Go forward</TooltipContent>
          </Tooltip>

          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                onClick={isLoading ? onStop : onRefresh}
                className="h-8 w-8"
              >
                {isLoading ? <X className="h-4 w-4" /> : <RotateCw className="h-4 w-4" />}
              </Button>
            </TooltipTrigger>
            <TooltipContent>{isLoading ? 'Stop' : 'Refresh'}</TooltipContent>
          </Tooltip>
        </div>

        {/* URL bar */}
        <form onSubmit={handleSubmit} className="flex-1 flex items-center gap-2">
          <div className="relative flex-1">
            <div className="absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none">
              {isSecure ? (
                <Shield className="h-4 w-4 text-green-600 dark:text-green-400" />
              ) : (
                <ShieldOff className="h-4 w-4 text-orange-600 dark:text-orange-400" />
              )}
            </div>
            <Input
              ref={inputRef}
              type="text"
              value={inputUrl}
              onChange={(e) => setInputUrl(e.target.value)}
              onFocus={() => setIsFocused(true)}
              onBlur={() => setIsFocused(false)}
              placeholder="Enter URL or search..."
              className="pl-10 pr-10 h-9"
            />
            <Button
              type="submit"
              variant="ghost"
              size="icon"
              className="absolute right-0 top-0 h-9 w-9"
            >
              <Search className="h-4 w-4" />
            </Button>
          </div>
        </form>
      </div>
    </TooltipProvider>
  )
}
