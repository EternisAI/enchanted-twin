import { McpServerType } from '@renderer/graphql/generated/graphql'
import Google from '@renderer/assets/icons/google'
import Slack from '@renderer/assets/icons/slack'
import XformerlyTwitter from '@renderer/assets/icons/x'
import IconContainer from '@renderer/assets/icons/IconContainer'
import { ComputerIcon, PlugIcon } from 'lucide-react'
import enchantedIcon from '@resources/icon.png'
import freysaIcon from '@resources/freysa.png'

interface ProviderConfig {
  provider: string
  scope: string
  description: string
  icon: React.ReactNode
  supportsMultipleConnections: boolean
}

export const PROVIDER_CONFIG: Record<McpServerType, ProviderConfig> = {
  GOOGLE: {
    provider: 'google',
    scope:
      'openid email profile https://www.googleapis.com/auth/drive https://mail.google.com/ https://www.googleapis.com/auth/calendar',
    description: 'Access Gmail, Google Drive, and Calendar',
    icon: <Google className="w-10 h-10" />,
    supportsMultipleConnections: true
  },
  SLACK: {
    provider: 'slack',
    scope:
      'channels:read,groups:read,channels:history,groups:history,im:read,mpim:read,search:read,users:read',
    description: 'Read messages and communicate with your team',
    icon: <Slack className="w-10 h-10" />,
    supportsMultipleConnections: true
  },
  TWITTER: {
    provider: 'twitter',
    scope: 'like.read tweet.read users.read offline.access tweet.write bookmark.read',
    description: 'Read and write tweets, manage bookmarks',
    icon: (
      <IconContainer className="bg-foreground">
        <XformerlyTwitter className="w-7 h-7 text-primary-foreground" />
      </IconContainer>
    ),
    supportsMultipleConnections: true
  },
  SCREENPIPE: {
    provider: 'screenpipe',
    scope: '',
    description: 'Record screen activity for AI context',
    icon: (
      <IconContainer>
        <ComputerIcon strokeWidth={1.5} className="w-7 h-7" />
      </IconContainer>
    ),
    supportsMultipleConnections: false
  },
  OTHER: {
    provider: 'other',
    scope: '',
    description: 'Connect custom MCP servers',
    icon: (
      <IconContainer>
        <PlugIcon strokeWidth={1.5} className="w-10 h-10" />
      </IconContainer>
    ),
    supportsMultipleConnections: true
  },
  ENCHANTED: {
    provider: 'enchanted',
    scope: '',
    description: 'Enhanced AI capabilities and tools',
    icon: <img src={enchantedIcon} alt="Enchanted" className="w-10 h-10" />,
    supportsMultipleConnections: false
  },
  FREYSA: {
    provider: 'freysa',
    scope: '',
    description: 'Generate videos with popular templates or create your own',
    icon: <img src={freysaIcon} alt="Freysa" className="w-10 h-10" />,
    supportsMultipleConnections: false
  }
}

// Legacy exports for backward compatibility
export const PROVIDER_MAP: Record<McpServerType, { provider: string; scope: string }> =
  Object.fromEntries(
    Object.entries(PROVIDER_CONFIG).map(([key, value]) => [
      key,
      { provider: value.provider, scope: value.scope }
    ])
  ) as Record<McpServerType, { provider: string; scope: string }>

export const PROVIDER_DESCRIPTION_MAP: Record<McpServerType, string> = Object.fromEntries(
  Object.entries(PROVIDER_CONFIG).map(([key, value]) => [key, value.description])
) as Record<McpServerType, string>

export const PROVIDER_ICON_MAP: Record<McpServerType, React.ReactNode> = Object.fromEntries(
  Object.entries(PROVIDER_CONFIG).map(([key, value]) => [key, value.icon])
) as Record<McpServerType, React.ReactNode>
