import { useMutation, useQuery } from '@apollo/client'
import {
  CompleteOAuthFlowDocument,
  GetOAuthStatusDocument,
  StartOAuthFlowDocument
} from '@renderer/graphql/generated/graphql'
import { Button } from '../ui/button'
import { Slack, Linkedin, Twitter } from 'lucide-react'
import { useEffect } from 'react'
import { toast } from 'sonner'

type Providers = 'google' | 'twitter' | 'linkedin' | 'slack'

type Provider = {
  provider: Providers
  label: string
  icon: React.ReactNode
  scope: string
  comingSoon?: boolean
}

const providers: Provider[] = [
  {
    provider: 'google',
    label: 'Google',
    icon: (
      <div className="w-4.5 h-4.5 rounded-full border border-current flex items-center justify-center">
        G
      </div>
    ),
    scope:
      'openid email profile https://www.googleapis.com/auth/userinfo.profile https://www.googleapis.com/auth/userinfo.email'
  },
  {
    provider: 'twitter',
    label: 'X',
    icon: <Twitter />,
    scope: 'tweet.read users.read offline.access like.read'
  },
  {
    provider: 'slack',
    label: 'Slack',
    icon: <Slack />,
    scope: 'channels:read chat:write'
  },
  {
    provider: 'linkedin',
    label: 'LinkedIn',
    icon: <Linkedin />,
    scope: 'r_basicprofile',
    comingSoon: true
  }
]

export default function OAuthPanel() {
  const [completeOAuthFlow] = useMutation(CompleteOAuthFlowDocument)

  const [startOAuthFlow] = useMutation(StartOAuthFlowDocument)
  const { data } = useQuery(GetOAuthStatusDocument, {
    pollInterval: 10000
  })

  console.log('data oauth', data)

  useEffect(() => {
    window.api.onOAuthCallback(async ({ code, state }) => {
      try {
        const { data } = await completeOAuthFlow({ variables: { state, authCode: code } })

        if (data?.completeOAuthFlow) {
          console.log('OAuth completed with provider:', data.completeOAuthFlow)
          toast.success(`Connected successfully to ${data.completeOAuthFlow}!`)
        }
      } catch (err) {
        console.error('OAuth completion failed:', err)
      }
    })
  }, [completeOAuthFlow])

  const loginWithProvider = async (provider: string, scope: string) => {
    const { data } = await startOAuthFlow({ variables: { provider, scope } })
    if (data?.startOAuthFlow) {
      const { authURL, redirectURI } = data.startOAuthFlow
      window.api.openOAuthUrl(authURL, redirectURI)
    }
  }

  const oAuthStatuses = data?.getOAuthStatus ?? []

  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm text-muted-foreground">Connect your account</p>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 p-4 border rounded-lg">
        {providers.map(({ provider, label, icon, scope, comingSoon }) => {
          const statusEntry = oAuthStatuses.find((entry) => entry.provider === provider)
          const expiresAt = statusEntry?.expiresAt
          const isConnected = !!expiresAt && new Date(expiresAt) > new Date()
          const expireDate = expiresAt ? new Date(expiresAt).toLocaleString() : ''
          return (
            <Button
              key={provider}
              variant="outline"
              size="lg"
              onClick={() => loginWithProvider(provider, scope)}
              disabled={!!comingSoon || isConnected}
              className="w-full cursor-pointer flex items-center justify-center gap-2"
            >
              {icon}
              <span>{label}</span>
              {comingSoon && <span className="text-xs italic">(Coming Soon)</span>}
              {!comingSoon && isConnected && (
                <span className="text-xs italic">(Connected until {expireDate})</span>
              )}
            </Button>
          )
        })}
      </div>
    </div>
  )
}
