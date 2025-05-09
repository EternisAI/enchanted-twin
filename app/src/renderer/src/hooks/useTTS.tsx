import { createContext, useContext } from 'react'
import { TTSAPI } from '@renderer/lib/ttsProvider'

export const TTSContext = createContext<TTSAPI | undefined>(undefined)

export function useTTS(): TTSAPI {
  const ctx = useContext(TTSContext)
  if (!ctx) throw new Error('useTTS must be used inside <TTSProvider>')
  return ctx
}
