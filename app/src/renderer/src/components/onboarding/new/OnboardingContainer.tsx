import { useEffect, useState } from 'react'
import { useNavigate } from '@tanstack/react-router'

import { useTheme } from '@renderer/lib/theme'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { useVoiceStore } from '@renderer/lib/stores/voice'
import useMicrophonePermission from '@renderer/hooks/useMicrophonePermission'
import EnableMicrophone from './EnableMicrophone'

import VoiceOnboarding from './VoiceOnboarding'
import TTSOnboarding from './TTSOnboarding'

type OnboardingType = 'VOICE' | 'TEXT'

export default function OnboardingContainer() {
  const navigate = useNavigate()
  const { theme } = useTheme()
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
    <div
      className="w-full h-full flex flex-col justify-center items-center"
      style={{
        background:
          theme === 'light'
            ? 'linear-gradient(180deg, #6068E9 0%, #A5AAF9 100%)'
            : 'linear-gradient(180deg, #18181B 0%, #000 100%)'
      }}
    >
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
