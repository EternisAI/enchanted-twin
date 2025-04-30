import { useQuery } from '@apollo/client'
import { GetMcpServersDocument } from '@renderer/graphql/generated/graphql'
import MCPServerItem from './MCPServerItem'
import { Card } from '../ui/card'

export default function MCPPanel({ header = true }: { header?: boolean }) {
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
    <Card className="flex flex-col max-w-3xl gap-4 mx-auto p-6">
      {header && (
        <div className="flex flex-col gap-2">
          <h2 className="text-2xl font-medium">Live connections</h2>
          <p className="text-muted-foreground">
            Connect your accounts to continually update your data
          </p>
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
    </Card>
  )
}
