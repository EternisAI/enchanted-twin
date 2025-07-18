import { Eye, EyeOff } from 'lucide-react'
import { Button } from '../ui/button'
import { Tooltip, TooltipContent, TooltipTrigger } from '../ui/tooltip'
import { memo } from 'react'
import { cn } from '@renderer/lib/utils'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '../ui/dialog'
import { useState } from 'react'
import { motion } from 'framer-motion'

interface AnonToggleButtonProps {
  isAnonymized: boolean
  setIsAnonymized: (isAnonymized: boolean) => void
}

export const AnonToggleButton = memo(function AnonToggleButton({
  isAnonymized,
  setIsAnonymized
}: AnonToggleButtonProps) {
  const [isDialogOpen, setIsDialogOpen] = useState(false)

  return (
    <>
      <div className="absolute top-4 right-4 flex justify-end mb-2 z-40">
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              onClick={() => setIsAnonymized(!isAnonymized)}
              className={cn(
                'p-2 no-drag backdrop-blur-sm',
                isAnonymized &&
                  '!bg-muted-foreground text-white hover:!bg-muted-foreground/80 hover:!text-white'
              )}
              variant="outline"
              size="icon"
            >
              {isAnonymized ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </Button>
          </TooltipTrigger>
          <TooltipContent className="max-w-[200px] space-y-1">
            <p>{isAnonymized ? 'Show original messages' : 'Show anonymized messages'}</p>
            <p className="text-xs text-primary-foreground/50 ">
              Messages are always anonymized before being sent.
            </p>
            <a
              href="#"
              onClick={(e) => {
                e.preventDefault()
                setIsDialogOpen(true)
              }}
              className="font-semibold hover:underline text-xs text-primary-foreground/50"
            >
              Learn more
            </a>
          </TooltipContent>
        </Tooltip>
      </div>
      <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>How Anonymization Works</DialogTitle>
          </DialogHeader>
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.3 }}
          >
            <DialogDescription className="space-y-4 prose text-primary">
              <div className="space-y-3">
                <div className="flex items-start space-x-3">
                  <div className="flex-shrink-0 w-6 h-6 rounded-full border-border border flex items-center justify-center text-sm font-medium">
                    1
                  </div>
                  <p className="text-sm">You send a message</p>
                </div>
                <div className="flex items-start space-x-3">
                  <div className="flex-shrink-0 w-6 h-6 rounded-full border-border border flex items-center justify-center text-sm font-medium">
                    2
                  </div>
                  <p className="text-sm">
                    Before sending, we filter out private and sensitive information, replacing it
                    with anonymized equivalents.
                    <br />
                    <span className="inline-block my-1.5 bg-muted-foreground/10 p-1 rounded-md">
                      My friend found an unreleased iPhone at a bar. Should they sell it to Gizmodo?
                    </span>
                    <br />
                    <span className="block text-center">becomes anonymized:</span>
                    <span className="inline-block my-1.5 bg-muted-foreground/10 p-1 rounded-md">
                      My friend found an unreleased{' '}
                      <span className="inline-block rounded-sm bg-muted-foreground text-primary-foreground px-1">
                        mobile device
                      </span>{' '}
                      at a bar. Should they sell it to{' '}
                      <span className="inline-block rounded-sm bg-muted-foreground px-1 text-primary-foreground">
                        TechReviews
                      </span>
                      ?
                    </span>
                    <br />
                    <span className="text-xs text-muted-foreground">
                      This is done on your machine, before the message is sent.
                    </span>
                  </p>
                </div>
                <div className="flex items-start space-x-3">
                  <div className="flex-shrink-0 w-6 h-6 rounded-full border-border border flex items-center justify-center text-sm font-medium">
                    3
                  </div>
                  <p className="text-sm">Enchanted sends the anonymized message to a secure LLM.</p>
                </div>
                <div className="flex items-start space-x-3">
                  <div className="flex-shrink-0 w-6 h-6 rounded-full border-border border flex items-center justify-center text-sm font-medium">
                    4
                  </div>
                  <p className="text-sm">
                    Enchanted de-anonymizes the response before displaying it.
                  </p>
                </div>
              </div>
              <p className="text-sm">
                This protects your privacy while keeping the conversation meaningful. You can toggle
                between original and anonymized views locally – but{' '}
                <strong>messages are always anonymized</strong> before AI processing.
              </p>
            </DialogDescription>
          </motion.div>
        </DialogContent>
      </Dialog>
    </>
  )
})
