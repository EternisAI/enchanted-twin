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
import { Check, PlugIcon, Trash2 } from 'lucide-react'
import { PROVIDER_MAP, PROVIDER_ICON_MAP } from '@renderer/constants/mcpProviders'

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
          command: server.command || 'npx',
          args: server.args || [],
          envs: server.envs || [],
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
    serverType: McpServerType,
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
    if (
      server.type === McpServerType.Enchanted ||
      server.type === McpServerType.Screenpipe ||
      server.type === McpServerType.Freysa
    ) {
      handleConnectMcpServer()
      return
    }

    if (server.type === McpServerType.Other) {
      setShowEnvInputs(enabled)
      return
    }

    if (enabled) {
      await handleOAuthFlow(server.type, startOAuthFlow)
    }
  }

  useEffect(() => {
    if (
      server.type === McpServerType.Other ||
      server.type === McpServerType.Enchanted ||
      server.type === McpServerType.Screenpipe ||
      server.type === McpServerType.Freysa
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
    <div className="p-4 w-full">
      <div className="font-semibold text-lg flex flex-wrap items-center justify-between lg:flex-row flex-col gap-5">
        <div className="flex items-center gap-5">
          <div className="w-10 h-10 rounded-md overflow-hidden flex items-center justify-center">
            {PROVIDER_ICON_MAP[server.type]}
          </div>
          <span className="font-semibold text-lg leading-none">{server.name}</span>
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
              {/* TODO: replace with Phosphor PlugChargingIcon */}
              <PlugIcon className="w-4 h-4" />
              Connect
            </Button>
          )}
          {(server.type === McpServerType.Other || server.connected) && onRemove && (
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
    </div>
  )
}
