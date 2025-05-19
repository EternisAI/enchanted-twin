import VoiceChatHome from '@renderer/components/voice/VoiceChatHome'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/voice/')({
  component: RouteComponent
})

function RouteComponent() {
  return <VoiceChatHome />
}
