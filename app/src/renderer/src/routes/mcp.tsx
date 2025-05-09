import MCPPage from '@renderer/pages/MCPPage'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/mcp')({
  component: RouteComponent
})

function RouteComponent() {
  return <MCPPage />
}
