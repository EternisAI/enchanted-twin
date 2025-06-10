import { createFileRoute } from '@tanstack/react-router'
import MCPPanel from '@renderer/components/oauth/MCPPanel'

export const Route = createFileRoute('/settings/connections')({
  component: ConnectionsSettings
})

function ConnectionsSettings() {
  return (
    <div className="p-8 max-w-3xl mx-auto">
      <div className="flex flex-col gap-4 w-full">
        <h3 className="text-xl font-semibold">Connections</h3>
        <p className="text-sm text-muted-foreground mb-4">
          Connect your accounts to continually update your data.
        </p>
        <MCPPanel header={false} />
      </div>
    </div>
  )
}
