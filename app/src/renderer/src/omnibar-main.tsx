// New file: Minimal bootstrap for omnibar window
import { createRoot } from 'react-dom/client'
import { ApolloClientProvider } from './graphql/provider'
import { SyncedThemeProvider } from './components/SyncedThemeProvider'
import { AuthProvider } from './contexts/AuthContext'
import OmnibarOverlay from './components/OmnibarOverlay'
import './assets/main.css'

function OmnibarApp() {
  return <OmnibarOverlay />
}

createRoot(document.getElementById('root')!).render(
  <SyncedThemeProvider>
    <ApolloClientProvider>
      <AuthProvider>
        <OmnibarApp />
      </AuthProvider>
    </ApolloClientProvider>
  </SyncedThemeProvider>
)
