/* eslint-disable react-refresh/only-export-components */
import './assets/main.css'

import { createRoot } from 'react-dom/client'
import { Toaster } from 'sonner'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { createHashHistory } from '@tanstack/react-router'

import { ApolloClientProvider } from './graphql/provider'
import { ThemeProvider } from './lib/theme'
import { TTSProvider } from './lib/ttsProvider'
import { GoLogsProvider } from './contexts/GoLogsContext'
import { AuthProvider } from './contexts/AuthContext'
import { routeTree } from '@renderer/routeTree.gen'
import InvitationGate from './components/onboarding/InvitationGate'
import UpdateNotification from './components/UpdateNotification'
import AppSetupGate from './components/setup/AppSetupGate'
import AppInitialize from './components/setup/AppInitialize'

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
      <Toaster position="bottom-right" />
      <TTSProvider>
        <ApolloClientProvider>
          <GoLogsProvider>
            <AuthProvider>
              <AppInitialize>
                <div className="flex flex-col h-full w-full">
                  <AppSetupGate>
                    <InvitationGate>
                      <UpdateNotification />
                      <RouterProvider router={router} />
                    </InvitationGate>
                  </AppSetupGate>
                </div>
              </AppInitialize>
            </AuthProvider>
          </GoLogsProvider>
        </ApolloClientProvider>
      </TTSProvider>
    </ThemeProvider>
  )
}

export default createRoot(document.getElementById('root')!).render(<App />)
