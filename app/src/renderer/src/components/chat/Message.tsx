/* eslint-disable @typescript-eslint/no-explicit-any */
//@TODO: JUST TEMPORARY TYPES
type Role = 'user' | 'assistant'

type ToolCall = {
  name: string
  args: Record<string, any>
}

type MessageProps = {
  message: {
    id: string
    text?: string
    imageUrls: string[]
    role: Role
    toolCalls: ToolCall[]
    toolResult?: any
    createdAt: string
  }
}

export function MessageBubble({ message }: MessageProps) {
  const isUser = message.role === 'user'

  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'} mb-4 gap-3`}>
      <div
        className={`max-w-md py-2 px-4 rounded-lg shadow ${
          isUser ? 'bg-white text-right' : 'bg-gray-100 text-left'
        }`}
      >
        {message.text && <p className="text-gray-800 mb-2">{message.text}</p>}

        {message.imageUrls.length > 0 && (
          <div className="flex gap-2 overflow-x-auto mb-2">
            {message.imageUrls.map((url, i) => (
              <img
                key={i}
                src={url}
                alt={`attachment-${i}`}
                className="w-24 h-24 object-cover rounded"
              />
            ))}
          </div>
        )}

        {message.toolCalls.length > 0 && (
          <div className="text-xs text-gray-600 mb-2">
            <strong>Tool Calls:</strong>
            <ul className="ml-4">
              {message.toolCalls.map((tool, i) => (
                <li key={i}>
                  <code>{tool.name}</code> {JSON.stringify(tool.args)}
                </li>
              ))}
            </ul>
          </div>
        )}

        {message.toolResult && (
          <div className="bg-green-50 p-2 rounded text-xs text-green-800 whitespace-pre-wrap break-words">
            <strong>Tool Result</strong>
            <pre>{JSON.stringify(message.toolResult, null, 2)}</pre>
          </div>
        )}

        <div className="text-xs text-gray-500 pt-1">
          {new Date(message.createdAt).toLocaleTimeString()}
        </div>
      </div>
    </div>
  )
}
