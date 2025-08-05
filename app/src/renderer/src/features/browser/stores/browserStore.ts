import { create } from 'zustand'
import { subscribeWithSelector } from 'zustand/middleware'
import { BrowserSession, BrowserState, BrowserInteraction } from '../types/browser.types'

interface BrowserStore extends BrowserState {
  // Session management
  createSession: (url: string) => string
  updateSession: (sessionId: string, updates: Partial<BrowserSession>) => void
  deleteSession: (sessionId: string) => void
  setActiveSession: (sessionId: string | null) => void

  // Interaction tracking
  addInteraction: (sessionId: string, interaction: Omit<BrowserInteraction, 'id'>) => void

  // State management
  setLoading: (loading: boolean) => void
  setError: (error: string | null) => void
  reset: () => void
}

const initialState: BrowserState = {
  sessions: new Map(),
  activeSessionId: null,
  isLoading: false,
  error: null
}

export const useBrowserStore = create<BrowserStore>()(
  subscribeWithSelector((set, get) => ({
    ...initialState,

    createSession: (url: string) => {
      const sessionId = `browser-session-${Date.now()}`
      const newSession: BrowserSession = {
        id: sessionId,
        url,
        title: '',
        content: {
          text: '',
          html: ''
        },
        metadata: {
          timestamp: new Date(),
          scrollPosition: { x: 0, y: 0 },
          viewportSize: { width: 0, height: 0 }
        },
        interactions: []
      }

      set((state) => {
        const newSessions = new Map(state.sessions)
        newSessions.set(sessionId, newSession)
        return {
          sessions: newSessions,
          activeSessionId: sessionId
        }
      })

      return sessionId
    },

    updateSession: (sessionId: string, updates: Partial<BrowserSession>) => {
      set((state) => {
        const session = state.sessions.get(sessionId)
        if (!session) return state

        const updatedSession = {
          ...session,
          ...updates,
          content: updates.content ? { ...session.content, ...updates.content } : session.content,
          metadata: updates.metadata
            ? { ...session.metadata, ...updates.metadata }
            : session.metadata
        }

        const newSessions = new Map(state.sessions)
        newSessions.set(sessionId, updatedSession)

        return { sessions: newSessions }
      })
    },

    deleteSession: (sessionId: string) => {
      set((state) => {
        const newSessions = new Map(state.sessions)
        newSessions.delete(sessionId)

        return {
          sessions: newSessions,
          activeSessionId: state.activeSessionId === sessionId ? null : state.activeSessionId
        }
      })
    },

    setActiveSession: (sessionId: string | null) => {
      set({ activeSessionId: sessionId })
    },

    addInteraction: (sessionId: string, interaction: Omit<BrowserInteraction, 'id'>) => {
      set((state) => {
        const session = state.sessions.get(sessionId)
        if (!session) return state

        const newInteraction: BrowserInteraction = {
          ...interaction,
          id: `interaction-${Date.now()}`
        }

        const updatedSession = {
          ...session,
          interactions: [...session.interactions, newInteraction]
        }

        const newSessions = new Map(state.sessions)
        newSessions.set(sessionId, updatedSession)

        return { sessions: newSessions }
      })
    },

    setLoading: (loading: boolean) => set({ isLoading: loading }),
    setError: (error: string | null) => set({ error }),

    reset: () => set(initialState)
  }))
)

// Selectors
export const selectActiveSession = (state: BrowserStore): BrowserSession | null => {
  return state.activeSessionId ? state.sessions.get(state.activeSessionId) || null : null
}

export const selectAllSessions = (state: BrowserStore): BrowserSession[] => {
  return Array.from(state.sessions.values())
}
