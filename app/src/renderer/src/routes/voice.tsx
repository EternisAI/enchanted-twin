import RoomLoader from '@renderer/components/voice/Voice'
import { createFileRoute } from '@tanstack/react-router'

export const Route = createFileRoute('/voice')({
  component: AdminPage
})

function AdminPage() {
  return <RoomLoader />
}
