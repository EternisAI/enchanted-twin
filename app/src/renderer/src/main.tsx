/* eslint-disable react-refresh/only-export-components */
import './assets/main.css'

import { StrictMode, useState, useEffect } from 'react'
import { createRoot } from 'react-dom/client'
import { Toaster } from 'sonner'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { createHashHistory } from '@tanstack/react-router'

import { ApolloClientProvider } from './graphql/provider'
import { ThemeProvider } from './lib/theme'
import { TTSProvider } from './lib/ttsProvider'
import { routeTree } from '@renderer/routeTree.gen'
import LaunchScreen from './pages/Launch'

const router = createRouter({
  routeTree,
  defaultViewTransition: true,
  history: createHashHistory()
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

const savedTheme = (() => {
  try {
    return (localStorage.getItem('theme') as 'dark' | 'light' | 'system') || 'system'
  } catch {
    return 'system'
  }
})()

function App() {
  const [isLaunchComplete, setIsLaunchComplete] = useState(false)

  useEffect(() => {
    window.api.onLaunch('launch-complete', () => {
      setIsLaunchComplete(true)
    })
  }, [])

  return (
    <ThemeProvider defaultTheme={savedTheme}>
      <TTSProvider>
        <ApolloClientProvider>
          {isLaunchComplete ? (
            <>
              <RouterProvider router={router} />
              <Toaster position="bottom-right" />
            </>
          ) : (
            <LaunchScreen />
          )}
        </ApolloClientProvider>
      </TTSProvider>
    </ThemeProvider>
  )
}

export default createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>
)
