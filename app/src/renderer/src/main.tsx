/* eslint-disable react-refresh/only-export-components */
import './assets/main.css'

import { createRoot } from 'react-dom/client'
import { Toaster } from 'sonner'
import { createRouter, RouterProvider } from '@tanstack/react-router'
import { createHashHistory } from '@tanstack/react-router'

import { ApolloClientProvider } from './graphql/provider'
import { SyncedThemeProvider } from './components/SyncedThemeProvider'
import { TTSProvider } from './lib/ttsProvider'
import { GoServerProvider } from './contexts/GoServerContext'
import { AuthProvider } from './contexts/AuthContext'
import { routeTree } from '@renderer/routeTree.gen'
import AuthGate from './components/onboarding/AuthGate'
import UpdateNotification from './components/UpdateNotification'
import DependenciesGate from './components/setup/DependenciesGate'
import { ModalProvider } from './components/providers/ModalProvider'

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

function App() {
  return (
    <SyncedThemeProvider>
      <Toaster position="bottom-right" />
      <TTSProvider>
        <ApolloClientProvider>
          <GoServerProvider>
            <ModalProvider>
              <UpdateNotification />
              <div className="flex flex-col h-screen w-screen bg-background">
                <DependenciesGate>
                  <AuthProvider>
                    <AuthGate>
                      <RouterProvider router={router} />
                    </AuthGate>
                  </AuthProvider>
                </DependenciesGate>
              </div>
            </ModalProvider>
          </GoServerProvider>
        </ApolloClientProvider>
      </TTSProvider>
    </SyncedThemeProvider>
  )
}

export default createRoot(document.getElementById('root')!).render(<App />)
