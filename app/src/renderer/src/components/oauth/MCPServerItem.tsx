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
import { PlugIcon } from 'lucide-react'
import {
  PROVIDER_MAP,
  PROVIDER_ICON_MAP,
  PROVIDER_DESCRIPTION_MAP
} from '@renderer/constants/mcpProviders'
import ScreenpipeConnectionButton from '../settings/permissions/ScreenpipeConnectionButton'

interface MCPServerItemProps {
  server: McpServerDefinition
  connectedServers?: McpServerDefinition[]
  onConnect: () => void
  onRemove?: () => void
}

export default function MCPServerItem({
  server,
  connectedServers = [],
  onConnect
}: MCPServerItemProps) {
  const [showEnvInputs, setShowEnvInputs] = useState(false)
  const [authStateId, setAuthStateId] = useState<string | null>(null)

  const [startOAuthFlow] = useMutation(StartOAuthFlowDocument)
  const [completeOAuthFlow] = useMutation(CompleteOAuthFlowDocument)
  const [connectMCPServer] = useMutation(ConnectMcpServerDocument)

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
    <div className="p-4 w-full hover:bg-muted  px-6">
      <div className="flex items-center justify-between flex-row gap-5">
        <div className="flex items-start gap-5 flex-1 min-w-0">
          <div className="w-10 h-10 rounded-md overflow-hidden flex items-center justify-center flex-shrink-0">
            {PROVIDER_ICON_MAP[server.type]}
          </div>
          <div className="flex flex-col gap-1 flex-1 min-w-0">
            <span className="font-semibold text-lg leading-none">{server.name}</span>
            <p className="text-sm text-muted-foreground">{PROVIDER_DESCRIPTION_MAP[server.type]}</p>
            {connectedServers.length > 0 && (
              <div className="flex flex-wrap gap-1">
                {connectedServers.map((connectedServer) => {
                  // Extract connection identifier from envs
                  const getConnectionIdentifier = () => {
                    if (!connectedServer.envs) return connectedServer.name

                    // Look for common identifier keys
                    const identifierKeys = [
                      'email',
                      'username',
                      'user',
                      'account',
                      'handle',
                      'workspace'
                    ]
                    for (const key of identifierKeys) {
                      const env = connectedServer.envs.find((e) =>
                        e.key.toLowerCase().includes(key)
                      )
                      if (env) return env.value
                    }

                    // Fallback to first env value or name
                    return connectedServer.envs[0]?.value || connectedServer.name
                  }

                  return (
                    <span
                      key={connectedServer.id}
                      className="text-xs bg-green-500/20 text-green-600 dark:text-green-400 px-2 py-1 rounded-full"
                    >
                      {getConnectionIdentifier()}
                    </span>
                  )
                })}
              </div>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2 flex-shrink-0">
          {server.type === McpServerType.Screenpipe ? (
            <ScreenpipeConnectionButton
              onConnectionSuccess={() => {
                handleConnectMcpServer()
                onConnect()
              }}
              buttonText="Connect"
            />
          ) : (
            <Button variant="outline" onClick={() => handleEnableToolsToggle(!showEnvInputs)}>
              <PlugIcon className="w-4 h-4" />
              Connect
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}
