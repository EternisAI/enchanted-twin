import { GetWhitelistStatusDocument } from '@renderer/graphql/generated/graphql'
import { client } from '@renderer/graphql/lib'
import { auth } from '@renderer/lib/firebase'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import React, { useEffect, useState } from 'react'
import { router } from '@renderer/main'
import { useTheme } from '@renderer/lib/theme'
import { Loader } from 'lucide-react'

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
  const { theme } = useTheme()
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
        <div
          className="flex-1 flex items-center justify-center"
          style={{
            background:
              theme === 'light'
                ? 'linear-gradient(180deg, #6068E9 0%, #A5AAF9 100%)'
                : 'linear-gradient(180deg, #18181B 0%, #000 100%)'
          }}
        >
          <div className="flex flex-col gap-12 text-primary-foreground p-10 border border-white/50 rounded-lg bg-white/5 min-w-2xl">
            <div className="flex flex-col gap-1 text-center">
              <h1 className="text-lg font-normal text-white">Starting Enchanted</h1>
            </div>

            <div className="flex flex-col gap-4">
              <div className="flex flex-col  items-center justify-center gap-3">
                <div className="flex flex-col gap-2 items-center max-w-sm">
                  <Loader className="animate-spin w-8 h-8 text-white" />
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    )
  }

  return <>{children}</>
}
