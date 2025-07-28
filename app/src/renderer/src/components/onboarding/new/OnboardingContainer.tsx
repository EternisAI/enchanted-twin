import { useEffect, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'

import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { useVoiceStore } from '@renderer/lib/stores/voice'
import useMicrophonePermission from '@renderer/hooks/useMicrophonePermission'
import EnableMicrophone from './EnableMicrophone'
import { checkVoiceDisabled } from '@renderer/lib/utils'

import VoiceOnboarding from './VoiceOnboarding'
import TTSOnboarding from './TTSOnboarding'

type OnboardingType = 'VOICE' | 'TEXT'

export default function OnboardingContainer() {
  const navigate = useNavigate()
  const { isCompleted, completeOnboarding } = useOnboardingStore()
  const { stopVoiceMode } = useVoiceStore()
  const { microphoneStatus } = useMicrophonePermission()
  const [onboardingType, setOnboardingType] = useState<OnboardingType>('VOICE')
  const [isOnboardingDisabled, setIsOnboardingDisabled] = useState(false)
  const [isLoading, setIsLoading] = useState(true)

  useEffect(() => {
    const checkVoice = async () => {
      const disabled = await checkVoiceDisabled()
      setIsOnboardingDisabled(disabled)
      setOnboardingType(disabled ? 'TEXT' : 'VOICE')
      setIsLoading(false)
    }
    checkVoice()
  }, [])

  useEffect(() => {
    if (isLoading) return

    if (isCompleted || isOnboardingDisabled) {
      console.log('isCompleted and pushing', isCompleted, isOnboardingDisabled)
      completeOnboarding()
      stopVoiceMode()
      navigate({ to: '/' })
    }
  }, [isCompleted, navigate, isLoading, isOnboardingDisabled])

  useEffect(() => {
    return () => {
      stopVoiceMode()
    }
  }, [stopVoiceMode])

  const onSkipMicrophoneAccess = () => {
    setOnboardingType('TEXT')
  }

  if (isLoading || isOnboardingDisabled) {
    return <></>
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
