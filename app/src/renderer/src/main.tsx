/* eslint-disable react-refresh/only-export-components */
import './assets/main.css'

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { Toaster } from 'sonner'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { createHashHistory } from '@tanstack/react-router'

import { ApolloClientProvider } from './graphql/provider'
import { ThemeProvider } from './lib/theme'
import { TTSProvider } from './lib/ttsProvider'
import { routeTree } from '@renderer/routeTree.gen'
import InvitationGate from './components/onboarding/InvitationGate'

export const router = createRouter({
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
  return (
    <ThemeProvider defaultTheme={savedTheme}>
      <TTSProvider>
        <ApolloClientProvider>
          <div className="flex flex-col h-full w-full">
            <Toaster position="bottom-right" />
            <InvitationGate>
              <RouterProvider router={router} />
            </InvitationGate>
          </div>
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
