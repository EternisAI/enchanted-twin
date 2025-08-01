import { useApolloClient } from '@apollo/client'
import { Button } from '@renderer/components/ui/button'
import { RefreshCw } from 'lucide-react'
import { GetChatsDocument } from '@renderer/graphql/generated/graphql'
import { useState } from 'react'

interface ChatListRefreshButtonProps {
  className?: string
  size?: 'default' | 'sm' | 'lg' | 'icon'
  variant?: 'default' | 'destructive' | 'outline' | 'secondary' | 'ghost' | 'link'
  showText?: boolean
}

export function ChatListRefreshButton({
  className,
  size = 'icon',
  variant = 'ghost',
  showText = false
}: ChatListRefreshButtonProps) {
  const client = useApolloClient()
  const [isRefreshing, setIsRefreshing] = useState(false)

  const handleRefresh = async () => {
    setIsRefreshing(true)
    try {
      // Refetch the chat list
      await client.refetchQueries({
        include: [GetChatsDocument]
      })

      // Clear cache for chat list to force fresh data
      await client.cache.evict({ fieldName: 'getChats' })
      await client.cache.gc()
    } catch (error) {
      console.error('Failed to refresh chat list:', error)
    } finally {
      setIsRefreshing(false)
    }
  }

  return (
    <Button
      variant={variant}
      size={size}
      onClick={handleRefresh}
      disabled={isRefreshing}
      className={className}
      title="Refresh chat list"
    >
      <RefreshCw className={`h-4 w-4 ${isRefreshing ? 'animate-spin' : ''}`} />
      {showText && <span className="text-sm ml-2">Refresh</span>}
    </Button>
  )
}
