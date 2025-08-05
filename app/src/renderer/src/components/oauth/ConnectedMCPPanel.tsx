import { useMutation, useQuery } from '@apollo/client'
import {
  GetMcpServersDocument,
  RemoveMcpServerDocument,
  McpServerDefinition,
  McpServerType
} from '@renderer/graphql/generated/graphql'
import { toast } from 'sonner'
import { useMemo } from 'react'
import { motion } from 'framer-motion'
import ConnectedMCPServerItem from './ConnectedMCPServerItem'
import { Skeleton } from '@renderer/components/ui/skeleton'
import { useScreenpipeConnection } from '@renderer/hooks/useScreenpipeConnection'

export default function ConnectedMCPPanel() {
  const { data, loading, error, refetch } = useQuery(GetMcpServersDocument, {
    fetchPolicy: 'network-only'
  })

  const { handleStopScreenpipe } = useScreenpipeConnection()

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

  const handleDisconnect = async (server: McpServerDefinition) => {
    try {
      // Special handling for Screenpipe - stop the process first
      if (server.type === McpServerType.Screenpipe) {
        console.log('[ConnectedMCPPanel] Stopping Screenpipe before disconnecting...')
        await handleStopScreenpipe()
      }

      // Then remove from MCP servers
      await deleteMcpServer({ variables: { id: server.id } })
    } catch (error) {
      console.error('[ConnectedMCPPanel] Error during disconnect:', error)
      toast.error('Failed to disconnect server properly')
    }
  }

  const ConnectedServerSkeleton = () => (
    <div className="p-4 w-full rounded-md">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-5">
          <Skeleton className="w-10 h-10 rounded-md" />
          <Skeleton className="h-6 w-32" />
        </div>
        <Skeleton className="h-6 w-6 rounded-full" />
      </div>
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
        {loading ? (
          <>
            <ConnectedServerSkeleton />
            <ConnectedServerSkeleton />
          </>
        ) : error ? (
          <div className="py-4 text-center text-destructive">
            Error loading connected servers: {error.message}
          </div>
        ) : (
          <>
            {connectedServers.map((server) => (
              <motion.div
                key={server.id}
                variants={{
                  hidden: { opacity: 0 },
                  visible: { opacity: 1 }
                }}
                transition={{ duration: 0.3, ease: 'easeOut' }}
              >
                <ConnectedMCPServerItem
                  server={server}
                  onDisconnect={() => handleDisconnect(server)}
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
          </>
        )}
      </motion.div>
    </div>
  )
}
