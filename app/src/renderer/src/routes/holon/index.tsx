import { createFileRoute } from '@tanstack/react-router'
import Holon from '@renderer/components/holon/Holon'

export const Route = createFileRoute('/holon/')({
  component: HolonPage
})

export default function HolonPage() {
  return <Holon />
}
