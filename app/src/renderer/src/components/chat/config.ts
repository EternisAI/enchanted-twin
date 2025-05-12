export const TOOL_NAMES = {
  search_tool: {
    inProgress: 'Searching the web',
    completed: 'Searched the web'
  },
  image_tool: {
    inProgress: 'Generating Image',
    completed: 'Generated Image'
  },
  twitter_get_timeline: {
    inProgress: 'Checking Twitter feed',
    completed: 'Loaded Twitter feed'
  },
  telegram_send_message: {
    inProgress: 'Sending message',
    completed: 'Message sent to Telegram'
  },
  perplexity_ask: {
    inProgress: 'Searching the web',
    completed: 'Web Search'
  }
}

export function extractReasoningAndReply(raw: string): {
  thinkingText: string | null
  replyText: string
} {
  const thinkingTag = '<think>'
  const thinkingEndTag = '</think>'

  if (!raw.startsWith(thinkingTag)) return { thinkingText: null, replyText: raw }

  const closingIndex = raw.indexOf(thinkingEndTag)
  if (closingIndex !== -1) {
    const thinking = raw.slice(thinkingTag.length, closingIndex).trim()
    const rest = raw.slice(closingIndex + thinkingEndTag.length).trim()
    return { thinkingText: thinking, replyText: rest }
  } else {
    const thinking = raw.slice(thinkingTag.length).trim()
    return { thinkingText: thinking, replyText: '' }
  }
}

export function formatToolName(toolName: string) {
  const replaceSnakeCaseToWords = (name: string) =>
    name.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase())

  const toolNameInProgress = TOOL_NAMES[toolName]?.inProgress || replaceSnakeCaseToWords(toolName)
  const toolNameCompleted = TOOL_NAMES[toolName]?.completed || replaceSnakeCaseToWords(toolName)
  return { toolNameInProgress, toolNameCompleted }
}
