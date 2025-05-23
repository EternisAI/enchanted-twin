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

export const TOOL_URLS = {
  generate_image:
    'image:https://upload.wikimedia.org/wikipedia/commons/thumb/5/54/NotoSans_-_Frame_With_Picture_-_1F5BC.svg/330px-NotoSans_-_Frame_With_Picture_-_1F5BC.svg.png',
  perplexity_ask:
    'image:https://upload.wikimedia.org/wikipedia/commons/thumb/5/55/Magnifying_glass_icon.svg/480px-Magnifying_glass_icon.svg.png',
  memory_tool:
    'https://upload.wikimedia.org/wikipedia/commons/thumb/9/92/Brain_-_Lorc_-_game-icons.svg/500px-Brain_-_Lorc_-_game-icons.svg.png',
  schedule_task:
    'https://upload.wikimedia.org/wikipedia/commons/thumb/f/fa/Line-style-icons-calendar-black.svg/510px-Line-style-icons-calendar-black.svg.png'
}

export function getToolUrl(toolName?: string) {
  return toolName ? TOOL_URLS[toolName as keyof typeof TOOL_URLS] : undefined
}
