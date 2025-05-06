import { useMutation, MutationFunction } from '@apollo/client'
import {
  StartOAuthFlowDocument,
  CompleteOAuthFlowDocument,
  McpServerDefinition,
  StartOAuthFlowMutation,
  StartOAuthFlowMutationVariables,
  McpServerType
} from '@renderer/graphql/generated/graphql'
import { useEffect, useState } from 'react'
import { Button } from '../ui/button'
import { toast } from 'sonner'
import { Card } from '../ui/card'
import Google from '@renderer/assets/icons/google'
import Slack from '@renderer/assets/icons/slack'
import XformerlyTwitter from '@renderer/assets/icons/x'
import {
  AlertDialog,
  AlertDialogTrigger,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel
} from '../ui/alert-dialog'

const PROVIDER_MAP: Record<McpServerType, { provider: string; scope: string }> = {
  GOOGLE: {
    provider: 'google',
    scope:
      'openid email profile https://www.googleapis.com/auth/drive https://mail.google.com/ https://www.googleapis.com/auth/calendar'
  },
  SLACK: {
    provider: 'slack',
    scope: 'channels:read,groups:read,channels:history,groups:history,im:read,mpim:read,search:read'
  },
  TWITTER: { provider: 'twitter', scope: 'like.read tweet.read users.read offline.access' },
  OTHER: { provider: 'other', scope: '' }
}

const PROVIDER_ICON_MAP: Record<McpServerType, React.ReactNode> = {
  GOOGLE: <Google />,
  SLACK: <Slack />,
  TWITTER: <XformerlyTwitter />,
  OTHER: <></>
}

interface MCPServerItemProps {
  server: McpServerDefinition
  onConnect: () => void
  onRemove?: () => void
}

export default function MCPServerItem({ server, onConnect, onRemove }: MCPServerItemProps) {
  const [showEnvInputs, setShowEnvInputs] = useState(false)
  const [authStateId, setAuthStateId] = useState<string | null>(null)
  const [startOAuthFlow] = useMutation(StartOAuthFlowDocument)
  const [completeOAuthFlow] = useMutation(CompleteOAuthFlowDocument)

  async function handleOAuthFlow(
    serverType: string,
    startOAuthFlow: MutationFunction<StartOAuthFlowMutation, StartOAuthFlowMutationVariables>
  ) {
    try {
      const providerInfo = PROVIDER_MAP[serverType] || {
        provider: serverType.toLowerCase(),
        scope: ''
      }

      setAuthStateId(server.id)

      const { data } = await startOAuthFlow({
        variables: {
          provider: providerInfo.provider,
          scope: providerInfo.scope
        }
      })

      if (data?.startOAuthFlow) {
        const { authURL, redirectURI } = data.startOAuthFlow
        window.api.openOAuthUrl(authURL, redirectURI)
        return true
      }
    } catch (error) {
      console.error('Failed to start OAuth flow:', error)
      toast.error('Failed to connect to service')
      setAuthStateId(null)
    }
    return false
  }

  const handleEnableToolsToggle = async (enabled: boolean) => {
    if (server.type === 'OTHER') {
      setShowEnvInputs(enabled)
      return
    }

    if (enabled) {
      await handleOAuthFlow(server.type, startOAuthFlow)
    }
  }

  useEffect(() => {
    if (server.type === 'OTHER') return

    window.api.onOAuthCallback(async ({ code, state }) => {
      if (!authStateId) {
        console.log(`${server.name} is Skipping OAuth callback for different server`)
        return
      }

      try {
        const { data } = await completeOAuthFlow({ variables: { state, authCode: code } })
        console.log('OAuth completion data:', data)
        if (data?.completeOAuthFlow) {
          toast.success(`Connected successfully to ${data.completeOAuthFlow}!`)
          onConnect()
        }
      } catch (err) {
        console.error('OAuth completion failed:', err)
      } finally {
        setAuthStateId(null)
      }
    })
  }, [completeOAuthFlow, server.name, server.type, onConnect, authStateId])

  return (
    <Card className="p-4 w-[350px] max-w-full">
      <div className="font-semibold text-lg flex items-center justify-between">
        <div className="flex items-center gap-2">
          {PROVIDER_ICON_MAP[server.type]}
          {server.name}
        </div>
        <div className="flex flex-col gap-2">
          {server.connected ? (
            <span className="text-xs bg-green-500/20 text-green-600 px-2 py-0.5 rounded-full font-medium">
              Connected
            </span>
          ) : (
            <Button
              variant="outline"
              size="sm"
              onClick={() => handleEnableToolsToggle(!showEnvInputs)}
            >
              Connect
            </Button>
          )}
          {server.connected && onRemove && (
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="outline" size="sm">
                  Remove
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Remove server connection</AlertDialogTitle>
                  <AlertDialogDescription>
                    This action cannot be undone. It will permanently remove the server.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Do not remove</AlertDialogCancel>
                  <Button
                    variant="destructive"
                    onClick={() => {
                      onRemove()
                    }}
                  >
                    Remove
                  </Button>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          )}
        </div>
      </div>
    </Card>
  )
}
