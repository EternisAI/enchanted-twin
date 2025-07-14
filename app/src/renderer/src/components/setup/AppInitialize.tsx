import React, { useEffect, useState } from 'react'

import { GetWhitelistStatusDocument } from '@renderer/graphql/generated/graphql'
import { client } from '@renderer/graphql/lib'
import { auth } from '@renderer/lib/firebase'
import { useOnboardingStore } from '@renderer/lib/stores/onboarding'
import { router } from '@renderer/main'
import Loading from '../Loading'

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
        const dependenciesDownloaded = await window.api.models.hasModelsDownloaded()

        const allDependenciesDownloaded = Object.values(dependenciesDownloaded).every(
          (downloaded) => downloaded
        )

        console.log('allDependenciesDownloaded', allDependenciesDownloaded)

        if (!allDependenciesDownloaded) {
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
        console.error('[AppInitialize] Error checking app initialization:', error)
        setLoading(false)
      } finally {
        setLoading(false)
      }
    }

    checkAppInitialization()
  }, [])

  if (loading) {
    return <Loading />
  }

  return <>{children}</>
}
