import { McpServerType } from '@renderer/graphql/generated/graphql'
import Google from '@renderer/assets/icons/google'
import Slack from '@renderer/assets/icons/slack'
import XformerlyTwitter from '@renderer/assets/icons/x'
import IconContainer from '@renderer/assets/icons/IconContainer'
import { ComputerIcon, PlugIcon } from 'lucide-react'
import enchantedIcon from '@resources/icon.png'
import freysaIcon from '@resources/freysa.jpg'

export const PROVIDER_MAP: Record<McpServerType, { provider: string; scope: string }> = {
  GOOGLE: {
    provider: 'google',
    scope:
      'openid email profile https://www.googleapis.com/auth/drive https://mail.google.com/ https://www.googleapis.com/auth/calendar'
  },
  SLACK: {
    provider: 'slack',
    scope:
      'channels:read,groups:read,channels:history,groups:history,im:read,mpim:read,search:read,users:read'
  },
  TWITTER: {
    provider: 'twitter',
    scope: 'like.read tweet.read users.read offline.access tweet.write bookmark.read'
  },
  SCREENPIPE: { provider: 'screenpipe', scope: '' },
  OTHER: { provider: 'other', scope: '' },
  ENCHANTED: { provider: 'enchanted', scope: '' },
  FREYSA: { provider: 'freysa', scope: '' }
}

export const PROVIDER_ICON_MAP: Record<McpServerType, React.ReactNode> = {
  GOOGLE: <Google className="w-10 h-10" />,
  SLACK: <Slack className="w-10 h-10" />,
  TWITTER: (
    <IconContainer className="bg-foreground">
      <XformerlyTwitter className="w-7 h-7 text-primary-foreground" />
    </IconContainer>
  ),
  SCREENPIPE: (
    <IconContainer>
      <ComputerIcon strokeWidth={1.5} className="w-7 h-7" />
    </IconContainer>
  ),
  OTHER: <PlugIcon className="w-10 h-10" />,
  ENCHANTED: <img src={enchantedIcon} alt="Enchanted" className="w-10 h-10" />,
  FREYSA: <img src={freysaIcon} alt="Freysa" className="w-10 h-10" />
}
