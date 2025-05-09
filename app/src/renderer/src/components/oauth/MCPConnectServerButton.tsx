import { Plus } from 'lucide-react'
import { Card } from '../ui/card'
import { Button } from '../ui/button'
import { DialogHeader, DialogTitle } from '../ui/dialog'
import { DialogContent } from '../ui/dialog'
import { Dialog, DialogTrigger } from '../ui/dialog'
import MCPConnectionForm from './MCPConnectionForm'
import { useState } from 'react'

export default function ConnectMCPServerButton({ onSuccess }: { onSuccess: () => void }) {
  const [isConnectOpen, setIsConnectOpen] = useState(false)

  return (
    <Dialog open={isConnectOpen} onOpenChange={setIsConnectOpen}>
      <DialogTrigger asChild>
        <Card className="p-4 w-[350px] max-w-full">
          <div className="font-semibold text-lg flex items-center justify-between">
            <div className="flex items-center gap-1">
              <Plus className="w-5 h-5" />
              Custom MCP
            </div>
            <div className="flex flex-col gap-2">
              <Button variant="outline" size="sm">
                Connect
              </Button>
            </div>
          </div>
        </Card>
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
