import { createContext, useContext, useEffect, useState, ReactNode } from 'react'
import {
  User,
  onAuthStateChanged,
  signOut as firebaseSignOut,
  GoogleAuthProvider,
  TwitterAuthProvider,
  signInWithCredential
} from 'firebase/auth'
import { auth, firebaseConfig } from '@renderer/lib/firebase'
import { useMutation, useQuery } from '@apollo/client'
import {
  StoreTokenDocument,
  GetWhitelistStatusDocument,
  ActivateInviteCodeDocument
} from '@renderer/graphql/generated/graphql'
import { toast } from 'sonner'

interface AuthContextType {
  user: User | null
  loading: boolean
  waitingForLogin: boolean
  signOut: () => Promise<void>
  signInWithGoogle: () => Promise<void>
  signInWithX: () => Promise<void>
  authError: string | null
  hasUpdatedToken: boolean
  whitelist: {
    isWhitelisted: boolean
    status: boolean | null
    loading: boolean
    called: boolean
    error: string | null
    activateInviteCode: (inviteCode: string) => Promise<void>
  }
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

  // @TODO: Move whitelist logic to InvitationGate
  const {
    data: whitelistData,
    loading: whitelistLoading,
    error: whitelistError,
    called: whitelistCalled,
    refetch: refetchWhitelist
  } = useQuery(GetWhitelistStatusDocument, {
    fetchPolicy: 'network-only',
    skip: !user || !hasUpdatedToken
  })

  console.log('whitelistCalled', { whitelistCalled, whitelistLoading })

  const [activateInviteCodeMutation] = useMutation(ActivateInviteCodeDocument, {
    onCompleted: async () => {
      toast.success('Invite code activated successfully!')
      await refetchWhitelist()
    },
    onError: (error) => {
      console.error('[Auth] Failed to activate invite code:', error)
      toast.error(`Failed to activate invite code: ${error.message}`)
      throw error
    }
  })

  const activateInviteCode = async (inviteCode: string) => {
    if (!inviteCode.trim()) {
      throw new Error('Please enter an invite code')
    }

    await activateInviteCodeMutation({
      variables: { inviteCode: inviteCode.trim() }
    })
  }

  const whitelistStatus = whitelistData?.whitelistStatus || null
  const isWhitelisted = Boolean(whitelistStatus || whitelistError)

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
          provider?: string
        }
      ]

      setAuthError(null)
      setLoading(false)
      setWaitingForLogin(false)

      try {
        let credential
        if (userData.provider === 'twitter') {
          credential = TwitterAuthProvider.credential(userData.accessToken, userData.idToken)
        } else {
          credential = GoogleAuthProvider.credential(userData.idToken, userData.accessToken)
        }
        await signInWithCredential(auth, credential)
        localStorage.setItem('enchanted_user_data', JSON.stringify(userData))
      } catch (error) {
        console.error('[Auth] ❌ Failed to sign in with credential:', error)
        setAuthError(error instanceof Error ? error.message : 'Authentication failed')
      }
    }

    const handleFirebaseAuthError = async (...args: unknown[]) => {
      const [, errorData] = args as [unknown, { error: string }]
      console.error('[Auth] ❌ Received Firebase auth error from main process:', errorData)

      // Clear any cached authentication state
      try {
        await firebaseSignOut(auth)
        localStorage.removeItem('enchanted_user_data')
      } catch (error) {
        console.error('[Auth] Failed to clear auth state:', error)
      }

      setAuthError(errorData.error)
      setLoading(false)
      setWaitingForLogin(false)
      setUser(null)
      window.electron.ipcRenderer.invoke('cleanup-oauth-server')
    }

    window.electron.ipcRenderer.on('firebase-auth-success', handleFirebaseAuthSuccess)
    window.electron.ipcRenderer.on('firebase-auth-error', handleFirebaseAuthError)

    return () => {
      console.log('[Auth] Cleaning up IPC listeners')
    }
  }, [])

  const signInWithGoogle = async () => {
    console.log('[Auth] Starting Google sign-in flow')
    setAuthError(null)
    setWaitingForLogin(true)

    try {
      console.log('[Auth] Invoking start-firebase-oauth')
      const result = (await window.electron.ipcRenderer.invoke('start-firebase-oauth', {
        ...firebaseConfig,
        provider: 'google'
      })) as unknown as {
        success: boolean
        loginUrl?: string
        error?: string
      }

      if (!result.success) {
        throw new Error(result.error || 'Failed to start OAuth server')
      }

      console.log('[Auth] Firebase OAuth server started successfully, waiting for auth callback...')
      // The authentication flow will continue through IPC events - wait for the IPC callback
    } catch (error) {
      console.error('[Auth] Failed to start Google sign-in:', error)
      setAuthError(error instanceof Error ? error.message : 'Failed to start authentication')
      setWaitingForLogin(false)
    }
  }

  const signInWithX = async () => {
    console.log('[Auth] Starting X sign-in flow')
    setAuthError(null)
    setWaitingForLogin(true)

    try {
      console.log('[Auth] Invoking start-firebase-oauth for X')
      const result = (await window.electron.ipcRenderer.invoke('start-firebase-oauth', {
        ...firebaseConfig,
        provider: 'twitter'
      })) as unknown as {
        success: boolean
        loginUrl?: string
        error?: string
      }

      if (!result.success) {
        throw new Error(result.error || 'Failed to start OAuth server')
      }

      console.log('[Auth] Firebase OAuth server started successfully, waiting for auth callback...')
      // The authentication flow will continue through IPC events - wait for the IPC callback
    } catch (error) {
      console.error('[Auth] Failed to start X sign-in:', error)
      setAuthError(error instanceof Error ? error.message : 'Failed to start authentication')
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
        authError,
        whitelist: {
          status: whitelistStatus,
          loading: whitelistLoading,
          error: whitelistError?.message || null,
          called: whitelistCalled,
          isWhitelisted,
          activateInviteCode
        },
        signInWithX
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
