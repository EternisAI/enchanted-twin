import { useMutation, useQuery } from '@apollo/client'
import {
  GetMcpServersDocument,
  GetToolsDocument,
  RemoveMcpServerDocument
} from '@renderer/graphql/generated/graphql'
import MCPServerItem from './MCPServerItem'
import { Card } from '../ui/card'
import { Plug } from 'lucide-react'
import { toast } from 'sonner'
import ConnectMCPServerButton from './MCPConnectServerButton'

export default function MCPPanel({ header = true }: { header?: boolean }) {
  const { data: toolsData } = useQuery(GetToolsDocument)
  const { data, loading, error, refetch } = useQuery(GetMcpServersDocument)
  const [deleteMcpServer] = useMutation(RemoveMcpServerDocument, {
    onCompleted: () => {
      toast.success('MCP server removed')
      refetch()
    },
    onError: () => {
      toast.error('Failed to remove MCP server')
    }
  })

  const mcpServers = data?.getMCPServers || []

  console.log('toolsData', toolsData)

  if (loading) return <div className="py-4 text-center">Loading MCP servers...</div>
  if (error)
    return (
      <div className="py-4 text-center text-destructive">
        Error loading MCP servers: {error.message}
      </div>
    )

  return (
    <Card className="flex flex-col max-w-4xl gap-4 mx-auto p-6">
      {header && (
        <div className="flex flex-col gap-2 items-center">
          <Plug className="w-6 h-6 text-primary" />
          <h2 className="text-2xl font-semibold">Connect your future</h2>
          <p className="text-muted-foreground">
            Continually update future data from your connections
          </p>
        </div>
      )}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-5">
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
        <ConnectMCPServerButton onSuccess={refetch} />
        {mcpServers.length === 0 && (
          <div className="text-center text-muted-foreground py-8 border rounded-lg">
            No MCP servers configured
          </div>
        )}
      </div>
    </Card>
  )
}
