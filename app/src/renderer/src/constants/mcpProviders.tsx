import { McpServerType } from '@renderer/graphql/generated/graphql'
import Google from '@renderer/assets/icons/google'
import Slack from '@renderer/assets/icons/slack'
import XformerlyTwitter from '@renderer/assets/icons/x'
import IconContainer from '@renderer/assets/icons/IconContainer'
import { PlugIcon } from 'lucide-react'
import screenpipeIcon from '@renderer/assets/icons/screenpipe.png'
import enchantedIcon from '@resources/icon.png'

interface ProviderConfig {
  provider: string
  scope: string
  description: string
  icon: React.ReactNode
  supportsMultipleConnections: boolean
}

export const PROVIDER_CONFIG: Record<McpServerType, ProviderConfig> = {
  [McpServerType.Google]: {
    provider: 'google',
    scope:
      'openid email profile https://www.googleapis.com/auth/drive https://mail.google.com/ https://www.googleapis.com/auth/calendar',
    description: '',
    icon: <Google className="size-8" />,
    supportsMultipleConnections: false
  },
  [McpServerType.Slack]: {
    provider: 'slack',
    scope:
      'channels:read,groups:read,channels:history,groups:history,im:read,mpim:read,search:read,users:read',
    description: '',
    icon: <Slack className="size-8" />,
    supportsMultipleConnections: false
  },
  [McpServerType.Twitter]: {
    provider: 'twitter',
    scope: 'like.read tweet.read users.read offline.access tweet.write bookmark.read',
    description: '',
    icon: (
      <IconContainer className="bg-foreground size-8">
        <XformerlyTwitter className="size-5 text-primary-foreground" />
      </IconContainer>
    ),
    supportsMultipleConnections: false
  },
  [McpServerType.Screenpipe]: {
    provider: 'screenpipe',
    scope: '',
    description: 'Record screen activity for AI context',
    icon: <img src={screenpipeIcon} alt="Screenpipe" className="size-10" />,
    supportsMultipleConnections: false
  },
  [McpServerType.Other]: {
    provider: 'other',
    scope: '',
    description: 'Connect custom MCP servers',
    icon: (
      <IconContainer>
        <PlugIcon strokeWidth={1.5} className="size-5" />
      </IconContainer>
    ),
    supportsMultipleConnections: false
  },
  [McpServerType.Enchanted]: {
    provider: 'enchanted',
    scope: '',
    description: 'Generate images and search the web',
    icon: <img src={enchantedIcon} alt="Essentials" className="size-10" />,
    supportsMultipleConnections: false
  },
  [McpServerType.Freysa]: {
    provider: 'freysa',
    scope: '',
    description: 'Generate videos with popular templates or create your own',
    icon: <img src={enchantedIcon} alt="Freysa" className="size-10" />,
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
