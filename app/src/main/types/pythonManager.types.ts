import { SpawnOptions } from 'node:child_process'

export interface DependencyProgress {
  dependency: string
  progress: number
  status: string
  error?: string
}

export type AgentState = 'initializing' | 'idle' | 'listening' | 'thinking' | 'speaking'

export interface AgentStateUpdate {
  state: AgentState
  timestamp: number
}

export interface AgentCommand {
  type: 'mute' | 'unmute' | 'get_state'
  timestamp: number
}

export type RunOptions = SpawnOptions & { label: string }

export interface LiveKitAgentCallbacks {
  onProgress?: (data: DependencyProgress) => void
  onSessionReady?: (ready: boolean) => void
  onStateChange?: (state: AgentState) => void
}

export interface EnvironmentVariables {
  CHAT_ID: string
  TTS_API_KEY: string
  TTS_URL: string
  TTS_MODEL: string
  STT_API_KEY: string
  STT_URL: string
  STT_MODEL: string
  SEND_MESSAGE_URL: string
  TERM: string
  PYTHONUNBUFFERED: string
  NO_COLOR: string
  LIVEKIT_DISABLE_TERMIOS: string
}
