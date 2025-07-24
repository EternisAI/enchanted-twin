import { createFileRoute } from '@tanstack/react-router'
import HolonComingSoon from '@renderer/components/holon/HolonComingSoon'

export const Route = createFileRoute('/holon/')({
  component: HolonPage
})

export default function HolonPage() {
  return <HolonComingSoon />
}
