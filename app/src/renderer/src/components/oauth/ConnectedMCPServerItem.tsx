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
import { Check, Unplug, InfoIcon } from 'lucide-react'
import { PROVIDER_ICON_MAP, PROVIDER_DESCRIPTION_MAP } from '@renderer/constants/mcpProviders'
import { toast } from 'sonner'

interface ConnectedMCPServerItemProps {
  server: McpServerDefinition
  onDisconnect: () => void
}

export default function ConnectedMCPServerItem({
  server,
  onDisconnect
}: ConnectedMCPServerItemProps) {
  const [isDisconnectDialogOpen, setIsDisconnectDialogOpen] = useState(false)
  const [isHovered, setIsHovered] = useState(false)
  const [isDisconnecting, setIsDisconnecting] = useState(false)

  const handleDisconnect = async () => {
    setIsDisconnecting(true)
    try {
      // Remove direct Screenpipe handling - ConnectedMCPPanel already handles this
      onDisconnect()
      setIsDisconnectDialogOpen(false)
    } catch (error) {
      console.error('[ConnectedMCPServerItem] Error during disconnect:', error)
      toast.error('Failed to disconnect properly')
    } finally {
      setIsDisconnecting(false)
    }
  }

  const getDisconnectDescription = () => {
    return 'This will disconnect the server from your application. You can reconnect it later from the Available tab.'
  }

  return (
    <div
      className="p-4 w-full hover:bg-muted rounded-md group"
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
    >
      <div className="flex items-center justify-between flex-row gap-5">
        <div className="flex items-center gap-5 flex-1 min-w-0">
          <div className="w-10 h-10 rounded-md overflow-hidden flex items-center justify-center flex-shrink-0">
            {PROVIDER_ICON_MAP[server.type]}
          </div>
          <div className="flex flex-col gap-1 flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <span className="font-semibold text-lg leading-none">{server.name}</span>
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <InfoIcon className="w-3 h-3 text-muted-foreground/50 hover:text-muted-foreground transition-colors duration-200" />
                  </TooltipTrigger>
                  <TooltipContent className="max-w-md">
                    <p>{PROVIDER_DESCRIPTION_MAP[server.type]}</p>
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>
            {server.connected && (
              <>
                {/* Extract connection identifier from envs */}
                {(() => {
                  const getConnectionIdentifier = () => {
                    if (!server.envs) return server.name

                    // Look for common identifier keys
                    const identifierKeys = [
                      'email',
                      'username',
                      'user',
                      'account',
                      'handle',
                      'workspace'
                    ]
                    for (const key of identifierKeys) {
                      const env = server.envs.find((e) => e.key.toLowerCase().includes(key))
                      if (env) return env.value
                    }

                    // Fallback to first env value or name
                    return server.envs[0]?.value || server.name
                  }

                  const identifier = getConnectionIdentifier()
                  // Only show if it's different from server name and looks like an email or meaningful identifier
                  if (
                    identifier !== server.name &&
                    (identifier.includes('@') || identifier.includes('.'))
                  ) {
                    return (
                      <div className="flex flex-wrap gap-1">
                        <span className="text-xs bg-green-500/20 text-green-600 dark:text-green-400 px-2 py-1 rounded-full">
                          {identifier}
                        </span>
                      </div>
                    )
                  }
                  return null
                })()}
              </>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2 relative flex-shrink-0">
          {/* Connected status - always present but fades out on hover */}
          <TooltipProvider>
            <Tooltip>
              <TooltipTrigger asChild>
                <Check
                  className={`w-6 h-6 text-green-600 dark:text-green-400 bg-green-500/20 rounded-full p-1 transition-opacity duration-200 ${isHovered ? 'opacity-0' : 'opacity-100'}`}
                />
              </TooltipTrigger>
              <TooltipContent>
                <p>Connected</p>
              </TooltipContent>
            </Tooltip>
          </TooltipProvider>

          {/* Disconnect button - fades in on hover */}
          <AlertDialog open={isDisconnectDialogOpen} onOpenChange={setIsDisconnectDialogOpen}>
            <AlertDialogTrigger asChild>
              <Button
                variant="outline"
                className={`absolute right-0 hover:bg-destructive/10 hover:text-destructive hover:border-destructive/30 ${isHovered ? 'opacity-100' : 'opacity-0 pointer-events-none'}`}
                onClick={() => setIsDisconnectDialogOpen(true)}
              >
                <Unplug className="w-4 h-4" />
                Disconnect
              </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
              <AlertDialogHeader>
                <AlertDialogTitle>Disconnect server</AlertDialogTitle>
                <AlertDialogDescription>{getDisconnectDescription()}</AlertDialogDescription>
              </AlertDialogHeader>
              <AlertDialogFooter>
                <AlertDialogCancel>Cancel</AlertDialogCancel>
                <Button variant="destructive" onClick={handleDisconnect} disabled={isDisconnecting}>
                  {isDisconnecting ? 'Disconnecting...' : 'Disconnect'}
                </Button>
              </AlertDialogFooter>
            </AlertDialogContent>
          </AlertDialog>
        </div>
      </div>
    </div>
  )
}
