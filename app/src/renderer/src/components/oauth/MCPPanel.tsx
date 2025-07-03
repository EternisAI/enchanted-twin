import { useMutation, useQuery } from '@apollo/client'
import {
  GetMcpServersDocument,
  GetToolsDocument,
  RemoveMcpServerDocument,
  McpServerType
} from '@renderer/graphql/generated/graphql'
import MCPServerItem from './MCPServerItem'
import { Plug } from 'lucide-react'
import { toast } from 'sonner'
import { useEffect, useMemo } from 'react'

export default function MCPPanel({ header = true }: { header?: boolean }) {
  const { data: toolsData } = useQuery(GetToolsDocument)
  const { data, loading, error, refetch } = useQuery(GetMcpServersDocument, {
    fetchPolicy: 'network-only'
  })
  const [deleteMcpServer] = useMutation(RemoveMcpServerDocument, {
    onCompleted: () => {
      toast.success('MCP server removed')
      refetch()
    },
    onError: () => {
      toast.error('Failed to remove MCP server')
    }
  })

  const allMcpServers = useMemo(() => data?.getMCPServers || [], [data])

  // Enchanted server is only allowed if Google is connected
  const hasGoogleConnected = useMemo(
    () => allMcpServers.some((server) => server.type === McpServerType.Google && server.connected),
    [allMcpServers]
  )

  useEffect(() => {
    if (allMcpServers.length === 0) return

    const enchantedServer = allMcpServers.find(
      (server) => server.type === McpServerType.Enchanted && server.connected
    )

    if (enchantedServer && !hasGoogleConnected) {
      deleteMcpServer({ variables: { id: enchantedServer.id } })
    }
  }, [allMcpServers, hasGoogleConnected, deleteMcpServer])

  const mcpServers = useMemo(
    () =>
      allMcpServers.filter((server) => {
        if (server.type === McpServerType.Enchanted && !hasGoogleConnected) {
          return false
        }
        return true
      }),
    [allMcpServers, hasGoogleConnected]
  )

  console.log('toolsData', toolsData)

  if (loading) return <div className="py-4 text-center">Loading MCP servers...</div>
  if (error)
    return (
      <div className="py-4 text-center text-destructive">
        Error loading MCP servers: {error.message}
      </div>
    )

  return (
    <div className="flex flex-col gap-4">
      {header && (
        <div className="flex flex-col gap-2 items-center">
          <Plug className="w-6 h-6 text-primary" />
          <h2 className="text-2xl font-semibold">Connect your future</h2>
          <p className="text-muted-foreground">
            Continually update future data from your connections
          </p>
        </div>
      )}
      <div className="flex flex-col gap-4 w-full">
        {mcpServers.map((server) => (
          <MCPServerItem
            key={server.id}
            server={server}
            onConnect={refetch}
            onRemove={() => {
              deleteMcpServer({ variables: { id: server.id } })
              refetch()
            }}
          />
        ))}
        {mcpServers.length === 0 && (
          <div className="text-center text-muted-foreground py-8 border rounded-lg">
            No MCP servers configured
          </div>
        )}
      </div>
    </div>
  )
}
