import ChatContainer from '@renderer/components/chat/ChatContainer'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/chat')({
  component: RouteComponent
})

function RouteComponent() {
  return <ChatContainer />
}
