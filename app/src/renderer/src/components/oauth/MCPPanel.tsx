import { useMutation, useQuery } from '@apollo/client'
import {
  GetMcpServersDocument,
  RemoveMcpServerDocument,
  McpServerType
} from '@renderer/graphql/generated/graphql'
import MCPServerItem from './MCPServerItem'
import { toast } from 'sonner'
import { useMemo, useEffect, useState } from 'react'
import { Skeleton } from '@renderer/components/ui/skeleton'
import { motion } from 'framer-motion'
import { PROVIDER_CONFIG } from '@renderer/constants/mcpProviders'
import ConnectMCPServerButton from './MCPConnectServerButton'
import { useRouter } from '@tanstack/react-router'

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

export default function MCPPanel() {
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

  const router = useRouter()

  // Check for screenpipe hash fragment to auto-open modal
  const [shouldAutoOpenScreenpipe, setShouldAutoOpenScreenpipe] = useState(false)

  useEffect(() => {
    // Check for #screenpipe hash fragment on mount
    if (window.location.hash === '#screenpipe') {
      setShouldAutoOpenScreenpipe(true)
      // Clear the hash after detecting it
      router.history.replace(window.location.pathname + window.location.search)
    }
  }, [router])

  const allMcpServers = useMemo(() => data?.getMCPServers || [], [data])

  const serversByType = useMemo(() => {
    const grouped = allMcpServers.reduce(
      (acc, server) => {
        if (!acc[server.type]) {
          acc[server.type] = []
        }
        acc[server.type]!.push(server)
        return acc
      },
      {} as Partial<Record<McpServerType, typeof allMcpServers>>
    )

    return grouped
  }, [allMcpServers])

  const serverTypes = useMemo(() => {
    return Object.keys(serversByType)
      .map((type) => {
        const servers = serversByType[type as McpServerType]!
        const connectedServers = servers.filter((s) => s.connected)
        const templateServer = servers[0]!
        const serverType = type as McpServerType
        const providerConfig = PROVIDER_CONFIG[serverType]

        return {
          type: serverType,
          templateServer,
          connectedServers,
          totalServers: servers.length,
          supportsMultipleConnections: providerConfig.supportsMultipleConnections
        }
      })
      .filter((serverType) => {
        // Show servers that either:
        // 1. Have no connections yet, OR
        // 2. Support multiple connections
        return serverType.connectedServers.length === 0 || serverType.supportsMultipleConnections
      })
  }, [serversByType])

  return (
    <div className="flex flex-col gap-4 p-0">
      <header className="flex gap-2 justify-between items-center pb-4 pr-2">
        <div className="flex flex-col gap-2">
          <h2 className="text-2xl font-bold leading-none">Quick Connects</h2>
          <p className="text-muted-foreground leading-none text-sm">
            Quick connect to your favorite apps and services.
          </p>
        </div>
        <ConnectMCPServerButton onSuccess={() => {}} />
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
                  shouldAutoOpenScreenpipe={
                    serverType.type === McpServerType.Screenpipe && shouldAutoOpenScreenpipe
                  }
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
