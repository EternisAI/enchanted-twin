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
          <AuthProvider>
            <GoLogsProvider>
              <div className="flex flex-col h-full w-full">
                <UpdateNotification />

                <Toaster position="bottom-right" />
                <InvitationGate>
                  <RouterProvider router={router} />
                </InvitationGate>
              </div>
            </GoLogsProvider>
          </AuthProvider>
        </ApolloClientProvider>
      </TTSProvider>
    </ThemeProvider>
  )
}

export default createRoot(document.getElementById('root')!).render(<App />)
