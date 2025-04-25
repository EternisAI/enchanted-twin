import { useQuery } from '@apollo/client'
import { GetMcpServersDocument } from '@renderer/graphql/generated/graphql'
import MCPServerItem from './MCPServerItem'

export default function MCPPanel() {
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
    <div className="flex flex-col max-w-3xl gap-3 mx-auto">
      <h2 className="text-xl font-bold mb-6">MCP Servers</h2>
      <div className="flex flex-wrap gap-10">
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
