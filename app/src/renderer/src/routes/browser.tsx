import { createFileRoute } from '@tanstack/react-router'
import { BrowserLayout } from '@renderer/features/browser/components/BrowserLayout'
import { checkBrowserEnabled } from '@renderer/lib/utils'
import { Navigate } from '@tanstack/react-router'

export const Route = createFileRoute('/browser')({
  component: BrowserRoute
})

function BrowserRoute() {
  // Check if browser feature is enabled
  if (!checkBrowserEnabled()) {
    return <Navigate to="/" />
  }

  return <BrowserLayout />
}
