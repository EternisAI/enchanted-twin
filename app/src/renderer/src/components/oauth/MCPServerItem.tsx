import { useMutation, MutationFunction } from '@apollo/client'
import {
  StartOAuthFlowDocument,
  CompleteOAuthFlowDocument,
  McpServerDefinition,
  StartOAuthFlowMutation,
  StartOAuthFlowMutationVariables,
  McpServerType,
  ConnectMcpServerDocument
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
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '../ui/tooltip'
import { Check, Trash2 } from 'lucide-react'
import icon from '../../../../../resources/icon.png'

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
  TWITTER: {
    provider: 'twitter',
    scope: 'like.read tweet.read users.read offline.access tweet.write bookmark.read'
  },
  SCREENPIPE: { provider: 'screenpipe', scope: '' },
  OTHER: { provider: 'other', scope: '' },
  ENCHANTED: { provider: 'enchanted', scope: '' }
}

const PROVIDER_ICON_MAP: Record<McpServerType, React.ReactNode> = {
  GOOGLE: <Google />,
  SLACK: <Slack />,
  TWITTER: <XformerlyTwitter />,
  SCREENPIPE: <></>,
  OTHER: <></>,
  ENCHANTED: <img src={icon} alt="Enchanted" className="w-8 h-8" />
}

interface MCPServerItemProps {
  server: McpServerDefinition
  onConnect: () => void
  onRemove?: () => void
}

export default function MCPServerItem({ server, onConnect, onRemove }: MCPServerItemProps) {
  const [showEnvInputs, setShowEnvInputs] = useState(false)
  const [authStateId, setAuthStateId] = useState<string | null>(null)
  const [isRemoveDialogOpen, setIsRemoveDialogOpen] = useState(false)

  const [startOAuthFlow] = useMutation(StartOAuthFlowDocument)
  const [completeOAuthFlow] = useMutation(CompleteOAuthFlowDocument)
  const [connectMCPServer] = useMutation(ConnectMcpServerDocument)

  const handleRemove = () => {
    if (onRemove) {
      onRemove()
      setIsRemoveDialogOpen(false)
    }
  }

  const handleConnectMcpServer = async () => {
    const { data } = await connectMCPServer({
      variables: {
        input: {
          name: server.name,
          command: 'npx',
          args: [],
          envs: [],
          type: server.type
        }
      }
    })

    if (data?.connectMCPServer) {
      toast.success(`Connected successfully to ${data.connectMCPServer}!`)
      onConnect()
    }
  }

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
    // Enchanted and Screenpipe are handled by the backend without OAuth
    if (server.type === McpServerType.Enchanted || server.type === McpServerType.Screenpipe) {
      handleConnectMcpServer()
      return
    }

    if (server.type === 'OTHER') {
      setShowEnvInputs(enabled)
      return
    }

    if (enabled) {
      await handleOAuthFlow(server.type, startOAuthFlow)
    }
  }

  useEffect(() => {
    if (
      server.type === 'OTHER' ||
      server.type === McpServerType.Enchanted ||
      server.type === McpServerType.Screenpipe
    )
      return

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

          window.api.analytics.capture('server_connected', {
            server: server.name,
            type: server.type
          })
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
      <div className="font-semibold text-lg flex flex-wrap items-center justify-between lg:flex-row flex-col gap-4">
        <div className="flex items-center gap-2">
          {PROVIDER_ICON_MAP[server.type]}
          <span className="font-semibold text-lg">{server.name}</span>
        </div>
        <div className="flex items-center gap-2">
          {server.connected ? (
            <>
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Check className="w-6 h-6 text-green-600 dark:text-green-400 bg-green-500/20 rounded-full p-1" />
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>Connected</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </>
          ) : (
            <Button
              variant="outline"
              size="sm"
              onClick={() => handleEnableToolsToggle(!showEnvInputs)}
            >
              Connect
            </Button>
          )}
          {(server.type === 'OTHER' || server.connected) && onRemove && (
            <AlertDialog open={isRemoveDialogOpen} onOpenChange={setIsRemoveDialogOpen}>
              <AlertDialogTrigger asChild>
                <TooltipProvider>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-8 w-8 hover:bg-destructive/10 hover:text-destructive rounded-full"
                        onClick={() => setIsRemoveDialogOpen(true)}
                      >
                        <Trash2 className="w-3 h-3" />
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>
                      <p>Remove connection</p>
                    </TooltipContent>
                  </Tooltip>
                </TooltipProvider>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Remove server connection</AlertDialogTitle>
                  <AlertDialogDescription>
                    This action cannot be undone. It will permanently remove the server.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>Do not delete</AlertDialogCancel>
                  <Button variant="destructive" onClick={handleRemove}>
                    Delete
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
