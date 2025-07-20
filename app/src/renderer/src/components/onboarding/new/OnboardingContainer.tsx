import { useEffect, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'

import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { useVoiceStore } from '@renderer/lib/stores/voice'
import useMicrophonePermission from '@renderer/hooks/useMicrophonePermission'
import EnableMicrophone from './EnableMicrophone'

import VoiceOnboarding from './VoiceOnboarding'
import TTSOnboarding from './TTSOnboarding'

type OnboardingType = 'VOICE' | 'TEXT'

export default function OnboardingContainer() {
  const navigate = useNavigate()
  const { isCompleted } = useOnboardingStore()
  const { stopVoiceMode } = useVoiceStore()
  const { microphoneStatus } = useMicrophonePermission()

  const [onboardingType, setOnboardingType] = useState<OnboardingType>('VOICE')

  useEffect(() => {
    if (isCompleted) {
      console.log('isCompleted and pushing', isCompleted)
      navigate({ to: '/' })
    }
  }, [isCompleted, navigate])

  useEffect(() => {
    return () => {
      stopVoiceMode()
    }
  }, [stopVoiceMode])

  const onSkipMicrophoneAccess = () => {
    setOnboardingType('TEXT')
  }

  return (
    <div className="w-full h-full flex flex-col justify-center items-center onboarding-background">
      {microphoneStatus === 'granted' ? (
        <VoiceOnboarding />
      ) : onboardingType === 'TEXT' ? (
        <TTSOnboarding />
      ) : (
        <EnableMicrophone onSkip={onSkipMicrophoneAccess} />
      )}
    </div>
  )
}
