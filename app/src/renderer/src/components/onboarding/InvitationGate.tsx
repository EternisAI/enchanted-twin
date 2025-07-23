import { useState } from 'react'
import { toast } from 'sonner'
import { motion } from 'framer-motion'

import { OnboardingLayout } from './OnboardingLayout'
import { Input } from '../ui/input'
import { Button } from '../ui/button'
import { Loader2, RefreshCcw } from 'lucide-react'
import { router } from '../../main'
import { OnboardingVoiceAnimation } from './new/Animations'
import { useAuth } from '@renderer/contexts/AuthContext'
import GoogleSignInButton from '../oauth/GoogleSignInButton'
import FreysaLoading from '@renderer/assets/icons/freysaLoading.png'
import XSignInButton from '../oauth/XSignInButton'
import Loading from '../Loading'
import { PrivacyButton } from '../chat/privacy/PrivacyButton'

export default function InvitationGate({ children }: { children: React.ReactNode }) {
  const [inviteCode, setInviteCode] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  const { user, authError, loading: authLoading, waitingForLogin, whitelist, signOut } = useAuth()

  const {
    loading: whitelistLoading,
    status: isWhitelisted,
    error: whitelistError,
    called: whitelistCalled,
    activateInviteCode
  } = whitelist

  const errorFetching = whitelistError || authError

  const handleInviteCodeSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!inviteCode.trim()) {
      toast.error('Please enter an invite code')
      return
    }

    setIsSubmitting(true)
    try {
      await activateInviteCode(inviteCode.trim())
      setInviteCode('')
      router.navigate({ to: '/onboarding' })
    } catch {
      // Error is handled in the auth context
    } finally {
      setIsSubmitting(false)
    }
  }

  if (isWhitelisted || errorFetching) {
    return <>{children}</>
  }

  if (waitingForLogin) {
    console.log('waitingForLogin', waitingForLogin)
    return (
      <div className="flex justify-center py-8 w-full">
        <OnboardingLayout title="Initializing Enchanted" subtitle="Checking whitelist status...">
          <div className="flex flex-col items-center justify-center gap-6 py-0 w-full text-primary">
            <h1 className="text-lg font-bold">
              A browser tab will open to login using your Google account
            </h1>
            <Loader2 className="h-8 w-8 animate-spin" />

            <div className="flex flex-col gap-3 mt-4">
              <p className="text-sm text-muted-foreground text-center">
                If the browser tab didn&apos;t open or you closed it, you can retry:
              </p>
              <div className="flex gap-3 justify-center">
                <Button
                  onClick={() => window.location.reload()}
                  variant="outline"
                  size="sm"
                  className="flex items-center gap-2"
                >
                  <RefreshCcw className="h-4 w-4" />
                  Retry Login
                </Button>
              </div>
            </div>
          </div>
        </OnboardingLayout>
      </div>
    )
  }

  if (whitelistLoading || authLoading || (user && !whitelistCalled)) {
    return <Loading />
  }

  if (!user) {
    return (
      <InvitationWrapper showTitlebar>
        <OnboardingLayout title="" subtitle="" className="text-white">
          <div className="flex flex-col gap-6 text-primary-foreground p-10 border border-white/48 rounded-lg bg-white/5 min-w-sm">
            <div className="flex flex-col gap-1 text-center items-center">
              <img src={FreysaLoading} alt="Enchanted" className="w-16 h-16" />
              <h1 className="text-lg font-normal text-white">Welcome to Enchanted</h1>
            </div>

            <div className="flex flex-col gap-4">
              <GoogleSignInButton />
              <XSignInButton />
            </div>
          </div>
          <PrivacyButton className="text-white hover:bg-transparent hover:text-white/90" />
        </OnboardingLayout>
      </InvitationWrapper>
    )
  }

  return (
    <InvitationWrapper showTitlebar>
      <OnboardingLayout title="" subtitle="" className="!text-white">
        <div className="flex flex-col gap-6 text-primary-foreground p-10 border border-white/48 rounded-lg bg-white/5 min-w-sm">
          <div className="flex flex-col gap-1 text-center items-center">
            <img src={FreysaLoading} alt="Enchanted" className="w-16 h-16" />
            <h1 className="text-lg font-normal text-white">Invitation Code</h1>
            <p className="text-white/80 text-sm">Enter your invite code to access Enchanted</p>
          </div>

          <form onSubmit={handleInviteCodeSubmit} className="flex flex-col items-center gap-4">
            <Input
              id="inviteCode"
              type="text"
              value={inviteCode}
              onChange={(e) => setInviteCode(e.target.value)}
              placeholder="Enter your invite code"
              className="max-w-md h-12 mx-auto !bg-white/20 border-white/20 text-white border-none placeholder:text-white/70"
              disabled={isSubmitting}
            />
            {inviteCode.trim() && (
              <motion.div
                initial={{ opacity: 0, y: 10 }}
                animate={{ opacity: 1, y: 0 }}
                exit={{ opacity: 0, y: -10 }}
                transition={{ duration: 0.2 }}
              >
                <Button
                  type="submit"
                  className="w-fit px-8 bg-white text-black hover:bg-white/60"
                  disabled={isSubmitting || !inviteCode.trim()}
                >
                  {isSubmitting ? (
                    <>
                      <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                      Activating...
                    </>
                  ) : (
                    'Activate Account'
                  )}
                </Button>
              </motion.div>
            )}

            <Button
              type="button"
              variant="ghost"
              className="w-fit px-8 text-white"
              onClick={signOut}
            >
              Or, sign in with different account
            </Button>
          </form>
        </div>
      </OnboardingLayout>
    </InvitationWrapper>
  )
}

interface InvitationWrapperProps {
  children: React.ReactNode
  showTitlebar?: boolean
  showAnimation?: boolean
  showPrivacyText?: boolean
}

function InvitationWrapper({
  children,
  showTitlebar = false,
  showAnimation = false,
  showPrivacyText = false
}: InvitationWrapperProps) {
  return (
    <div className="flex flex-col gap-6 justify-between items-center relative overflow-hidden onboarding-background">
      {showTitlebar && (
        <div className="titlebar text-center fixed top-0 left-0 right-0 text-muted-foreground text-xs h-8 z-20 flex items-center justify-center backdrop-blur-sm" />
      )}

      {children}

      {showAnimation && (
        <div className="absolute top-[-20px] left-0">
          <OnboardingVoiceAnimation
            layerCount={9}
            getFreqData={() => new Uint8Array(256).fill(13)}
          />
        </div>
      )}

      {showPrivacyText && (
        <p className="absolute bottom-8 text-sm text-center text-secondary-foreground/50 max-w-md">
          Everything stays local on your device, <br /> and only you can access your Google
          account—never us.
        </p>
      )}
    </div>
  )
}
