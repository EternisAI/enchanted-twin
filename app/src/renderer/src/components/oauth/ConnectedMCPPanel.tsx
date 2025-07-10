import { useMutation, useQuery } from '@apollo/client'
import {
  GetMcpServersDocument,
  RemoveMcpServerDocument,
  McpServerDefinition
} from '@renderer/graphql/generated/graphql'
import { toast } from 'sonner'
import { useMemo } from 'react'
import { motion } from 'framer-motion'
import ConnectedMCPServerItem from './ConnectedMCPServerItem'

export default function ConnectedMCPPanel() {
  const { data, loading, error, refetch } = useQuery(GetMcpServersDocument, {
    fetchPolicy: 'network-only'
  })

  const [deleteMcpServer] = useMutation(RemoveMcpServerDocument, {
    onCompleted: () => {
      toast.success('MCP server disconnected')
      refetch()
    },
    onError: () => {
      toast.error('Failed to disconnect MCP server')
    }
  })

  const connectedServers = useMemo(() => {
    return (data?.getMCPServers || []).filter((server: McpServerDefinition) => server.connected)
  }, [data])

  if (loading) return <div className="py-4 text-center">Loading connected servers...</div>
  if (error)
    return (
      <div className="py-4 text-center text-destructive">
        Error loading connected servers: {error.message}
      </div>
    )

  return (
    <div className="flex flex-col gap-4">
      <motion.div
        className="flex flex-col gap-4 w-full"
        initial="hidden"
        animate="visible"
        variants={{
          hidden: { opacity: 0 },
          visible: {
            opacity: 1,
            transition: {
              staggerChildren: 0.1,
              delayChildren: 0.1
            }
          }
        }}
      >
        {connectedServers.map((server) => (
          <motion.div
            key={server.id}
            variants={{
              hidden: { opacity: 0, y: 20 },
              visible: { opacity: 1, y: 0 }
            }}
            transition={{ duration: 0.3, ease: 'easeOut' }}
          >
            <ConnectedMCPServerItem
              server={server}
              onDisconnect={() => {
                deleteMcpServer({ variables: { id: server.id } })
              }}
            />
          </motion.div>
        ))}
        {connectedServers.length === 0 && (
          <motion.div
            className="text-center text-muted-foreground py-8 border rounded-lg"
            variants={{
              hidden: { opacity: 0, y: 20 },
              visible: { opacity: 1, y: 0 }
            }}
            transition={{ duration: 0.3, ease: 'easeOut' }}
          >
            No connected MCP servers. Connect some servers from the Available tab.
          </motion.div>
        )}
      </motion.div>
    </div>
  )
}
