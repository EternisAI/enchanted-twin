import { ToolCall } from '@renderer/graphql/generated/graphql'
import HolonToolComponent from '../holon/HolonToolComponent'
import HolonCreatedComponent from '../holon/HolonCreatedComponent'

export type ToolName =
  | 'search_tool'
  | 'image_tool'
  | 'twitter_get_timeline'
  | 'telegram_send_message'
  | 'perplexity_ask'
  | 'generate_image'
  | 'memory_tool'
  | 'schedule_task'
  | 'preview_thread'
  | 'send_to_holon'

export const TOOL_CONFIG: Record<
  ToolName,
  {
    inProgress: string
    completed: string
    url?: string
    component?: React.ComponentType<{
      toolCall: ToolCall
    }>
  }
> = {
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
    completed: 'Web Search',
    url: 'image:https://upload.wikimedia.org/wikipedia/commons/thumb/5/55/Magnifying_glass_icon.svg/480px-Magnifying_glass_icon.svg.png'
  },
  generate_image: {
    inProgress: 'Generating Image',
    completed: 'Generated Image',
    url: 'image:https://upload.wikimedia.org/wikipedia/commons/thumb/5/54/NotoSans_-_Frame_With_Picture_-_1F5BC.svg/330px-NotoSans_-_Frame_With_Picture_-_1F5BC.svg.png'
  },
  memory_tool: {
    inProgress: 'Accessing Memory',
    completed: 'Memory Retrieved',
    url: 'https://upload.wikimedia.org/wikipedia/commons/thumb/9/92/Brain_-_Lorc_-_game-icons.svg/500px-Brain_-_Lorc_-_game-icons.svg.png'
  },
  schedule_task: {
    inProgress: 'Scheduling Task',
    completed: 'Task Scheduled',
    url: 'https://upload.wikimedia.org/wikipedia/commons/thumb/f/fa/Line-style-icons-calendar-black.svg/510px-Line-style-icons-calendar-black.svg.png'
  },
  preview_thread: {
    inProgress: 'Creating Preview',
    completed: 'Preview Created',
    component: HolonToolComponent
  },
  send_to_holon: {
    inProgress: 'Sending to Holon',
    completed: 'Holon Generated',
    component: HolonCreatedComponent
  }
}

export function getToolConfig(toolName: string) {
  const replaceSnakeCaseToWords = (name: string) =>
    name.replace(/_/g, ' ').replace(/\b\w/g, (l) => l.toUpperCase())

  const config = TOOL_CONFIG[toolName as ToolName]

  if (config) {
    return {
      toolNameInProgress: config.inProgress,
      toolNameCompleted: config.completed,
      customComponent: config.component,
      toolUrl: config.url
    }
  }

  // Fallback for unknown tools
  const fallbackName = replaceSnakeCaseToWords(toolName)
  return {
    toolNameInProgress: fallbackName,
    toolNameCompleted: fallbackName,
    customComponent: undefined,
    toolUrl: undefined
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
