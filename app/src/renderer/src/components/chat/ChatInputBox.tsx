import React from 'react'
import { Textarea } from '../ui/textarea'
import { TooltipContent } from '../ui/tooltip'
import { TooltipTrigger } from '../ui/tooltip'
import { Tooltip } from '../ui/tooltip'
import { AnimatePresence, motion } from 'framer-motion'
import { Button } from '../ui/button'
import { AudioLines, Lightbulb, X } from 'lucide-react'
import { cn } from '@renderer/lib/utils'
import { SendButton } from './MessageInput'
import { toast } from 'sonner'

type ChatInputBoxProps = {
  query: string
  isVoiceInstalled: boolean
  textareaRef: React.RefObject<HTMLTextAreaElement | null>
  onInputChange: (query: string) => void
  handleSubmit: (e: React.KeyboardEvent<HTMLTextAreaElement>) => void
  isReasonSelected: boolean
  setIsReasonSelected: (isReasonSelected: boolean) => void
  isVoiceMode: boolean
  onVoiceModeChange: (toggleSidebar?: boolean) => void
  handleCreateChat: () => void
}

export default function ChatInputBox({
  query,
  textareaRef,
  isReasonSelected,
  isVoiceInstalled,
  isVoiceMode,
  onInputChange,
  handleSubmit,
  setIsReasonSelected,
  handleCreateChat,
  onVoiceModeChange
}: ChatInputBoxProps) {
  return (
    <div className="flex items-center gap-2 w-full border border-gray-200 dark:border-gray-800 rounded-lg px-2.5 py-0">
      <Textarea
        ref={textareaRef}
        value={query}
        onChange={(e) => onInputChange(e.target.value)}
        onKeyDown={(e: React.KeyboardEvent<HTMLTextAreaElement>) => {
          if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault()
            handleSubmit(e)
          }
        }}
        placeholder="What are you thinking?"
        className="!text-base flex-1 border-0 shadow-none focus-visible:ring-0 focus-visible:ring-offset-0 py-4 pl-2 pr-1 resize-none overflow-y-hidden min-h-[50px] bg-transparent"
        rows={1}
      />

      <motion.div className="flex items-center gap-2">
        {!isVoiceMode && (
          <ReasoningButton
            isSelected={isReasonSelected}
            onClick={() => setIsReasonSelected(!isReasonSelected)}
            disabled={isVoiceMode}
          />
        )}
        {(query.length > 0 || isVoiceMode) && (
          <SendButton
            className="w-9 h-9"
            text={query}
            onSend={handleCreateChat}
            isWaitingTwinResponse={false}
          />
        )}
        {!isVoiceMode && query.length === 0 && (
          <EnableVoiceModeButton
            onClick={() => {
              onVoiceModeChange(false)
            }}
            isVoiceInstalled={isVoiceInstalled}
          />
        )}
        {isVoiceMode && (
          <DisableVoiceModeButton
            onClick={() => onVoiceModeChange(false)}
            isVoiceInstalled={isVoiceInstalled}
          />
        )}
      </motion.div>
    </div>
  )
}

interface ReasoningButtonProps {
  isSelected: boolean
  onClick: () => void
  disabled?: boolean
}

function ReasoningButton({ isSelected, onClick, disabled }: ReasoningButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          onClick={onClick}
          className={cn(
            '!px-2.5 rounded-full transition-all shadow-none hover:shadow-lg active:shadow-sm border-none',
            isSelected
              ? 'text-orange-500 !bg-orange-100/50 dark:!bg-orange-300/20 ring-orange-200 border-orange-200'
              : '!bg-gray-200 dark:!bg-gray-800'
          )}
          variant="outline"
          disabled={disabled}
        >
          <Lightbulb className="w-4 h-4" />
        </Button>
      </TooltipTrigger>
      <TooltipContent>
        <p>Reasoning</p>
      </TooltipContent>
    </Tooltip>
  )
}

interface VoiceModeButtonProps {
  isVoiceInstalled: boolean
  onClick: () => void
}

export function EnableVoiceModeButton({ onClick, isVoiceInstalled }: VoiceModeButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          onClick={() => {
            if (isVoiceInstalled) {
              onClick()
            } else {
              toast.error('Voice dependencies installation in progress')
            }
          }}
          className={cn(
            '!px-4.5 relative rounded-full transition-all shadow-none hover:shadow-lg active:shadow-sm border-none'
          )}
        >
          <AnimatePresence mode="wait" initial={false}>
            <motion.span
              key="off"
              initial={{ opacity: 0, scale: 0.8 }}
              animate={{ opacity: 1, scale: 1 }}
              exit={{ opacity: 0, scale: 0.8 }}
              transition={{ duration: 0.2 }}
              className="flex items-center gap-2"
            >
              <AudioLines className="w-4 h-4" />
              Voice Output
            </motion.span>
          </AnimatePresence>
        </Button>
      </TooltipTrigger>
      <TooltipContent>
        <p>{isVoiceInstalled ? 'Listen to output' : 'Installing voice dependencies...'}</p>
      </TooltipContent>
    </Tooltip>
  )
}

export function DisableVoiceModeButton({ onClick }: VoiceModeButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button
          onClick={onClick}
          variant="outline"
          className={cn(
            '!px-4.5 relative rounded-full transition-all shadow-none hover:shadow-lg active:shadow-sm border-none'
          )}
          // variant={isVoiceMode ? 'outline' : 'default'}
        >
          <AnimatePresence mode="wait" initial={false}>
            <motion.div
              key="stop"
              initial={{ scale: 0, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              exit={{ scale: 0, opacity: 0 }}
              transition={{ duration: 0.2, ease: 'easeOut' }}
              className="absolute inset-0 flex items-center justify-center"
            >
              <X className="w-5 h-5" />
            </motion.div>
          </AnimatePresence>
        </Button>
      </TooltipTrigger>
      <TooltipContent>
        <p>Stop voice output</p>
      </TooltipContent>
    </Tooltip>
  )
}
