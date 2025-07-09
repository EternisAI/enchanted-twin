import { GetWhitelistStatusDocument } from '@renderer/graphql/generated/graphql'
import { client } from '@renderer/graphql/lib'
import { auth } from '@renderer/lib/firebase'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import React, { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { router } from '@renderer/main'

type AppInitializeState =
  | 'DEPENDECIES_REQUIRED'
  | 'AUTH_REQUIRED'
  | 'WHITELIST_REQUIRED'
  | 'ONBOARDING_REQUIRED'
  | 'READY'

// server can start here
export default function AppInitialize({ children }: { children: React.ReactNode }) {
  const [initializeState, setInitializeState] = useState<AppInitializeState>('DEPENDECIES_REQUIRED')
  const [loading, setLoading] = useState(true)
  const { isCompleted: onboardingCompleted } = useOnboardingStore()
  console.log('initializeState', initializeState, loading)

  useEffect(() => {
    const checkAppInitialization = async () => {
      try {
        // first check if all dependencies are downloaded
        const { embeddings, anonymizer, onnx } = await window.api.models.hasModelsDownloaded()

        if (!embeddings || !anonymizer || !onnx) {
          setInitializeState('DEPENDECIES_REQUIRED')
          return
        }

        // 2. then check if the user is authenticated
        await auth.authStateReady()
        const user = await auth.currentUser

        if (!user) {
          setInitializeState('AUTH_REQUIRED')
          return
        }

        // 3. then check if the user is whitelisted
        const { data: whitelistData } = await client.query({
          query: GetWhitelistStatusDocument,
          fetchPolicy: 'network-only'
        })

        const whitelist = whitelistData?.whitelistStatus || false
        if (!whitelist) {
          setInitializeState('WHITELIST_REQUIRED')
          return
        }

        // 4. then check if the user has completed onboarding
        if (!onboardingCompleted) {
          setInitializeState('ONBOARDING_REQUIRED')
          router.navigate({ to: '/onboarding' })
          return
        }

        setInitializeState('READY')
      } catch (error) {
        console.error(error)
        setLoading(false)
      } finally {
        setLoading(false)
      }
    }

    checkAppInitialization()
  }, [])

  if (loading) {
    return (
      <div className="flex flex-col h-screen w-screen">
        <div className="titlebar text-center fixed top-0 left-0 right-0 text-muted-foreground text-xs h-8 z-20 flex items-center justify-center backdrop-blur-sm" />
        <div className="flex-1 flex items-center justify-center">
          <Loader2 className="h-8 w-8 animate-spin" />
        </div>
      </div>
    )
  }

  return <>{children}</>
}
