import { motion } from 'framer-motion'
import { ToolCall } from '@renderer/graphql/generated/graphql'
import ToolCallProcessing from './ToolCallProcessing'
import ToolCallResult from './ToolCallResult'
import { useEffect, useState } from 'react'

interface ToolCallCenterProps {
  activeToolCalls: ToolCall[]
  historicToolCalls: ToolCall[]
}

export default function ToolCallCenter({
  activeToolCalls,
  historicToolCalls
}: ToolCallCenterProps) {
  const [isShowing, setIsShowing] = useState(false)

  useEffect(() => {
    if (activeToolCalls.length > 0 || historicToolCalls.length > 0) {
      setIsShowing(true)
    }
  }, [activeToolCalls.length, historicToolCalls.length])

  if (!isShowing) return null

  return (
    <>
      <motion.div
        className="fixed top-10 right-0 h-[95%] w-72"
        initial={{ x: '100%' }}
        animate={{ x: '0%' }}
        transition={{ type: 'spring', stiffness: 300, damping: 55 }}
      >
        <div className="h-full w-full p-4 overflow-y-auto overflow-x-hidden flex flex-col gap-8">
          <ToolCallProcessing toolCalls={activeToolCalls} />
          {/* {notifications.length > 0 && <ToolCallNotificationList notifications={notifications} />} */}
          <ToolCallResult toolCalls={activeToolCalls} />
          {historicToolCalls.length > 0 && (
            <div className="flex flex-col gap-2">
              <p className="text-sm text-gray-500">Tool Call History</p>
              <ToolCallResult toolCalls={historicToolCalls} />
            </div>
          )}
        </div>
      </motion.div>
    </>
  )
}
