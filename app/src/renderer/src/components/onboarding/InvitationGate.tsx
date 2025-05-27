import React, { useState, useMemo } from 'react'
import { useQuery, useMutation } from '@apollo/client'
import {
  GetMcpServersDocument,
  GetWhitelistStatusDocument,
  ActivateInviteCodeDocument,
  McpServerType
} from '@renderer/graphql/generated/graphql'
import { OnboardingLayout } from './OnboardingLayout'
import { Card } from '../ui/card'
import { Input } from '../ui/input'
import { Button } from '../ui/button'
import { AlertCircle, Loader2 } from 'lucide-react'
import { toast } from 'sonner'
import MCPServerItem from '../oauth/MCPServerItem'

export default function InvitationGate({ children }: { children: React.ReactNode }) {
  const [inviteCode, setInviteCode] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isActivated, setIsActivated] = useState(false)

  const {
    data: mcpData,
    loading: mcpLoading,
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

  console.log('whitelistData', whitelistData, hasGoogleConnected)

  const isWhitelisted = whitelistData?.whitelistStatus

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
  }

  // If user is activated or whitelisted, show the app
  if (isActivated || isWhitelisted) {
    return <>{children}</>
  }

  if (mcpLoading || whitelistLoading) {
    return (
      <div className="flex justify-center py-8 w-full">
        <OnboardingLayout
          title="Initializing Enchanted"
          subtitle="Please wait while we check your status"
        >
          <div className="flex justify-center py-8 w-full">
            <Loader2 className="h-8 w-8 animate-spin" />
          </div>
        </OnboardingLayout>
      </div>
    )
  }

  if (!hasGoogleConnected) {
    return (
      <OnboardingLayout
        title="Connect your Google account"
        subtitle="First, you need to connect your Google account to continue"
      >
        <div className="flex flex-col gap-6 items-center">
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
          <Card className="p-4 bg-blue-50 dark:bg-blue-950/20 border-blue-200 dark:border-blue-800">
            <div className="flex items-center gap-3">
              <AlertCircle className="h-5 w-5 text-blue-600 dark:text-blue-400" />
              <p className="text-sm text-blue-800 dark:text-blue-200">
                Please connect your Google account above to proceed with the setup.
              </p>
            </div>
          </Card>
        </div>
      </OnboardingLayout>
    )
  }

  // Show invite code input if not whitelisted
  return (
    <OnboardingLayout
      title="Enter your invite code"
      subtitle="Enter your invite code to activate your account"
    >
      <Card className="p-6">
        <form onSubmit={handleInviteCodeSubmit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <label htmlFor="inviteCode" className="block text-sm font-medium mb-2">
              Invite Code
            </label>
            <Input
              id="inviteCode"
              type="text"
              value={inviteCode}
              onChange={(e) => setInviteCode(e.target.value)}
              placeholder="Enter your invite code"
              className="w-full"
              disabled={isSubmitting}
            />
          </div>
          <Button type="submit" className="w-full" disabled={isSubmitting || !inviteCode.trim()}>
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
      </Card>

      <Card className="p-4 bg-amber-50 dark:bg-amber-950/20 border-amber-200 dark:border-amber-800">
        <div className="flex items-center gap-3">
          <AlertCircle className="h-5 w-5 text-amber-600 dark:text-amber-400" />
          <p className="text-sm text-amber-800 dark:text-amber-200">
            Don&apos;t have an invite code? Contact support to request access.
          </p>
        </div>
      </Card>
    </OnboardingLayout>
  )
}
