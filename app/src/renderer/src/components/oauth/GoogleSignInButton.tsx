import { useState } from 'react'
import { useAuth } from '@renderer/contexts/AuthContext'
import { Button } from '@renderer/components/ui/button'

export default function GoogleSignInButton() {
  const { signInWithGoogle, loading, authError } = useAuth()
  const [isSigningIn, setIsSigningIn] = useState(false)

  const handleSignIn = async () => {
    setIsSigningIn(true)
    try {
      await signInWithGoogle()
    } catch (error) {
      console.error('Sign-in failed:', error)
    } finally {
      setIsSigningIn(false)
    }
  }

  return (
    <div className="flex flex-col items-center space-y-4">
      <Button
        onClick={handleSignIn}
        disabled={loading || isSigningIn}
        className="flex items-center px-6 py-3 bg-white/14 text-white hover:bg-white/20 disabled:opacity-50 w-[250px]"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="24"
          height="24"
          viewBox="0 0 24 24"
          fill="none"
        >
          <path
            d="M21.8047 10.0417H21V10H12V14H17.651C16.8281 16.3281 14.612 18 12 18C8.6875 18 6 15.3125 6 12C6 8.6875 8.6875 6 12 6C13.5286 6 14.9219 6.57813 15.9818 7.51823L18.8099 4.6901C17.0234 3.02604 14.6328 2 12 2C6.47656 2 2 6.47656 2 12C2 17.5234 6.47656 22 12 22C17.5234 22 22 17.5234 22 12C22 11.3307 21.9323 10.6745 21.8047 10.0417Z"
            fill="white"
          />
          <path
            d="M3.15381 7.34635L6.43766 9.75521C7.32829 7.55469 9.48193 6 12.0002 6C13.5288 6 14.922 6.57813 15.9819 7.51823L18.8101 4.6901C17.0236 3.02604 14.633 2 12.0002 2C8.15902 2 4.82829 4.16927 3.15381 7.34635Z"
            fill="white"
          />
          <path
            d="M11.9998 21.9993C14.5832 21.9993 16.9295 21.0098 18.7056 19.403L15.6092 16.7832C14.6066 17.5436 13.3566 17.9993 11.9998 17.9993C9.39827 17.9993 7.18994 16.3405 6.35921 14.0254L3.09619 16.5384C4.75244 19.778 8.11442 21.9993 11.9998 21.9993Z"
            fill="white"
          />
          <path
            d="M21.8047 10.0417H21V10H12V14H17.651C17.2552 15.1198 16.5365 16.0833 15.6068 16.7865C15.6094 16.7839 15.6094 16.7839 15.6094 16.7839L18.7057 19.4036C18.4844 19.6016 22 17 22 12C22 11.3307 21.9323 10.6745 21.8047 10.0417Z"
            fill="white"
          />
        </svg>
        <span>{isSigningIn || loading ? 'Signing in...' : 'Continue with Google'}</span>
      </Button>

      {authError && (
        <div className="text-red-600 text-sm bg-red-50 px-4 py-2 rounded-md border border-red-200">
          <p className="font-semibold">Authentication Error</p>
          <p>{authError}</p>
        </div>
      )}
    </div>
  )
}
