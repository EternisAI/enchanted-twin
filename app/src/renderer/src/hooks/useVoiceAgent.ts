import { useState, useEffect, useCallback } from 'react'

type AgentState = 'initializing' | 'idle' | 'listening' | 'thinking' | 'speaking'

export default function useVoiceAgent() {
  const [agentState, setAgentState] = useState<AgentState>('idle')
  const [isSessionReady, setIsSessionReady] = useState(false)
  const [isMuted, setIsMuted] = useState(false)

  useEffect(() => {
    const cleanupSession = window.api.livekit.onSessionStateChange(({ sessionReady }) => {
      setIsSessionReady(sessionReady)
    })

    const cleanupAgentState = window.api.livekit.onAgentStateChange(({ state }) => {
      setAgentState(state as AgentState)
    })

    const initializeStates = async () => {
      try {
        const sessionReady = await window.api.livekit.isSessionReady()
        setIsSessionReady(sessionReady)

        const currentAgentState = await window.api.livekit.getAgentState()
        console.log('currentAgentState', currentAgentState)
        setAgentState(currentAgentState)
      } catch (error) {
        console.error('Failed to initialize voice agent states:', error)
      }
    }

    initializeStates()

    return () => {
      cleanupSession()
      cleanupAgentState()
    }
  }, [])

  const mute = useCallback(async () => {
    try {
      const success = await window.api.livekit.mute()
      if (success) {
        setIsMuted(true)
      }
      return success
    } catch (error) {
      console.error('Failed to mute agent:', error)
      return false
    }
  }, [])

  const unmute = useCallback(async () => {
    try {
      const success = await window.api.livekit.unmute()
      if (success) {
        setIsMuted(false)
      }
      return success
    } catch (error) {
      console.error('Failed to unmute agent:', error)
      return false
    }
  }, [])

  const toggleMute = useCallback(async () => {
    return isMuted ? await unmute() : await mute()
  }, [isMuted, mute, unmute])

  return {
    agentState,
    isSessionReady,
    isMuted,
    mute,
    unmute,
    toggleMute,
    isAgentSpeaking: agentState === 'speaking'
  }
}
