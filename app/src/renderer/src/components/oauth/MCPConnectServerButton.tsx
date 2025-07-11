import { PlusIcon } from 'lucide-react'
import { Button } from '../ui/button'
import { DialogHeader, DialogTitle, DialogContent, Dialog, DialogTrigger } from '../ui/dialog'
import MCPConnectionForm from './MCPConnectionForm'
import { useState } from 'react'

export default function ConnectMCPServerButton({ onSuccess }: { onSuccess: () => void }) {
  const [isConnectOpen, setIsConnectOpen] = useState(false)

  return (
    <Dialog open={isConnectOpen} onOpenChange={setIsConnectOpen}>
      <DialogTrigger asChild>
        <Button variant="default" size="lg" className="h-10 rounded-md">
          <PlusIcon className="w-4 h-4" />
          Connect MCP
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-2xl overflow-y-auto max-h-[90vh]">
        <DialogHeader>
          <DialogTitle>Connect Custom MCP Server</DialogTitle>
        </DialogHeader>
        <MCPConnectionForm
          onSuccess={() => {
            onSuccess()
            setIsConnectOpen(false)
          }}
        />
      </DialogContent>
    </Dialog>
  )
}
