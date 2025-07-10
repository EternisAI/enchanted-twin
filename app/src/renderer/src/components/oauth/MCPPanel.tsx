import { useMutation, useQuery } from '@apollo/client'
import {
  GetMcpServersDocument,
  GetToolsDocument,
  RemoveMcpServerDocument,
  McpServerType
} from '@renderer/graphql/generated/graphql'
import MCPServerItem from './MCPServerItem'
import { toast } from 'sonner'
import { useEffect, useMemo } from 'react'
import { Skeleton } from '@renderer/components/ui/skeleton'
import { motion } from 'framer-motion'

export default function MCPPanel() {
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

  const serversByType = useMemo(() => {
    const grouped = allMcpServers.reduce(
      (acc, server) => {
        if (!acc[server.type]) {
          acc[server.type] = []
        }
        acc[server.type].push(server)
        return acc
      },
      {} as Record<McpServerType, typeof allMcpServers>
    )

    // Filter out Enchanted servers when Google isn't connected
    // if (!hasGoogleConnected && grouped[McpServerType.Enchanted]) {
    //   delete grouped[McpServerType.Enchanted]
    // }

    return grouped
  }, [allMcpServers])

  const serverTypes = useMemo(() => {
    return Object.keys(serversByType).map((type) => {
      const servers = serversByType[type as McpServerType]
      const connectedServers = servers.filter((s) => s.connected)
      const templateServer = servers[0] // Use first server as template for type info

      return {
        type: type as McpServerType,
        templateServer,
        connectedServers,
        totalServers: servers.length
      }
    })
  }, [serversByType])

  console.log('toolsData', toolsData)

  const MCPServerSkeleton = () => (
    <div className="p-4 w-full rounded-md">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-5">
          <Skeleton className="w-10 h-10 rounded-md" />
          <Skeleton className="h-6 w-32" />
        </div>
        <Skeleton className="h-8 w-20" />
      </div>
    </div>
  )

  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-col gap-2 border-b pb-3">
        <h2 className="text-2xl font-bold leading-none">Available Servers</h2>
        <p className="text-muted-foreground leading-none text-sm">
          Connect to MCP servers to extend your capabilities.
        </p>
      </header>
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
            <MCPServerSkeleton />
            <MCPServerSkeleton />
            <MCPServerSkeleton />
          </>
        ) : error ? (
          <motion.div
            className="py-4 text-center text-destructive"
            variants={{
              hidden: { opacity: 0, y: 20 },
              visible: { opacity: 1, y: 0 }
            }}
            transition={{ duration: 0.3, ease: 'easeOut' }}
          >
            Error loading MCP servers: {error.message}
          </motion.div>
        ) : (
          <>
            {serverTypes.map((serverType) => (
              <motion.div
                key={serverType.type}
                variants={{
                  hidden: { opacity: 0 },
                  visible: { opacity: 1 }
                }}
                transition={{ duration: 0.2, ease: 'easeOut' }}
              >
                <MCPServerItem
                  server={serverType.templateServer}
                  connectedServers={serverType.connectedServers}
                  onConnect={refetch}
                  onRemove={() => {
                    deleteMcpServer({ variables: { id: serverType.templateServer.id } })
                    refetch()
                  }}
                />
              </motion.div>
            ))}
            {serverTypes.length === 0 && (
              <motion.div
                className="text-center text-muted-foreground py-8 border rounded-lg"
                variants={{
                  hidden: { opacity: 0 },
                  visible: { opacity: 1 }
                }}
                transition={{ duration: 0.2, ease: 'easeOut' }}
              >
                No MCP servers available
              </motion.div>
            )}
          </>
        )}
      </motion.div>
    </div>
  )
}
