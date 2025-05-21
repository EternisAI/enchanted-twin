import { useState, useEffect } from 'react'
import { motion } from 'framer-motion'
import { ToolCall } from '@renderer/graphql/generated/graphql'
import ToolCallProcessing from './ToolCallProcessing'
import ToolCallResult from './ToolCallResult'

interface ToolCallCenterProps {
  activeToolCalls: ToolCall[]
}

export default function ToolCallCenter({ activeToolCalls }: ToolCallCenterProps) {
  const [isShowing, setIsShowing] = useState(false)

  useEffect(() => {
    if (activeToolCalls.length > 0) {
      setIsShowing(true)
    }
  }, [activeToolCalls.length])

  return (
    <>
      {!isShowing && (
        <div
          className="fixed top-0 right-0 h-full w-40 z-50"
          onMouseEnter={() => setIsShowing(true)}
        />
      )}

      <motion.div
        className="fixed top-0 right-0 h-full w-72 z-40 bg-background/80 backdrop-blur-sm border-l shadow-lg"
        // onMouseLeave={() => setIsShowing(false)}
        initial={{ x: '100%' }}
        animate={{ x: isShowing ? '0%' : '100%' }}
        transition={{ type: 'spring', stiffness: 300, damping: 30 }}
      >
        <div className="h-full w-full p-4 flex flex-col gap-4">
          <ToolCallProcessing toolCalls={activeToolCalls} />
          <ToolCallResult toolCalls={activeToolCalls} />
        </div>
      </motion.div>
    </>
  )
}
