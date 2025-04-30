import { useQuery } from '@apollo/client'
import { GetMcpServersDocument } from '@renderer/graphql/generated/graphql'
import MCPServerItem from './MCPServerItem'
import { Plug } from 'lucide-react'

export default function MCPPanel({ hideTitle }: { hideTitle?: boolean }) {
  const { data, loading, error, refetch } = useQuery(GetMcpServersDocument)

  const mcpServers = data?.getMCPServers || []

  console.log('mcpServers', mcpServers)

  if (loading) return <div className="py-4 text-center">Loading MCP servers...</div>
  if (error)
    return (
      <div className="py-4 text-center text-destructive">
        Error loading MCP servers: {error.message}
      </div>
    )

  return (
    <div className="flex flex-col items-center max-w-3xl gap-8 mx-auto">
      {!hideTitle && (
        <div className="flex items-center gap-3 mb-4">
          <div className="bg-primary/10 p-2 rounded-full">
            <Plug className="h-5 w-5 text-primary" />
          </div>
          <h3 className="text-lg font-medium">Connections</h3>
        </div>
      )}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
        {mcpServers.map((server) => (
          <MCPServerItem key={server.id} server={server} onConnect={refetch} />
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
