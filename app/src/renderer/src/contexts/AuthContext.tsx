import { createContext, useContext, useEffect, useState, ReactNode } from 'react'
import {
  User,
  onAuthStateChanged,
  signOut as firebaseSignOut,
  GoogleAuthProvider,
  signInWithCredential
} from 'firebase/auth'
import { auth, firebaseConfig } from '@renderer/lib/firebase'
import { useMutation } from '@apollo/client'
import { StoreTokenDocument } from '@renderer/graphql/generated/graphql'

interface AuthContextType {
  user: User | null
  loading: boolean
  waitingForLogin: boolean
  signOut: () => Promise<void>
  signInWithGoogle: () => Promise<void>
  authError: string | null
  hasUpdatedToken: boolean
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [authError, setAuthError] = useState<string | null>(null)
  const [waitingForLogin, setWaitingForLogin] = useState(false)
  const [hasUpdatedToken, setHasUpdatedToken] = useState(false)

  const [storeToken] = useMutation(StoreTokenDocument, {
    onError: async (error) => {
      console.error('[Auth] Failed to store token:', error)
    }
  })

  useEffect(() => {
    if (!user) return
    const storeTokenPeriodically = async () => {
      try {
        const jwt = await user.getIdToken()
        const refreshToken = await user.refreshToken
        await storeToken({
          variables: {
            input: {
              token: jwt,
              refreshToken: refreshToken
            }
          }
        })
        setHasUpdatedToken(true)
      } catch (error) {
        console.error('[Auth] Failed to store token:', error)
      }
    }

    storeTokenPeriodically()

    const interval = setInterval(storeTokenPeriodically, 5 * 60 * 1000)

    return () => {
      clearInterval(interval)
    }
  }, [user, storeToken])

  useEffect(() => {
    const unsubscribe = onAuthStateChanged(auth, (user) => {
      setUser(user)
      setLoading(false)
    })

    return unsubscribe
  }, [])

  // Listen for Firebase auth success from main process
  useEffect(() => {
    const handleFirebaseAuthSuccess = async (...args: unknown[]) => {
      const [, userData] = args as [
        unknown,
        {
          uid: string
          email: string
          displayName: string
          photoURL: string
          accessToken: string
          idToken: string
          refreshToken?: string
        }
      ]

      setAuthError(null)
      setLoading(false)
      setWaitingForLogin(false)

      try {
        const credential = GoogleAuthProvider.credential(userData.idToken, userData.accessToken)
        await signInWithCredential(auth, credential)
        localStorage.setItem('enchanted_user_data', JSON.stringify(userData))
      } catch (error) {
        console.error('[Auth] ❌ Failed to sign in with Google credential:', error)
        setAuthError(error instanceof Error ? error.message : 'Authentication failed')
      }
    }

    const handleFirebaseAuthError = (...args: unknown[]) => {
      const [, errorData] = args as [unknown, { error: string }]
      console.error('[Auth] ❌ Received Firebase auth error from main process:', errorData)
      setAuthError(errorData.error)
      setLoading(false)
      setWaitingForLogin(false)
      window.electron.ipcRenderer.invoke('cleanup-oauth-server')
    }

    window.electron.ipcRenderer.on('firebase-auth-success', handleFirebaseAuthSuccess)
    window.electron.ipcRenderer.on('firebase-auth-error', handleFirebaseAuthError)

    return () => {
      console.log('[Auth] Cleaning up IPC listeners')
    }
  }, [])

  const signInWithGoogle = async () => {
    console.log('[Auth]  Starting Google sign-in flow')
    setAuthError(null)
    setWaitingForLogin(true)

    try {
      console.log('[Auth]  Invoking start-firebase-oauth')
      const result = (await window.electron.ipcRenderer.invoke(
        'start-firebase-oauth',
        firebaseConfig
      )) as unknown as {
        success: boolean
        loginUrl?: string
        error?: string
      }

      if (!result.success) {
        throw new Error(result.error || 'Failed to start OAuth server')
      }

      console.log(
        '[Auth]  Firebase OAuth server started successfully, waiting for auth callback...'
      )
      // The authentication flow will continue through IPC events -  wait for the IPC callback
    } catch (error) {
      console.error('[Auth]  Failed to start Google sign-in:', error)
      setAuthError(error instanceof Error ? error.message : 'Failed to start authentication')
      // setLoading(false)
      setWaitingForLogin(false)
    }
  }

  const signOut = async () => {
    try {
      await firebaseSignOut(auth)
      setUser(null)
      console.log('[Auth] Signed out from Firebase')
      localStorage.removeItem('enchanted_user_data')
      await window.electron.ipcRenderer.invoke('cleanup-oauth-server')
    } catch (error) {
      console.error('Error signing out:', error)
      throw error
    }
  }

  return (
    <AuthContext.Provider
      value={{
        user,
        loading,
        waitingForLogin,
        hasUpdatedToken,
        signOut,
        signInWithGoogle,
        authError
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (context === undefined) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return context
}
