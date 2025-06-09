import React, { useState, useMemo, useEffect } from 'react'
import { useQuery, useMutation } from '@apollo/client'
import {
  GetMcpServersDocument,
  GetWhitelistStatusDocument,
  ActivateInviteCodeDocument,
  McpServerType
} from '@renderer/graphql/generated/graphql'
import { OnboardingLayout } from './OnboardingLayout'
import { Input } from '../ui/input'
import { Button } from '../ui/button'
import { Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import MCPServerItem from '../oauth/MCPServerItem'
import { router } from '../../main'
import { OnboardingVoiceAnimation } from './voice/Animations'
import { useTheme } from '@renderer/lib/theme'
import { useSettingsStore } from '@renderer/lib/stores/settings'

export default function InvitationGate({ children }: { children: React.ReactNode }) {
  const [inviteCode, setInviteCode] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isActivated, setIsActivated] = useState(false)
  const { setActiveTab } = useSettingsStore()

  const {
    data: mcpData,
    loading: mcpLoading,
    error: mcpError,
    refetch: refetchMcpServers
  } = useQuery(GetMcpServersDocument, {
    fetchPolicy: 'network-only'
  })

  const hasGoogleConnected = useMemo(() => {
    if (!mcpData?.getMCPServers) return false
    return mcpData.getMCPServers.some(
      (server) => server.type === McpServerType.Google && server.connected
    )
  }, [mcpData])

  const {
    data: whitelistData,
    loading: whitelistLoading,
    error: whitelistError,
    refetch: refetchWhitelist
  } = useQuery(GetWhitelistStatusDocument, {
    fetchPolicy: 'network-only',
    skip: !hasGoogleConnected
  })

  const [activateInviteCode] = useMutation(ActivateInviteCodeDocument, {
    onCompleted: async () => {
      toast.success('Invite code activated successfully!')
      await refetchWhitelist()
      setIsActivated(true)
    },
    onError: (error) => {
      console.error(error)
      toast.error(`Failed to activate invite code: ${error.message}`)
      setIsSubmitting(false)
    }
  })

  console.log('whitelistData', whitelistData, whitelistError)

  const errorFetching = useMemo(() => {
    return whitelistError || mcpError
  }, [whitelistError, mcpError])

  useEffect(() => {
    const handleError = async () => {
      if (errorFetching) {
        console.error('Whitelist query failed:', errorFetching?.message)
        setActiveTab('advanced')
        await new Promise((resolve) => setTimeout(resolve, 3000))
        router.navigate({ to: '/settings' })
      }
    }
    handleError()
  }, [errorFetching, setActiveTab])

  const isWhitelisted = whitelistData?.whitelistStatus || whitelistError

  const handleInviteCodeSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!inviteCode.trim()) {
      toast.error('Please enter an invite code')
      return
    }

    setIsSubmitting(true)
    try {
      await activateInviteCode({
        variables: { inviteCode: inviteCode.trim() }
      })
      setInviteCode('')
    } catch {
      // Error is handled in onError callback
    }
    setIsSubmitting(false)
    router.navigate({ to: '/onboarding' })
  }

  if (isActivated || isWhitelisted || errorFetching) {
    return <>{children}</>
  }

  if (mcpLoading || whitelistLoading) {
    return (
      <div className="flex justify-center py-8 w-full">
        <OnboardingLayout title="Initializing Enchanted" subtitle="Checking whitelist status...">
          <div className="flex justify-center py-0 w-full text-primary">
            <Loader2 className="h-8 w-8 animate-spin" />
          </div>
        </OnboardingLayout>
      </div>
    )
  }

  if (!hasGoogleConnected) {
    return (
      <InvitationWrapper showTitlebar showAnimation showPrivacyText>
        <OnboardingLayout
          title="Beta Access"
          subtitle="Login with Google for Beta access."
          className="text-white"
        >
          <div className="flex flex-col gap-6 items-center ">
            <MCPServerItem
              server={{
                id: 'GOOGLE',
                type: McpServerType.Google,
                connected: false,
                command: '',
                enabled: false,
                name: 'Google'
              }}
              onConnect={() => {
                refetchMcpServers()
              }}
              onRemove={() => {}}
            />
          </div>
        </OnboardingLayout>
      </InvitationWrapper>
    )
  }

  return (
    <InvitationWrapper showTitlebar showAnimation showPrivacyText>
      <OnboardingLayout
        title="Invitation Code"
        subtitle="Enter your invite code to access Enchanted"
        className="!text-white"
      >
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
          <Button
            type="submit"
            className="w-fit px-8 bg-white text-black"
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
        </form>
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
  const { theme } = useTheme()

  return (
    <div
      className="flex flex-col gap-6 justify-between items-center relative overflow-hidden"
      style={{
        background:
          theme === 'light'
            ? 'linear-gradient(180deg, #6068E9 0%, #A5AAF9 100%)'
            : 'linear-gradient(180deg, #18181B 0%, #000 100%)'
      }}
    >
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
