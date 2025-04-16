import MessageInput from './MessageInput'

export default function ChatHome() {
  return (
    <div
      className="flex flex-col items-center py-10 w-full h-full"
      style={{
        viewTransitionName: 'page-content'
      }}
    >
      <style>
        {`
          :root {
            --direction: -1;
          }
        `}
      </style>
      <div className="flex flex-col flex-1 min-h-full w-full justify-between">
        <div
          className="p-6 flex flex-col items-center overflow-y-auto scrollbar-thin scrollbar-thumb-gray-300 scrollbar-track-transparent"
          style={{ maxHeight: `calc(100vh - ${'130px'})` }}
        >
          <h1 className="text-2xl font-bold text-black">Home</h1>
          <h5 className="text-gray-500 text-md">Send a message to start a conversation</h5>
        </div>
        <div
          className="px-6 py-6 border-t border-gray-200"
          style={{ height: '130px' } as React.CSSProperties}
        >
          <MessageInput onSend={() => {}} isWaitingTwinResponse={false} />
        </div>
      </div>
    </div>
  )
}
