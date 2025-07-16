import { McpServerType } from '@renderer/graphql/generated/graphql'
import Google from '@renderer/assets/icons/google'
import Slack from '@renderer/assets/icons/slack'
import XformerlyTwitter from '@renderer/assets/icons/x'
import IconContainer from '@renderer/assets/icons/IconContainer'
import { PlugIcon } from 'lucide-react'
import screenpipeIcon from '@renderer/assets/icons/screenpipe.png'
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
    icon: (
      <IconContainer>
        <Google className="size-7" />
      </IconContainer>
    ),
    supportsMultipleConnections: false
  },
  SLACK: {
    provider: 'slack',
    scope:
      'channels:read,groups:read,channels:history,groups:history,im:read,mpim:read,search:read,users:read',
    description: 'Read messages and communicate with your team',
    icon: (
      <IconContainer>
        <Slack className="size-7" />
      </IconContainer>
    ),
    supportsMultipleConnections: false
  },
  TWITTER: {
    provider: 'twitter',
    scope: 'like.read tweet.read users.read offline.access tweet.write bookmark.read',
    description: 'Read and write tweets, manage bookmarks',
    icon: (
      <IconContainer className="bg-foreground">
        <XformerlyTwitter className="size-7 text-primary-foreground" />
      </IconContainer>
    ),
    supportsMultipleConnections: false
  },
  SCREENPIPE: {
    provider: 'screenpipe',
    scope: '',
    description: 'Record screen activity for AI context',
    icon: (
      <IconContainer>
        <img src={screenpipeIcon} alt="Screenpipe" className="size-7" />
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
        <PlugIcon strokeWidth={1.5} className="size-7" />
      </IconContainer>
    ),
    supportsMultipleConnections: false
  },
  ENCHANTED: {
    provider: 'enchanted',
    scope: '',
    description: 'Generate images and search the web',
    icon: (
      <IconContainer>
        <img src={enchantedIcon} alt="Essentials" className="size-7" />
      </IconContainer>
    ),
    supportsMultipleConnections: false
  },
  FREYSA: {
    provider: 'freysa',
    scope: '',
    description: 'Generate videos with popular templates or create your own',
    icon: (
      <IconContainer>
        <img src={freysaIcon} alt="Freysa" className="size-7" />
      </IconContainer>
    ),
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
