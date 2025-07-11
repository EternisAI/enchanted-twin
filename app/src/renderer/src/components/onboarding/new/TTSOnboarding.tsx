import { useEffect, useState } from 'react'

import { Role } from '@renderer/graphql/generated/graphql'

import { auth } from '@renderer/lib/firebase'
import MessageInput from '@renderer/components/chat/MessageInput'
import { useProcessMessageHistoryStream } from '@renderer/hooks/useProcessMessageHistoryStream'
import useOnboardingChat, { INITIAL_AGENT_MESSAGE } from '@renderer/hooks/useOnboardingChat'
import { MessageDisplay, OnboardingBase } from './contexts/OnboardingBase'

export default function TTSOnboarding() {
  const {
    lastMessage,
    lastAgentMessage,
    chatId,
    triggerAnimation,
    createOnboardingChat,
    skipOnboarding
  } = useOnboardingChat()
  const [isTTSPlaying, setIsTTSPlaying] = useState(false)
  const [messageHistory, setMessageHistory] = useState<Array<{ text: string; role: Role }>>([])
  const [streamingResponse, setStreamingResponse] = useState('')
  const [currentAudio, setCurrentAudio] = useState<HTMLAudioElement | null>(null)

  const handleSendMessage = async (text: string) => {
    console.log('[TTSOnboarding] Sending message:', text)
    const newMessageHistory = [...messageHistory, { text, role: Role.User }]
    setMessageHistory(newMessageHistory)
  }

  useEffect(() => {
    const initializeTTSOnboarding = async () => {
      generateTTSForResponse(INITIAL_AGENT_MESSAGE)
      await createOnboardingChat()
    }
    initializeTTSOnboarding()
  }, [])

  const handleResponseChunk = (messageId: string, chunk: string, isComplete: boolean) => {
    setStreamingResponse((prev) => prev + chunk)

    if (isComplete) {
      console.log('[TTSOnboarding] Response complete, generating TTS')
      const completeResponse = streamingResponse + chunk
      generateTTSForResponse(completeResponse)
      setStreamingResponse('')
    }
  }

  const stopTTS = () => {
    if (currentAudio) {
      console.log('[TTS] Stopping audio playback')
      currentAudio.pause()
      currentAudio.currentTime = 0
      setIsTTSPlaying(false)
      setCurrentAudio(null)
    }
  }

  const generateTTSForResponse = async (responseText: string) => {
    try {
      const firebaseToken = await auth.currentUser?.getIdToken()

      if (!firebaseToken) {
        console.error('[TTS] No Firebase token available')
        return
      }

      const ttsResult = await window.api.tts.generate(responseText, firebaseToken)

      if (!ttsResult.success) {
        console.error('[TTS] Failed to generate TTS:', ttsResult.error)
        return
      }

      if (!ttsResult.audioBuffer) {
        console.error('[TTS] No audio buffer returned')
        return
      }

      const audioBlob = new Blob([ttsResult.audioBuffer], { type: 'audio/mpeg' })

      if (audioBlob) {
        console.log('[TTS] Successfully generated audio, size:', audioBlob.size)
        const audioUrl = URL.createObjectURL(audioBlob)
        const audio = new Audio(audioUrl)

        setCurrentAudio(audio)

        audio.addEventListener('loadeddata', () => {
          console.log('[TTS] Audio loaded, duration:', audio.duration)
        })

        audio.addEventListener('ended', () => {
          console.log('[TTS] Audio playback finished')
          URL.revokeObjectURL(audioUrl)
          setIsTTSPlaying(false)
          setCurrentAudio(null)
        })

        audio.addEventListener('error', (e) => {
          console.error('[TTS] Audio playback error:', e)
          URL.revokeObjectURL(audioUrl)
          setIsTTSPlaying(false)
          setCurrentAudio(null)
        })

        await audio.play()
        setIsTTSPlaying(true)
        console.log('[TTS] Started audio playback')
      } else {
        console.error('[TTS] Failed to generate audio')
      }
    } catch (error) {
      console.error('[TTS] Error generating TTS:', error)
    }
  }

  useProcessMessageHistoryStream(chatId, messageHistory, true, handleResponseChunk)

  return (
    <OnboardingBase
      isAnimationRunning={isTTSPlaying}
      triggerAnimation={triggerAnimation}
      onSkip={skipOnboarding}
    >
      <MessageDisplay lastAgentMessage={lastAgentMessage} lastMessage={lastMessage}>
        <MessageInput
          onSend={handleSendMessage}
          isWaitingTwinResponse={isTTSPlaying}
          isReasonSelected={false}
          voiceMode
          onStop={stopTTS}
        />
      </MessageDisplay>
    </OnboardingBase>
  )
}
