import { createFileRoute } from '@tanstack/react-router'
import HolonHome from '@renderer/components/holon/HolonHome'

export const Route = createFileRoute('/holon/')({
  component: HolonPage
})

export default function HolonPage() {
  return <HolonHome />
}
