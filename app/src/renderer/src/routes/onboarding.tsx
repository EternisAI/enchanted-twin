import { createFileRoute } from '@tanstack/react-router'
import VoiceOnboarding from '@renderer/components/onboarding/voice/VoiceOnboarding'

export const Route = createFileRoute('/onboarding')({
  component: VoiceOnboarding
})
