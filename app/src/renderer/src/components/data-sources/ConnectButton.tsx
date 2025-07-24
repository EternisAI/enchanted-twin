import { Loader2, Plus } from 'lucide-react'
import { Button } from '../ui/button'
import { useNavigate } from '@tanstack/react-router'
import { GetMcpServersDocument } from '@renderer/graphql/generated/graphql'
import { useQuery } from '@apollo/client'

import { SMALL_PROVIDER_ICON_MAP } from '@renderer/constants/mcpProviders'
import { useMemo } from 'react'
import { motion } from 'framer-motion'

export function ConnectSourcesButton() {
  const navigate = useNavigate()
  const { data, loading } = useQuery(GetMcpServersDocument, {
    fetchPolicy: 'network-only'
  })
  const allMcpServers = useMemo(() => data?.getMCPServers || [], [data])
  const availableMcpServers = useMemo(
    () => allMcpServers.filter((server) => !server.connected),
    [allMcpServers]
  )

  if (availableMcpServers.length === 0) {
    return null
  }

  const containerVariants = {
    hidden: { opacity: 0 },
    visible: {
      opacity: 1,
      transition: {
        staggerChildren: 0.1
      }
    }
  }
  const itemVariants = {
    hidden: { opacity: 0, x: -10 },
    visible: { opacity: 1, x: 0 }
  }

  return (
    <Button
      onClick={() => {
        navigate({
          to: '/settings/data-sources'
        })
      }}
      variant="outline"
      className="group"
      size="sm"
    >
      <Plus className="w-4 h-4" />
      Connect Sources
      <motion.div className="flex items-center">
        {loading ? (
          <Loader2 className="size-4 animate-spin" />
        ) : (
          <motion.div
            className="flex flex-row items-center -space-x-1.5"
            variants={containerVariants}
            initial="hidden"
            animate="visible"
          >
            {availableMcpServers.map((server) => (
              <motion.div
                className="rounded-sm overflow-hidden bg-white group-hover:bg-muted p-0.5 transition-all duration-200 ease-in-out"
                key={server.id}
                variants={itemVariants}
              >
                {SMALL_PROVIDER_ICON_MAP[server.type]}
              </motion.div>
            ))}
          </motion.div>
        )}
      </motion.div>
    </Button>
  )
}
