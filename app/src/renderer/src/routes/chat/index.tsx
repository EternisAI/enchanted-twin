import ChatHome from '@renderer/components/chat/ChatHome'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/chat/')({
  component: RouteComponent
})

function RouteComponent() {
  return (
    <div className="h-full p-4">
      <ChatHome />
    </div>
  )
}
