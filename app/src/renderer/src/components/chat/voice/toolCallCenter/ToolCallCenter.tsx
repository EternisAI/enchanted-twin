import { motion } from 'framer-motion'
import { ToolCall } from '@renderer/graphql/generated/graphql'
import ToolCallProcessing from './ToolCallProcessing'
import ToolCallResult from './ToolCallResult'

interface ToolCallCenterProps {
  activeToolCalls: ToolCall[]
  historicToolCalls: ToolCall[]
}

export default function ToolCallCenter({
  activeToolCalls,
  historicToolCalls
}: ToolCallCenterProps) {
  // const [isShowing, setIsShowing] = useState(false)
  // const { notifications } = useNotifications()

  // useEffect(() => {
  //   if (activeToolCalls.length > 0) {
  //     setIsShowing(true)
  //   }
  // }, [activeToolCalls.length])

  return (
    <>
      {/* {!isShowing && (
        <div
          className="fixed top-0 right-0 h-full w-72 z-50 border border-red-500"
          onMouseEnter={() => setIsShowing(true)}
        />
      )} */}

      <motion.div
        className="fixed top-4 right-0 h-[75%] w-72"
        // className="h-full w-72 bg-background/80 backdrop-blur-sm"
        // onMouseLeave={() => setIsShowing(false)}
        initial={{ x: '100%' }}
        animate={{ x: '0%' }}
        // animate={{ x: isShowing ? '0%' : '100%' }}
        transition={{ type: 'spring', stiffness: 300, damping: 30 }}
      >
        <div className="h-full w-full p-4 overflow-y-auto overflow-x-hidden flex flex-col gap-8">
          <ToolCallProcessing toolCalls={activeToolCalls} />
          {/* {notifications.length > 0 && <ToolCallNotificationList notifications={notifications} />} */}
          <ToolCallResult toolCalls={activeToolCalls} />
          <div className="flex flex-col gap-2">
            <p className="text-sm text-gray-500">Tool Call History</p>
            <ToolCallResult toolCalls={historicToolCalls} />
          </div>
        </div>
      </motion.div>
    </>
  )
}
