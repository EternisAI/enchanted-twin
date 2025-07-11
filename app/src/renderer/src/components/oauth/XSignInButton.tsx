import { useState } from 'react'
import { useAuth } from '@renderer/contexts/AuthContext'
import { Button } from '@renderer/components/ui/button'
import XformerlyTwitter from '@renderer/assets/icons/x'

export default function XSignInButton() {
  const { signInWithX, loading, authError } = useAuth()
  const [isSigningIn, setIsSigningIn] = useState(false)

  const handleSignIn = async () => {
    setIsSigningIn(true)
    try {
      await signInWithX()
    } catch (error) {
      console.error('X sign-in failed:', error)
    } finally {
      setIsSigningIn(false)
    }
  }

  return (
    <div className="flex flex-col items-center space-y-4">
      <Button
        onClick={handleSignIn}
        disabled={loading || isSigningIn}
        className="flex items-center space-x-3 px-6 py-3 bg-white border border-gray-300 text-gray-700 hover:bg-gray-50 disabled:opacity-50 w-[200px]"
      >
        <XformerlyTwitter />
        <span>{isSigningIn || loading ? 'Signing in...' : 'Continue with X'}</span>
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
