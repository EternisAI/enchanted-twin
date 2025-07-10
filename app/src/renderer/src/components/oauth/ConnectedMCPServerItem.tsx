import { McpServerDefinition } from '@renderer/graphql/generated/graphql'
import { useState } from 'react'
import { Button } from '../ui/button'
import {
  AlertDialog,
  AlertDialogTrigger,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogCancel
} from '../ui/alert-dialog'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '../ui/tooltip'
import { Check, Unplug } from 'lucide-react'
import { PROVIDER_ICON_MAP, PROVIDER_DESCRIPTION_MAP } from '@renderer/constants/mcpProviders'

interface ConnectedMCPServerItemProps {
  server: McpServerDefinition
  onDisconnect: () => void
}

export default function ConnectedMCPServerItem({ server, onDisconnect }: ConnectedMCPServerItemProps) {
  const [isDisconnectDialogOpen, setIsDisconnectDialogOpen] = useState(false)
  const [isHovered, setIsHovered] = useState(false)

  const handleDisconnect = () => {
    onDisconnect()
    setIsDisconnectDialogOpen(false)
  }

  return (
    <div 
      className="p-4 w-full hover:bg-muted rounded-md group"
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
    >
      <div className="font-semibold text-lg flex items-center justify-between flex-row gap-5">
        <div className="flex items-center gap-5 flex-1 min-w-0">
          <div className="w-10 h-10 rounded-md overflow-hidden flex items-center justify-center flex-shrink-0">
            {PROVIDER_ICON_MAP[server.type]}
          </div>
          <div className="flex flex-col gap-1 flex-1 min-w-0">
            <span className="font-semibold text-lg leading-none">{server.name}</span>
            <p className="text-sm text-muted-foreground leading-relaxed">
              {PROVIDER_DESCRIPTION_MAP[server.type]}
            </p>
            {server.connected && (
              <div className="flex flex-wrap gap-1">
                {/* Extract connection identifier from envs */}
                {(() => {
                  const getConnectionIdentifier = () => {
                    if (!server.envs) return server.name
                    
                    // Look for common identifier keys
                    const identifierKeys = ['email', 'username', 'user', 'account', 'handle', 'workspace']
                    for (const key of identifierKeys) {
                      const env = server.envs.find(e => e.key.toLowerCase().includes(key))
                      if (env) return env.value
                    }
                    
                    // Fallback to first env value or name
                    return server.envs[0]?.value || server.name
                  }
                  
                  const identifier = getConnectionIdentifier()
                  // Only show if it's different from server name and looks like an email or meaningful identifier
                  if (identifier !== server.name && (identifier.includes('@') || identifier.includes('.'))) {
                    return (
                      <span className="text-xs bg-green-500/20 text-green-600 dark:text-green-400 px-2 py-1 rounded-full">
                        {identifier}
                      </span>
                    )
                  }
                  return null
                })()}
              </div>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2 relative flex-shrink-0">
          {/* Connected status - always present but fades out on hover */}
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Check className={`w-6 h-6 text-green-600 dark:text-green-400 bg-green-500/20 rounded-full p-1 transition-opacity duration-200 ${isHovered ? 'opacity-0' : 'opacity-100'}`} />
              </TooltipTrigger>
              <TooltipContent>
                <p>Connected</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>
          
          {/* Disconnect button - fades in on hover */}
          <AlertDialog open={isDisconnectDialogOpen} onOpenChange={setIsDisconnectDialogOpen}>
            <AlertDialogTrigger asChild>
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="outline"
                      size="sm"
                      className={`absolute right-0 hover:bg-destructive/10 hover:text-destructive hover:border-destructive/30 transition-opacity duration-200 ${isHovered ? 'opacity-100' : 'opacity-0 pointer-events-none'}`}
                      onClick={() => setIsDisconnectDialogOpen(true)}
                    >
                      <Unplug className="w-4 h-4" />
                      Disconnect
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>Disconnect server</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Disconnect server</AlertDialogTitle>
                <AlertDialogDescription>
                  This will disconnect the server from your application. You can reconnect it later from the Available tab.
                </AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <Button variant="destructive" onClick={handleDisconnect}>
                  Disconnect
                </Button>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </div>
    </div>
  )
}