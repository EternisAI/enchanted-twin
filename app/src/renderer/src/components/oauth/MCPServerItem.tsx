import { useMutation, MutationFunction } from '@apollo/client'
import {
  StartOAuthFlowDocument,
  CompleteOAuthFlowDocument,
  McpServerDefinition,
  StartOAuthFlowMutation,
  StartOAuthFlowMutationVariables,
  McpServerType,
  ConnectMcpServerDocument,
  CompleteOAuthFlowCompositionDocument
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

interface ProviderConfig {
  provider: string
  scope: string
  icon: React.ReactNode
  connectMethod: 'oauth' | 'direct' | 'env'
  completionMethod: 'standard' | 'composition' | 'none'
  needsOAuthCallback: boolean
}

const PROVIDER_CONFIG: Record<McpServerType, ProviderConfig> = {
  GOOGLE: {
    provider: 'google',
    scope:
      'openid email profile https://www.googleapis.com/auth/drive https://mail.google.com/ https://www.googleapis.com/auth/calendar',
    icon: <Google />,
    connectMethod: 'oauth',
    completionMethod: 'standard',
    needsOAuthCallback: true
  },
  SLACK: {
    provider: 'slack',
    scope: 'channels:read,groups:read,channels:history,groups:history,im:read,mpim:read,search:read,users:read',
    icon: <Slack />,
    connectMethod: 'oauth',
    completionMethod: 'standard',
    needsOAuthCallback: true
  },
  TWITTER: {
    provider: 'twitter',
    scope: 'like.read tweet.read users.read offline.access tweet.write bookmark.read',
    icon: <XformerlyTwitter />,
    connectMethod: 'oauth',
    completionMethod: 'composition',
    needsOAuthCallback: true
  },
  SCREENPIPE: {
    provider: 'screenpipe',
    scope: '',
    icon: <></>,
    connectMethod: 'direct',
    completionMethod: 'none',
    needsOAuthCallback: false
  },
  OTHER: {
    provider: 'other',
    scope: '',
    icon: <></>,
    connectMethod: 'env',
    completionMethod: 'none',
    needsOAuthCallback: false
  },
  ENCHANTED: {
    provider: 'enchanted',
    scope: '',
    icon: <img src={icon} alt="Enchanted" className="w-8 h-8" />,
    connectMethod: 'direct',
    completionMethod: 'none',
    needsOAuthCallback: false
  }
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
  const [completeOAuthFlowComposition] = useMutation(CompleteOAuthFlowCompositionDocument)
  const [connectMCPServer] = useMutation(ConnectMcpServerDocument)

  const config = PROVIDER_CONFIG[server.type]

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
    config: ProviderConfig,
    startOAuthFlow: MutationFunction<StartOAuthFlowMutation, StartOAuthFlowMutationVariables>
  ) {
    try {
      setAuthStateId(server.id)

      const { data } = await startOAuthFlow({
        variables: {
          provider: config.provider,
          scope: config.scope
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
    switch (config.connectMethod) {
      case 'direct':
        handleConnectMcpServer()
        break
      case 'env':
        setShowEnvInputs(enabled)
        break
      case 'oauth':
        if (enabled) {
          await handleOAuthFlow(config, startOAuthFlow)
        }
        break
    }
  }

  const handleOAuthCompletion = async (data: {
    code: string
    state: string
    connectedAccountId: string
  }) => {
    try {
      if (config.completionMethod === 'composition') {
        const { data: completionData } = await completeOAuthFlowComposition({
          variables: { accountId: data.connectedAccountId, provider: config.provider }
        })
        console.log('OAuth completion data:', completionData)
        if (completionData?.completeOAuthFlowComposio) {
          toast.success(`Connected successfully to ${completionData.completeOAuthFlowComposio}!`)
        }
      } else if (config.completionMethod === 'standard') {
        const { data: completionData } = await completeOAuthFlow({
          variables: { state: data.state, authCode: data.code }
        })
        console.log('OAuth completion data:', completionData)
        if (completionData?.completeOAuthFlow) {
          toast.success(`Connected successfully to ${completionData.completeOAuthFlow}!`)
        }
      }
    } catch (err) {
      console.error('OAuth completion failed:', err)
    } finally {
      onConnect()
      window.api.analytics.capture('server_connected', {
        server: server.name,
        type: server.type
      })
      setAuthStateId(null)
    }
  }

  useEffect(() => {
    if (!config.needsOAuthCallback) return

    window.api.onOAuthCallback(async ({ code, state, connectedAccountId }) => {
      if (!authStateId) {
        console.log(`${server.name} is Skipping OAuth callback for different server`)
        return
      }

      await handleOAuthCompletion({ code, state, connectedAccountId })
    })
  }, [
    completeOAuthFlow,
    completeOAuthFlowComposition,
    server.name,
    server.type,
    onConnect,
    authStateId,
    config.needsOAuthCallback
  ])

  return (
    <Card className="p-4 w-[350px] max-w-full">
      <div className="font-semibold text-lg flex flex-wrap items-center justify-between lg:flex-row flex-col gap-4">
        <div className="flex items-center gap-2">
          {config.icon}
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
